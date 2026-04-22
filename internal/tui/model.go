package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/puria/minos/internal/actions"
	"github.com/puria/minos/internal/config"
	"github.com/puria/minos/internal/discovery"
	"github.com/puria/minos/internal/gitx"
	uitypes "github.com/puria/minos/internal/model"
	"github.com/puria/minos/internal/render"
	"golang.org/x/sync/errgroup"
)

type pane int

const (
	paneRepos pane = iota
	paneEntities
	paneStatus
)

type scanMsg struct {
	repos []gitx.Repo
	err   error
}

type actionDoneMsg struct {
	err            error
	completedPaths []string
	failedPaths    []string
}

type stashStatMsg struct {
	repoPath string
	ref      string
	stat     string
	err      error
}

type summaryMsg struct {
	out string
	err error
}

type model struct {
	cfg            config.Config
	version        string
	scanner        discovery.Scanner
	inspector      gitx.Inspector
	executor       actions.Executor
	repos          []gitx.Repo
	filtered       []int
	sortMode       uitypes.SortMode
	repoIndex      int
	repoOffset     int
	entityIndex    int
	entityOffset   int
	focus          pane
	loading        bool
	err            string
	status         string
	filterInput    textinput.Model
	filtering      bool
	viewport       viewport.Model
	help           help.Model
	keys           keyMap
	styles         render.Styles
	width          int
	height         int
	showHelp       bool
	confirming     bool
	pending        *actions.PendingAction
	summaryView    string
	statusContent  string
	selectedPath   string
	selectedRepos  map[string]struct{}
	summaryRunning bool
}

