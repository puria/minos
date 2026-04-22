package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/puria/minos/internal/actions"
	"github.com/puria/minos/internal/config"
	"github.com/puria/minos/internal/discovery"
	"github.com/puria/minos/internal/gitx"
	uitypes "github.com/puria/minos/internal/model"
	"github.com/puria/minos/internal/render"
)

type fakeRepoInspector struct {
	repos map[string]gitx.Repo
	errs  map[string]error
}

func (f fakeRepoInspector) Inspect(_ context.Context, root string, repoPath string, opts gitx.InspectOptions) (gitx.Repo, error) {
	if err := f.errs[repoPath]; err != nil {
		return gitx.Repo{}, err
	}
	return f.repos[repoPath], nil
}

type fakeTUIRunner struct {
	mu        sync.Mutex
	responses map[string]fakeTUIResponse
}

type fakeTUIResponse struct {
	out string
	err error
}

func (f *fakeTUIRunner) Run(_ context.Context, dir string, name string, args ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := dir + "|" + name + "|" + strings.Join(args, " ")
	resp, ok := f.responses[key]
	if !ok {
		return "", fmt.Errorf("unexpected command: %s", key)
	}
	return resp.out, resp.err
}

func TestSelectionMovesAcrossRepos(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.repos = []gitx.Repo{
		{DisplayPath: "a"},
		{DisplayPath: "b"},
	}
	m.applyFilter()
	m.move(1)
	if m.repoIndex != 1 {
		t.Fatalf("expected repo index 1, got %d", m.repoIndex)
	}
}

func TestRepoSelectionScrollsVisibleWindow(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.height = 12
	for i := 0; i < 20; i++ {
		m.repos = append(m.repos, gitx.Repo{DisplayPath: "repo"})
	}
	m.applyFilter()
	for i := 0; i < 10; i++ {
		m.move(1)
	}
	if m.repoIndex != 10 {
		t.Fatalf("expected repo index 10, got %d", m.repoIndex)
	}
	if m.repoOffset == 0 {
		t.Fatalf("expected repo offset to scroll, got %d", m.repoOffset)
	}
}

func TestListWindowKeepsSelectionVisible(t *testing.T) {
	start, end, offset := listWindow(20, 9, 0, 5)
	if start != 5 || end != 10 || offset != 5 {
		t.Fatalf("unexpected window: start=%d end=%d offset=%d", start, end, offset)
	}
}

func TestRestoreSelectionByPath(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.height = 12
	m.repos = []gitx.Repo{
		{CanonicalPath: "/tmp/a", DisplayPath: "a"},
		{CanonicalPath: "/tmp/b", DisplayPath: "b"},
		{CanonicalPath: "/tmp/c", DisplayPath: "c"},
	}
	m.applyFilter()
	m.selectedPath = "/tmp/c"
	m.restoreSelection()
	if m.repoIndex != 2 {
		t.Fatalf("expected repo index 2, got %d", m.repoIndex)
	}
}

func TestRenderConfirmationShowsShadowedBox(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.width = 80
	m.height = 24
	m.pending = &actions.PendingAction{Prompt: "Delete directory /tmp/repo ?"}
	out := m.renderConfirmation("background")
	if !strings.Contains(out, "╭") || !strings.Contains(out, "╰") || !strings.Contains(out, "░") {
		t.Fatalf("unexpected confirmation rendering: %s", out)
	}
}

func TestRenderBaseViewFitsTerminalHeight(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.width = 100
	m.height = 20
	m.showHelp = true
	m.repos = []gitx.Repo{{CanonicalPath: "/tmp/a", DisplayPath: "a"}}
	m.applyFilter()
	out := m.renderBaseView()
	if got := lipgloss.Height(out); got > m.height {
		t.Fatalf("base view height %d exceeds terminal height %d", got, m.height)
	}
}

func TestRenderBaseViewShowsVersionInFooter(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "v1.2.3")
	m.width = 100
	m.height = 20
	m.repos = []gitx.Repo{{CanonicalPath: "/tmp/a", DisplayPath: "a"}}
	m.applyFilter()
	out := m.renderBaseView()
	if !strings.Contains(out, "minos v1.2.3") {
		t.Fatalf("expected version in footer, got %q", out)
	}
}

