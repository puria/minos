package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/puria/minos/internal/actions"
	"github.com/puria/minos/internal/config"
	"github.com/puria/minos/internal/gitx"
	uitypes "github.com/puria/minos/internal/model"
	"github.com/puria/minos/internal/render"
)

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

func assertErr(msg string) error { return &testErr{msg: msg} }

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