func Run(cfg config.Config, version string) error {
	runner := gitx.ExecRunner{}
	m := newModel(cfg, runner, version)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(cfg config.Config, runner gitx.Runner, version string) model {
	filterInput := textinput.New()
	filterInput.Prompt = "filter> "
	filterInput.CharLimit = 256
	filterInput.Width = 30

	vp := viewport.New(10, 10)
	vp.SetContent("Scanning repositories...")

	return model{
		cfg:           cfg,
		version:       version,
		scanner:       discovery.NewScanner(gitx.NewInspector(runner)),
		inspector:     gitx.NewInspector(runner),
		executor:      actions.NewExecutor(runner),
		filterInput:   filterInput,
		viewport:      vp,
		help:          help.New(),
		keys:          newKeyMap(),
		styles:        render.NewStyles(),
		loading:       true,
		selectedRepos: map[string]struct{}{},
	}
}

func (m model) Init() tea.Cmd {
	return m.scanCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.filtering {
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.applyFilter()
				return m, nil
			case "enter":
				m.filtering = false
				m.applyFilter()
				return m, nil
			default:
				m.applyFilter()
			}
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
	case scanMsg:
		m.loading = false
		m.err = ""
		if msg.err != nil {
			m.err = msg.err.Error()
		}
		m.repos = msg.repos
		m.applyFilter()
		m.restoreSelection()
		m.status = fmt.Sprintf("Loaded %d repos", len(msg.repos))
		return m, m.loadSelectedPreviewCmd()
	case actionDoneMsg:
		if m.pending != nil && m.pending.Kind == actions.ConfirmQuit {
			return m, tea.Quit
		}
		if m.pending != nil && m.pending.Kind == actions.DeleteRepoDirectories {
			for _, path := range msg.completedPaths {
				delete(m.selectedRepos, path)
			}
			if len(msg.failedPaths) > 0 {
				m.err = msg.err.Error()
				m.status = fmt.Sprintf("Deleted %d repos, %d failed", len(msg.completedPaths), len(msg.failedPaths))
				m.confirming = false
				m.pending = nil
				m.loading = true
				return m, m.scanCmd()
			}
		}
		if m.pending != nil && m.pending.Kind == actions.DeleteRepoDirectory {
			if msg.err != nil {
				m.err = msg.err.Error()
				m.status = "Action failed"
				return m, nil
			}
			delete(m.selectedRepos, m.pending.Target)
		}
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = "Action failed"
			return m, nil
		}
		m.status = "Action completed"
		m.confirming = false
		m.pending = nil
		m.loading = true
		return m, m.scanCmd()
	case stashStatMsg:
		if msg.err == nil {
			for i := range m.repos {
				if m.repos[i].CanonicalPath != msg.repoPath {
					continue
				}
				for j := range m.repos[i].Stashes {
					if m.repos[i].Stashes[j].Ref == msg.ref {
						m.repos[i].Stashes[j].Stat = msg.stat
					}
				}
			}
			m.refreshViewport()
		}
	case summaryMsg:
		m.summaryRunning = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.summaryView = msg.out
			m.statusContent = msg.out
			m.viewport.SetContent(wrapViewportText(m.statusContent, m.viewport.Width))
			m.viewport.GotoTop()
		}
	}

	if m.confirming {
		if keymsg, ok := msg.(tea.KeyMsg); ok {
			switch keymsg.String() {
			case "y", "enter":
				if m.pending == nil {
					m.confirming = false
					return m, nil
				}
				return m, m.executePendingCmd(*m.pending)
			case "n", "esc":
				m.confirming = false
				m.pending = nil
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			if len(m.selectedRepos) > 0 {
				m.pending = &actions.PendingAction{
					Kind:   actions.ConfirmQuit,
					Prompt: fmt.Sprintf("Quit with %d selected repos?\n\nSelections will be lost.", len(m.selectedRepos)),
				}
				m.confirming = true
				return m, nil
			}
			return m, tea.Quit
		case key.Matches(msg, m.keys.FocusNext):
			m.focus = (m.focus + 1) % 3
		case key.Matches(msg, m.keys.FocusPrev):
			m.focus = (m.focus + 2) % 3
		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			m.status = "Refreshing..."
			return m, m.scanCmd()
		case key.Matches(msg, m.keys.Sort):
			m.sortMode = uitypes.NextSortMode(m.sortMode)
			m.applyFilter()
			m.status = "Sort: " + m.sortMode.String()
		case key.Matches(msg, m.keys.Filter):
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case key.Matches(msg, m.keys.ToggleSel):
			m.toggleRepoSelection()
		case key.Matches(msg, m.keys.Up):
			m.move(-1)
			m.summaryView = ""
			m.refreshViewport()
			return m, m.loadSelectedPreviewCmd()
		case key.Matches(msg, m.keys.Down):
			m.move(1)
			m.summaryView = ""
			m.refreshViewport()
			return m, m.loadSelectedPreviewCmd()
		case key.Matches(msg, m.keys.Delete):
			m.planDelete(false)
		case key.Matches(msg, m.keys.Force):
			m.planDelete(true)
		case key.Matches(msg, m.keys.Summary):
			m.summaryRunning = true
			m.status = "Explaining changes..."
			m.summaryView = "Running summarizer...\n\nThis may take a few seconds."
			m.statusContent = m.summaryView
			m.viewport.SetContent(wrapViewportText(m.statusContent, m.viewport.Width))
			m.viewport.GotoTop()
			return m, m.runSummaryCmd()
		}
	}

	if m.focus == paneStatus {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	view := m.renderBaseView()
	if m.confirming && m.pending != nil {
		return m.renderConfirmation(view)
	}
	return view
}

func (m model) renderBaseView() string {
	footer := m.status
	if m.err != "" {
		footer = m.styles.Danger.Render(m.err)
	}
	if footer == "" {
		footer = fmt.Sprintf("root=%s  sort=%s", m.cfg.Root, m.sortMode.String())
	}
	if count := len(m.selectedRepos); count > 0 {
		footer = fmt.Sprintf("%s  selected=%d", footer, count)
	}
	if m.summaryRunning {
		footer = fmt.Sprintf("%s  %s", footer, m.styles.BadgeWorking.Render("tool working"))
	}
	if m.version != "" {
		footer = fmt.Sprintf("%s  %s", footer, m.styles.Muted.Render("minos "+m.version))
	}

	var helpView string
	if m.showHelp {
		helpView = m.help.View(m.keys)
	}

	filterHeight := 0
	if m.filtering {
		filterHeight = lipgloss.Height(m.filterInput.View())
	}
	footerHeight := lipgloss.Height(footer)
	helpHeight := lipgloss.Height(helpView)
	paneFrameHeight := 2
	bodyH := max(1, m.height-filterHeight-footerHeight-helpHeight-paneFrameHeight)

	leftW := max(24, m.width/3)
	midW := max(28, m.width/3)
	rightW := max(36, m.width-leftW-midW-6)

	left := m.renderRepos(leftW, bodyH)
	middle := m.renderEntities(midW, bodyH)
	right := m.renderStatus(rightW, bodyH)

	layout := lipgloss.JoinHorizontal(lipgloss.Top, left, middle, right)

	view := lipgloss.JoinVertical(lipgloss.Left, layout, footer, helpView)
	if m.filtering {
		view = lipgloss.JoinVertical(lipgloss.Left, m.filterInput.View(), view)
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, m.styles.App.Render(view))
}

func (m model) renderConfirmation(base string) string {
	lines := normalizeScreen(ansi.Strip(base), m.width, m.height)

	bodyWidth := min(76, max(48, m.width-12))
	contentWidth := bodyWidth - 2
	content := wrapLines([]string{
		"DANGER MODE",
		"",
		m.pending.Prompt,
		"",
		"Press y/enter to confirm.",
		"Press n/esc to cancel.",
	}, contentWidth-2)
	box := boxLines(content, bodyWidth)

	top := max(0, (m.height-len(box))/2)
	left := max(0, (m.width-bodyWidth)/2)
	drawOverlay(lines, shadowLines(box), top+1, left+2)
	drawOverlay(lines, box, top, left)

	return strings.Join(lines, "\n")
}

func (m *model) renderRepos(width int, height int) string {
	repo := m.selectedRepo()
	header := m.styles.Header.Render("Repositories")
	rows := []string{header}
	visibleRows := listVisibleRows(height)
	start, end, offset := listWindow(len(m.filtered), m.repoIndex, m.repoOffset, visibleRows)
	m.repoOffset = offset
	if m.loading {
		rows = append(rows, "Scanning...")
	}
	if len(m.filtered) == 0 && !m.loading {
		rows = append(rows, "No repositories")
	}
	for i := start; i < end; i++ {
		idx := m.filtered[i]
		prefix := "  "
		if _, ok := m.selectedRepos[m.repos[idx].CanonicalPath]; ok {
			prefix = "* "
		}
		line := repoListLine(prefix, m.repos[idx], m.styles, width-4)
		style := lipgloss.NewStyle()
		if m.repos[idx].Summary.SafeToRemove {
			style = m.styles.SafeRepo
		} else if m.repos[idx].Summary.DirtyWorktreeCount > 0 || m.repos[idx].Summary.StashCount > 0 {
			style = m.styles.DirtyRepo
		}
		if i == m.repoIndex {
			style = m.styles.Selected
		}
		rows = append(rows, style.Width(width-4).Render(line))
	}
	if repo != nil && len(repo.Errors) > 0 {
		rows = append(rows, m.styles.Muted.Width(width-4).Render(fitLine(fmt.Sprintf("%d errors", len(repo.Errors)), width-4)))
	}
	return m.paneStyle(paneRepos).Width(width).Height(height).Render(strings.Join(rows, "\n"))
}

func (m *model) renderEntities(width int, height int) string {
	header := m.styles.Header.Render("Details")
	rows := []string{header}
	repo := m.selectedRepo()
	entities := m.entities()
	visibleRows := listVisibleRows(height)
	start, end, offset := listWindow(len(entities), m.entityIndex, m.entityOffset, visibleRows)
	m.entityOffset = offset
	if repo == nil {
		rows = append(rows, "No selection")
	} else {
		for i := start; i < end; i++ {
			entity := entities[i]
			line := fitLine(render.EntityRow(*repo, entity), width-4)
			style := lipgloss.NewStyle()
			if entity.Kind == uitypes.EntityWorktree && repo.Worktrees[entity.Index].Dirty {
				style = m.styles.Dirty
			}
			if i == m.entityIndex {
				style = m.styles.Selected
			}
			rows = append(rows, style.Width(width-4).Render(line))
		}
	}
	return m.paneStyle(paneEntities).Width(width).Height(height).Render(strings.Join(rows, "\n"))
}

func (m *model) renderStatus(width int, height int) string {
	m.viewport.Width = width - 4
	m.viewport.Height = height - 2
	if m.statusContent != "" {
		m.viewport.SetContent(wrapViewportText(m.statusContent, m.viewport.Width))
	}
	content := m.viewport.View()
	return m.paneStyle(paneStatus).Width(width).Height(height).Render(m.styles.Header.Render("Status") + "\n" + content)
}

func (m *model) paneStyle(p pane) lipgloss.Style {
	if m.focus == p {
		return m.styles.PaneFocused
	}
	return m.styles.Pane
}

func (m *model) move(delta int) {
	switch m.focus {
	case paneRepos:
		if len(m.filtered) == 0 {
			return
		}
		m.repoIndex = clamp(m.repoIndex+delta, 0, len(m.filtered)-1)
		m.repoOffset = ensureVisible(m.repoIndex, m.repoOffset, listVisibleRows(max(8, m.height-4)))
		m.entityIndex = 0
		m.entityOffset = 0
	case paneEntities:
		entities := m.entities()
		if len(entities) == 0 {
			return
		}
		m.entityIndex = clamp(m.entityIndex+delta, 0, len(entities)-1)
		m.entityOffset = ensureVisible(m.entityIndex, m.entityOffset, listVisibleRows(max(8, m.height-4)))
	case paneStatus:
		if delta < 0 {
			m.viewport.LineUp(1)
		} else {
			m.viewport.LineDown(1)
		}
	}
}

func (m *model) applyFilter() {
	m.filtered = uitypes.FilterAndSort(m.repos, m.filterInput.Value(), m.sortMode, m.cfg.ShowCleanOnly, m.cfg.ShowSafeOnly)
	if len(m.filtered) == 0 {
		m.repoIndex = 0
		m.repoOffset = 0
		m.entityIndex = 0
		m.entityOffset = 0
		m.refreshViewport()
		return
	}
	m.repoIndex = clamp(m.repoIndex, 0, len(m.filtered)-1)
	m.repoOffset = ensureVisible(m.repoIndex, 0, listVisibleRows(max(8, m.height-4)))
	entities := m.entities()
	if len(entities) == 0 {
		m.entityIndex = 0
		m.entityOffset = 0
	} else {
		m.entityIndex = clamp(m.entityIndex, 0, len(entities)-1)
		m.entityOffset = ensureVisible(m.entityIndex, 0, listVisibleRows(max(8, m.height-4)))
	}
	m.refreshViewport()
}

func (m *model) refreshViewport() {
	repo := m.selectedRepo()
	entity := m.selectedEntity()
	if m.summaryView != "" && entity != nil && entity.Kind == uitypes.EntityWorktree {
		m.statusContent = m.summaryView
	} else {
		m.statusContent = render.RightPane(repo, entity)
	}
	m.viewport.SetContent(wrapViewportText(m.statusContent, m.viewport.Width))
	m.viewport.GotoTop()
}

func (m *model) restoreSelection() {
	if len(m.filtered) == 0 {
		m.selectedPath = ""
		return
	}
	if m.selectedPath != "" {
		for i, idx := range m.filtered {
			if m.repos[idx].CanonicalPath == m.selectedPath {
				m.repoIndex = i
				m.repoOffset = ensureVisible(m.repoIndex, m.repoOffset, listVisibleRows(max(8, m.height-4)))
				return
			}
		}
	}
	m.repoIndex = clamp(m.repoIndex, 0, len(m.filtered)-1)
	m.repoOffset = ensureVisible(m.repoIndex, m.repoOffset, listVisibleRows(max(8, m.height-4)))
}

func (m *model) selectedRepo() *gitx.Repo {
	if len(m.filtered) == 0 || m.repoIndex >= len(m.filtered) {
		return nil
	}
	return &m.repos[m.filtered[m.repoIndex]]
}

func (m *model) entities() []uitypes.EntityRow {
	return uitypes.BuildEntities(m.selectedRepo())
}

func (m *model) selectedEntity() *uitypes.EntityRow {
	entities := m.entities()
	if len(entities) == 0 || m.entityIndex >= len(entities) {
		return nil
	}
	return &entities[m.entityIndex]
}

func (m *model) resize() {
	m.viewport.Width = max(20, m.width/3)
	m.viewport.Height = max(8, m.height-6)
}

func (m model) scanCmd() tea.Cmd {
	return func() tea.Msg {
		repos, err := m.scanner.Scan(context.Background(), m.cfg)
		return scanMsg{repos: repos, err: err}
	}
}

func (m model) loadSelectedPreviewCmd() tea.Cmd {
	repo := m.selectedRepo()
	entity := m.selectedEntity()
	if repo == nil || entity == nil || entity.Kind != uitypes.EntityStash {
		return nil
	}
	stash := repo.Stashes[entity.Index]
	if stash.Stat != "" {
		return nil
	}
	return func() tea.Msg {
		stat, err := m.inspector.PopulateStashStat(context.Background(), repo.CanonicalPath, stash.Ref)
		return stashStatMsg{repoPath: repo.CanonicalPath, ref: stash.Ref, stat: stat, err: err}
	}
}

func (m *model) planDelete(force bool) {
	repo := m.selectedRepo()
	if repo == nil {
		return
	}
	m.selectedPath = repo.CanonicalPath
	var (
		action actions.PendingAction
		err    error
	)
	entity := m.selectedEntity()
	if m.focus == paneRepos || entity == nil {
		if m.focus == paneRepos && len(m.selectedRepos) > 0 {
			repos := m.selectedRepoSet()
			action, err = actions.PlanRepoDeleteMany(m.cfg.Root, repos)
		} else {
			action, err = actions.PlanRepoDelete(m.cfg.Root, *repo)
		}
	} else {
		switch entity.Kind {
		case uitypes.EntityWorktree:
			action, err = actions.PlanWorktreeRemove(m.cfg.Root, *repo, repo.Worktrees[entity.Index], force)
		case uitypes.EntityBranch:
			action, err = actions.PlanBranchDelete(m.cfg.Root, *repo, repo.Branches[entity.Index], force)
		case uitypes.EntityStash:
			action, err = actions.PlanStashDrop(m.cfg.Root, *repo, repo.Stashes[entity.Index])
		case uitypes.EntitySubmodule:
			m.err = "submodule deletion is not supported from the repo entity pane"
			return
		}
	}
	if err != nil {
		m.err = err.Error()
		return
	}
	m.pending = &action
	m.confirming = true
}

func (m *model) toggleRepoSelection() {
	if m.focus != paneRepos {
		return
	}
	repo := m.selectedRepo()
	if repo == nil {
		return
	}
	if _, ok := m.selectedRepos[repo.CanonicalPath]; ok {
		delete(m.selectedRepos, repo.CanonicalPath)
		return
	}
	m.selectedRepos[repo.CanonicalPath] = struct{}{}
}

func (m *model) selectedRepoSet() []gitx.Repo {
	repos := make([]gitx.Repo, 0, len(m.selectedRepos))
	for _, repo := range m.repos {
		if _, ok := m.selectedRepos[repo.CanonicalPath]; ok {
			repos = append(repos, repo)
		}
	}
	return repos
}

func (m model) executePendingCmd(action actions.PendingAction) tea.Cmd {
	return func() tea.Msg {
		if action.Kind == actions.ConfirmQuit {
			return actionDoneMsg{}
		}
		if action.Kind == actions.DeleteRepoDirectories {
			var (
				completed []string
				failed    []string
				errs      []string
				mu        sync.Mutex
			)
			group, ctx := errgroup.WithContext(context.Background())
			group.SetLimit(min(8, max(1, min(len(action.Targets), m.cfg.GitWorkers))))
			for _, target := range action.Targets {
				target := target
				group.Go(func() error {
					err := m.executor.Execute(ctx, actions.PendingAction{
						Kind:   actions.DeleteRepoDirectory,
						Root:   action.Root,
						Target: target,
					})
					mu.Lock()
					defer mu.Unlock()
					if err != nil {
						failed = append(failed, target)
						errs = append(errs, fmt.Sprintf("%s: %v", target, err))
						return nil
					}
					completed = append(completed, target)
					return nil
				})
			}
			_ = group.Wait()
			if len(errs) > 0 {
				return actionDoneMsg{
					err:            errors.New(strings.Join(errs, "\n")),
					completedPaths: completed,
					failedPaths:    failed,
				}
			}
			return actionDoneMsg{completedPaths: completed}
		}
		err := m.executor.Execute(context.Background(), action)
		return actionDoneMsg{err: err}
	}
}

func (m model) runSummaryCmd() tea.Cmd {
	if strings.TrimSpace(m.cfg.SummarizerCmd) == "" {
		return func() tea.Msg { return summaryMsg{err: fmt.Errorf("no summarizer configured")} }
	}
	repo := m.selectedRepo()
	entity := m.selectedEntity()
	if repo == nil {
		return nil
	}
	wt, ok := summarizeTargetWorktree(*repo, entity)
	if !ok {
		return func() tea.Msg { return summaryMsg{err: fmt.Errorf("no worktree available to explain")} }
	}
	if wt.Path == "" {
		return nil
	}
	prompt := buildSummaryPrompt(*repo, wt)
	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", normalizeSummarizerCmd(m.cfg.SummarizerCmd))
		cmd.Dir = wt.Path
		cmd.Stdin = strings.NewReader(prompt)
		var out bytes.Buffer
		var errOut bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errOut
		err := cmd.Run()
		if err != nil {
			msg := strings.TrimSpace(errOut.String())
			if msg == "" {
				msg = err.Error()
			}
			return summaryMsg{err: fmt.Errorf("summarizer: %s", msg)}
		}
		return summaryMsg{out: strings.TrimSpace(out.String())}
	}
}