func TestFitLineTruncatesLongRepoNames(t *testing.T) {
	line := fitLine("dyne/starters/saas/{{cookiecutter.project_name}}/admin/zencode/zenflows-crypto/test/test_helper/bats-support [2 wt]", 30)
	if lipgloss.Width(line) != 30 {
		t.Fatalf("unexpected width: %d", lipgloss.Width(line))
	}
	if !strings.Contains(line, "…") {
		t.Fatalf("expected ellipsis, got %q", line)
	}
}

func TestMoveRepoRefreshesStatusPane(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.repos = []gitx.Repo{
		{
			CanonicalPath: "/tmp/a",
			DisplayPath:   "a",
			Worktrees:     []gitx.Worktree{{Path: "/tmp/a", Branch: "main", IsMain: true, StatusShort: "## main\n M a.go"}},
		},
		{
			CanonicalPath: "/tmp/b",
			DisplayPath:   "b",
			Worktrees:     []gitx.Worktree{{Path: "/tmp/b", Branch: "dev", IsMain: true, StatusShort: "## dev\n?? new.txt"}},
		},
	}
	m.applyFilter()
	m.summaryView = "stale"
	m.viewport.Width = 80
	m.viewport.Height = 20
	m.move(1)
	m.summaryView = ""
	m.refreshViewport()
	if !strings.Contains(m.viewport.View(), "## dev") || !strings.Contains(m.viewport.View(), "new.txt") {
		t.Fatalf("expected viewport to refresh for second repo, got %q", m.viewport.View())
	}
}

func TestToggleRepoSelection(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.focus = paneRepos
	m.repos = []gitx.Repo{
		{CanonicalPath: "/tmp/a", DisplayPath: "a"},
	}
	m.applyFilter()
	m.toggleRepoSelection()
	if len(m.selectedRepos) != 1 {
		t.Fatalf("expected one selected repo, got %d", len(m.selectedRepos))
	}
	m.toggleRepoSelection()
	if len(m.selectedRepos) != 0 {
		t.Fatalf("expected selection cleared, got %d", len(m.selectedRepos))
	}
}

func TestRepoListLineRightAlignsBadges(t *testing.T) {
	line := repoListLine("  ", gitx.Repo{
		DisplayPath: "very/long/path/to/repo",
		Summary: gitx.RepoSummary{
			NonDefaultLocalBranchCount: 2,
			AheadBranchCount:           1,
		},
	}, render.NewStyles(), 40)
	if !strings.Contains(line, "2 br") || !strings.Contains(line, "1 ahead") {
		t.Fatalf("expected badges in line, got %q", line)
	}
}

func TestSummarizeTargetWorktreeUsesCheckedOutBranchPath(t *testing.T) {
	repo := gitx.Repo{
		Worktrees: []gitx.Worktree{
			{Path: "/tmp/main", Branch: "main", IsMain: true},
			{Path: "/tmp/feature", Branch: "feature"},
		},
		Branches: []gitx.Branch{
			{Name: "feature", CheckedOut: true, CheckedOutPath: "/tmp/feature"},
		},
	}
	entity := &uitypes.EntityRow{Kind: uitypes.EntityBranch, Index: 0}
	wt, ok := summarizeTargetWorktree(repo, entity)
	if !ok || wt.Path != "/tmp/feature" {
		t.Fatalf("unexpected worktree: %#v ok=%v", wt, ok)
	}
}

func TestWrapViewportTextWrapsLongLines(t *testing.T) {
	out := wrapViewportText("this is a very long line that should wrap nicely", 10)
	for _, line := range strings.Split(out, "\n") {
		if lipgloss.Width(line) > 10 {
			t.Fatalf("line exceeds width: %q", line)
		}
	}
}

func TestQuitWithSelectionRequiresConfirmation(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.selectedRepos["/tmp/a"] = struct{}{}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	next := updated.(model)
	if cmd != nil {
		t.Fatalf("expected no immediate quit command")
	}
	if !next.confirming || next.pending == nil || next.pending.Kind != actions.ConfirmQuit {
		t.Fatalf("expected quit confirmation, got %#v", next.pending)
	}
}

func TestBatchDeleteKeepsFailedSelections(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.pending = &actions.PendingAction{Kind: actions.DeleteRepoDirectories}
	m.confirming = true
	m.selectedRepos["/tmp/a"] = struct{}{}
	m.selectedRepos["/tmp/b"] = struct{}{}
	updated, _ := m.Update(actionDoneMsg{
		err:            assertErr("partial failure"),
		completedPaths: []string{"/tmp/a"},
		failedPaths:    []string{"/tmp/b"},
	})
	next := updated.(model)
	if _, ok := next.selectedRepos["/tmp/a"]; ok {
		t.Fatalf("expected successful path to be cleared")
	}
	if _, ok := next.selectedRepos["/tmp/b"]; !ok {
		t.Fatalf("expected failed path to remain selected")
	}
}

func TestKeyMapFullHelp(t *testing.T) {
	keys := newKeyMap()
	help := keys.FullHelp()
	if len(help) != 2 {
		t.Fatalf("unexpected full help groups: %#v", help)
	}
}

func TestInitAndScanCmd(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	m := newModel(config.Config{Root: root, GitWorkers: 1}, "test")
	m.scanner = discovery.NewScanner(fakeRepoInspector{
		repos: map[string]gitx.Repo{
			repoPath: {CanonicalPath: repoPath, DisplayPath: "repo"},
		},
	})
	msg := m.Init()()
	scan, ok := msg.(scanMsg)
	if !ok || len(scan.repos) != 1 {
		t.Fatalf("unexpected scan result: %#v", msg)
	}
}

func TestViewAndResize(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	if got := m.View(); got != "Loading..." {
		t.Fatalf("unexpected zero-size view: %q", got)
	}
	m.width = 90
	m.height = 24
	m.resize()
	if m.viewport.Width < 20 || m.viewport.Height < 8 {
		t.Fatalf("unexpected viewport size: %#v", m.viewport)
	}
}

func TestLoadSelectedPreviewCmd(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	runner := &fakeTUIRunner{
		responses: map[string]fakeTUIResponse{
			repoPath + "|git|stash show --stat stash@{0}": {out: " file.go | 2 +- \n"},
		},
	}
	m := newModel(config.Config{Root: root}, "test")
	m.inspector = gitx.NewInspector(runner)
	m.repos = []gitx.Repo{{
		CanonicalPath: repoPath,
		DisplayPath:   "repo",
		Stashes:       []gitx.Stash{{Ref: "stash@{0}"}},
	}}
	m.applyFilter()
	m.focus = paneEntities
	m.entityIndex = 0
	cmd := m.loadSelectedPreviewCmd()
	if cmd == nil {
		t.Fatalf("expected preview command")
	}
	msg := cmd().(stashStatMsg)
	if msg.stat != "file.go | 2 +-" {
		t.Fatalf("unexpected stash stat: %#v", msg)
	}
}

func TestPlanDeleteVariants(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	m := newModel(config.Config{Root: root}, "test")
	m.repos = []gitx.Repo{{
		CanonicalPath: repoPath,
		DisplayPath:   "repo",
		Worktrees:     []gitx.Worktree{{Path: filepath.Join(repoPath, "wt"), DisplayLabel: "wt"}},
		Branches:      []gitx.Branch{{Name: "feat/test"}},
		Stashes:       []gitx.Stash{{Ref: "stash@{0}"}},
		Submodules:    []gitx.Submodule{{DisplayPath: "test/bats"}},
	}}
	m.applyFilter()

	m.focus = paneRepos
	m.planDelete(false)
	if m.pending == nil || m.pending.Kind != actions.DeleteRepoDirectory {
		t.Fatalf("expected repo delete action, got %#v", m.pending)
	}

	m.focus = paneEntities
	m.entityIndex = 0
	m.pending = nil
	m.planDelete(true)
	if m.pending == nil || m.pending.Kind != actions.RemoveLinkedWorktree {
		t.Fatalf("expected worktree delete action, got %#v", m.pending)
	}

	m.entityIndex = 1
	m.pending = nil
	m.planDelete(true)
	if m.pending == nil || m.pending.Kind != actions.DeleteBranchForce {
		t.Fatalf("expected force branch delete, got %#v", m.pending)
	}

	m.entityIndex = 2
	m.pending = nil
	m.planDelete(false)
	if m.pending == nil || m.pending.Kind != actions.DropStash {
		t.Fatalf("expected stash drop, got %#v", m.pending)
	}

	m.entityIndex = 3
	m.pending = nil
	m.planDelete(false)
	if !strings.Contains(m.err, "submodule deletion is not supported") {
		t.Fatalf("expected submodule error, got %q", m.err)
	}
}