func normalizeSummarizerCmd(cmd string) string {
	trimmed := strings.TrimSpace(cmd)
	switch trimmed {
	case "codex":
		return "codex exec --skip-git-repo-check --color never --ephemeral -"
	default:
		return cmd
	}
}

func summarizeTargetWorktree(repo gitx.Repo, entity *uitypes.EntityRow) (gitx.Worktree, bool) {
	if entity != nil {
		switch entity.Kind {
		case uitypes.EntityWorktree:
			return repo.Worktrees[entity.Index], true
		case uitypes.EntityBranch:
			br := repo.Branches[entity.Index]
			if br.CheckedOut {
				for _, wt := range repo.Worktrees {
					if wt.Path == br.CheckedOutPath {
						return wt, true
					}
				}
			}
		}
	}
	for _, wt := range repo.Worktrees {
		if wt.IsMain {
			return wt, true
		}
	}
	if len(repo.Worktrees) > 0 {
		return repo.Worktrees[0], true
	}
	return gitx.Worktree{}, false
}

func buildSummaryPrompt(repo gitx.Repo, wt gitx.Worktree) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Summarize the following git changes concisely.\n\nRepo: %s\nWorktree: %s\nBranch: %s\n\n", repo.DisplayPath, wt.Path, wt.Branch)
	fmt.Fprintf(&b, "Status:\n%s\n\n", wt.StatusShort)
	return b.String()
}