func TestSelectedRepoSet(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.repos = []gitx.Repo{
		{CanonicalPath: "/tmp/a"},
		{CanonicalPath: "/tmp/b"},
	}
	m.selectedRepos["/tmp/b"] = struct{}{}
	got := m.selectedRepoSet()
	if len(got) != 1 || got[0].CanonicalPath != "/tmp/b" {
		t.Fatalf("unexpected selected repos: %#v", got)
	}
}

func TestExecutePendingCmd(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "repo")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	m := newModel(config.Config{Root: root, GitWorkers: 2}, "test")

	msg := m.executePendingCmd(actions.PendingAction{Kind: actions.ConfirmQuit})().(actionDoneMsg)
	if msg.err != nil {
		t.Fatalf("unexpected confirm quit error: %v", msg.err)
	}

	msg = m.executePendingCmd(actions.PendingAction{
		Kind:    actions.DeleteRepoDirectories,
		Root:    root,
		Targets: []string{target},
	})().(actionDoneMsg)
	if len(msg.completedPaths) != 1 || msg.completedPaths[0] != target {
		t.Fatalf("unexpected delete result: %#v", msg)
	}
}

func TestExecutePendingCmdAggregatesErrors(t *testing.T) {
	root := t.TempDir()
	m := newModel(config.Config{Root: root, GitWorkers: 2}, "test")
	msg := m.executePendingCmd(actions.PendingAction{
		Kind:    actions.DeleteRepoDirectories,
		Root:    root,
		Targets: []string{"/tmp/outside"},
	})().(actionDoneMsg)
	if msg.err == nil || len(msg.failedPaths) != 1 {
		t.Fatalf("expected aggregated error, got %#v", msg)
	}
}

func TestRunSummaryCmd(t *testing.T) {
	root := t.TempDir()
	wt := filepath.Join(root, "repo")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	m := newModel(config.Config{Root: root, SummarizerCmd: "cat"}, "test")
	m.repos = []gitx.Repo{{
		CanonicalPath: wt,
		DisplayPath:   "repo",
		Worktrees:     []gitx.Worktree{{Path: wt, IsMain: true, Branch: "main", StatusShort: "## main\n M file.go"}},
	}}
	m.applyFilter()
	cmd := m.runSummaryCmd()
	if cmd == nil {
		t.Fatalf("expected summary cmd")
	}
	msg := cmd().(summaryMsg)
	if msg.err != nil || !strings.Contains(msg.out, "Summarize the following git changes concisely") {
		t.Fatalf("unexpected summary msg: %#v", msg)
	}
}

func TestRunSummaryCmdErrors(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp", SummarizerCmd: ""}, "test")
	msg := m.runSummaryCmd()().(summaryMsg)
	if msg.err == nil {
		t.Fatalf("expected no-summarizer error")
	}

	m = newModel(config.Config{Root: "/tmp", SummarizerCmd: "cat"}, "test")
	m.repos = []gitx.Repo{{CanonicalPath: "/tmp/repo"}}
	m.applyFilter()
	msg = m.runSummaryCmd()().(summaryMsg)
	if msg.err == nil {
		t.Fatalf("expected no-worktree error")
	}
}

func TestNormalizeSummarizerCmdAndPrompt(t *testing.T) {
	if got := normalizeSummarizerCmd("codex"); !strings.Contains(got, "codex exec") {
		t.Fatalf("unexpected codex normalization: %q", got)
	}
	if got := normalizeSummarizerCmd("claude -p"); got != "claude -p" {
		t.Fatalf("unexpected passthrough cmd: %q", got)
	}
	prompt := buildSummaryPrompt(gitx.Repo{DisplayPath: "repo"}, gitx.Worktree{Path: "/tmp/repo", Branch: "main", StatusShort: "## main"})
	if !strings.Contains(prompt, "Repo: repo") || !strings.Contains(prompt, "Status:") {
		t.Fatalf("unexpected prompt: %q", prompt)
	}
}

func TestSummarizeTargetWorktreeFallbacks(t *testing.T) {
	repo := gitx.Repo{
		Worktrees: []gitx.Worktree{{Path: "/tmp/main", IsMain: true}, {Path: "/tmp/other"}},
	}
	wt, ok := summarizeTargetWorktree(repo, nil)
	if !ok || wt.Path != "/tmp/main" {
		t.Fatalf("unexpected main fallback: %#v ok=%v", wt, ok)
	}
	repo = gitx.Repo{Worktrees: []gitx.Worktree{{Path: "/tmp/other"}}}
	wt, ok = summarizeTargetWorktree(repo, nil)
	if !ok || wt.Path != "/tmp/other" {
		t.Fatalf("unexpected first-worktree fallback: %#v ok=%v", wt, ok)
	}
}

func TestUpdateConfirmationPaths(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.confirming = true
	m.pending = &actions.PendingAction{Kind: actions.DeleteRepoDirectory}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected execution cmd")
	}
	_ = updated

	m = newModel(config.Config{Root: "/tmp"}, "test")
	m.confirming = true
	m.pending = &actions.PendingAction{Kind: actions.DeleteRepoDirectory}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	next := updated.(model)
	if next.confirming || next.pending != nil {
		t.Fatalf("expected confirmation cancel, got %#v", next.pending)
	}
}

func TestUpdateSummaryAndScanMessages(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.viewport.Width = 20
	m.viewport.Height = 5
	updated, _ := m.Update(summaryMsg{out: "hello"})
	next := updated.(model)
	if next.summaryView != "hello" {
		t.Fatalf("expected summary view update")
	}

	updated, _ = m.Update(scanMsg{repos: []gitx.Repo{{CanonicalPath: "/tmp/a", DisplayPath: "a"}}})
	next = updated.(model)
	if next.loading || len(next.repos) != 1 {
		t.Fatalf("expected scan to populate repos: %#v", next)
	}
}

func TestUpdateStatusKeys(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.width = 100
	m.height = 20
	m.repos = []gitx.Repo{{CanonicalPath: "/tmp/a", DisplayPath: "a"}}
	m.applyFilter()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !updated.(model).showHelp {
		t.Fatalf("expected help toggle")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if !strings.Contains(updated.(model).status, "Sort:") {
		t.Fatalf("expected sort status")
	}
}

func TestViewWithConfirmationAndHelpers(t *testing.T) {
	m := newModel(config.Config{Root: "/tmp"}, "test")
	m.width = 80
	m.height = 20
	m.pending = &actions.PendingAction{Prompt: "Delete repo?"}
	m.confirming = true
	out := m.View()
	if !strings.Contains(out, "DANGER MODE") {
		t.Fatalf("expected confirmation view, got %q", out)
	}
	if got := clamp(-1, 5); got != 0 {
		t.Fatalf("unexpected clamp low: %d", got)
	}
	if got := clamp(9, 5); got != 5 {
		t.Fatalf("unexpected clamp high: %d", got)
	}
	if got := ensureVisible(5, 0, 3); got != 3 {
		t.Fatalf("unexpected ensureVisible result: %d", got)
	}
	if got := padRight("x", 3); got != "x  " {
		t.Fatalf("unexpected padRight: %q", got)
	}
	if got := fitLine("abcd", 3); !strings.Contains(got, "…") {
		t.Fatalf("expected ellipsis, got %q", got)
	}
}

func assertErr(msg string) error { return &testErr{msg: msg} }

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