func repoListLine(prefix string, repo gitx.Repo, styles render.Styles, width int) string {
	if width <= 0 {
		return ""
	}
	badges := render.RepoBadges(repo, styles)
	if badges == "" {
		return fitLine(prefix+repo.DisplayPath, width)
	}
	leftWidth := max(1, width-lipgloss.Width(badges)-1)
	left := fitLine(prefix+repo.DisplayPath, leftWidth)
	return padRight(left, leftWidth) + " " + badges
}

func clamp(v int, low int, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func listVisibleRows(height int) int {
	return max(1, height-3)
}

func ensureVisible(index int, offset int, visible int) int {
	if visible <= 0 {
		return 0
	}
	if index < offset {
		return index
	}
	if index >= offset+visible {
		return index - visible + 1
	}
	return offset
}

func listWindow(total int, selected int, offset int, visible int) (start int, end int, nextOffset int) {
	if total <= 0 || visible <= 0 {
		return 0, 0, 0
	}
	selected = clamp(selected, 0, total-1)
	offset = ensureVisible(selected, offset, visible)
	if offset > total-visible {
		offset = max(0, total-visible)
	}
	end = min(total, offset+visible)
	return offset, end, offset
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func normalizeScreen(screen string, width int, height int) []string {
	raw := strings.Split(screen, "\n")
	lines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		line := ""
		if i < len(raw) {
			line = raw[i]
		}
		lines = append(lines, padRight(line, width))
	}
	return lines
}

func drawOverlay(screen []string, overlay []string, top int, left int) {
	for row, line := range overlay {
		targetRow := top + row
		if targetRow < 0 || targetRow >= len(screen) {
			continue
		}
		baseRunes := []rune(screen[targetRow])
		overlayRunes := []rune(line)
		for col, ch := range overlayRunes {
			targetCol := left + col
			if targetCol < 0 || targetCol >= len(baseRunes) {
				continue
			}
			if ch == '\000' {
				continue
			}
			baseRunes[targetCol] = ch
		}
		screen[targetRow] = string(baseRunes)
	}
}

func boxLines(content []string, width int) []string {
	inner := max(2, width-2)
	lines := []string{"╭" + strings.Repeat("─", inner) + "╮"}
	for _, line := range content {
		lines = append(lines, "│"+padRight(line, inner)+"│")
	}
	lines = append(lines, "╰"+strings.Repeat("─", inner)+"╯")
	return lines
}

func shadowLines(box []string) []string {
	shadow := make([]string, 0, len(box)+1)
	for i, line := range box {
		width := len([]rune(line))
		if i == 0 {
			shadow = append(shadow, strings.Repeat("\000", width))
			continue
		}
		shadow = append(shadow, strings.Repeat("\000", max(0, width-1))+"░")
	}
	shadow = append(shadow, strings.Repeat("░", len([]rune(box[len(box)-1]))))
	return shadow
}

func wrapLines(lines []string, width int) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			out = append(out, "")
			continue
		}
		words := strings.Fields(line)
		current := ""
		for _, word := range words {
			next := word
			if current != "" {
				next = current + " " + word
			}
			if lipgloss.Width(next) <= width {
				current = next
				continue
			}
			if current != "" {
				out = append(out, current)
			}
			current = word
		}
		if current != "" {
			out = append(out, current)
		}
	}
	return out
}

func padRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) >= width {
		runes := []rune(s)
		if len(runes) > width {
			return string(runes[:width])
		}
		return s
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

func fitLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return padRight(ansi.Truncate(s, width, "…"), width)
}

func wrapViewportText(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			out = append(out, "")
			continue
		}
		out = append(out, wrapLineToWidth(line, width)...)
	}
	return strings.Join(out, "\n")
}

func wrapLineToWidth(line string, width int) []string {
	if lipgloss.Width(line) <= width {
		return []string{line}
	}
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}
	out := []string{}
	current := ""
	for _, word := range words {
		if lipgloss.Width(word) > width {
			if current != "" {
				out = append(out, current)
				current = ""
			}
			runes := []rune(word)
			for len(runes) > 0 {
				n := min(width, len(runes))
				out = append(out, string(runes[:n]))
				runes = runes[n:]
			}
			continue
		}
		next := word
		if current != "" {
			next = current + " " + word
		}
		if lipgloss.Width(next) <= width {
			current = next
			continue
		}
		out = append(out, current)
		current = word
	}
	if current != "" {
		out = append(out, current)
	}
	return out
}
