package actions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puria/minos/internal/gitx"
)

type fakeRunner struct {
	out   string
	err   error
	calls []runnerCall
}

type runnerCall struct {
	dir  string
	name string
	args []string
}

func (f *fakeRunner) Run(_ context.Context, dir string, name string, args ...string) (string, error) {
	f.calls = append(f.calls, runnerCall{
		dir:  dir,
		name: name,
		args: append([]string(nil), args...),
	})
	return f.out, f.err
}

func TestNewExecutor(t *testing.T) {
	runner := &fakeRunner{}
	executor := NewExecutor(runner)
	if executor.Runner != runner {
		t.Fatalf("expected runner to be stored")
	}
}

func TestPlanRepoDelete(t *testing.T) {
	root := t.TempDir()
	repo := gitx.Repo{CanonicalPath: filepath.Join(root, "repo")}

	action, err := PlanRepoDelete(root, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != DeleteRepoDirectory {
		t.Fatalf("unexpected kind: %v", action.Kind)
	}
	if action.Target != repo.CanonicalPath {
		t.Fatalf("unexpected target: %s", action.Target)
	}
	if !strings.Contains(action.Prompt, repo.CanonicalPath) {
		t.Fatalf("expected prompt to mention path, got %q", action.Prompt)
	}
}

func TestPlanRepoDeleteRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	_, err := PlanRepoDelete(root, gitx.Repo{CanonicalPath: "/tmp/outside"})
	if err == nil {
		t.Fatalf("expected outside repo to fail")
	}
}

func TestPlanRepoDeleteManyRejectsEmpty(t *testing.T) {
	root := t.TempDir()
	_, err := PlanRepoDeleteMany(root, nil)
	if err == nil {
		t.Fatalf("expected empty selection to fail")
	}
}

func TestPlanRepoDeleteMany(t *testing.T) {
	root := t.TempDir()
	repos := []gitx.Repo{
		{CanonicalPath: filepath.Join(root, "a")},
		{CanonicalPath: filepath.Join(root, "b")},
	}

	action, err := PlanRepoDeleteMany(root, repos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != DeleteRepoDirectories {
		t.Fatalf("unexpected kind: %v", action.Kind)
	}
	if len(action.Targets) != 2 {
		t.Fatalf("unexpected targets: %#v", action.Targets)
	}
	if !strings.Contains(action.Prompt, "Delete 2 directories?") {
		t.Fatalf("unexpected prompt: %q", action.Prompt)
	}
}

func TestPlanRepoDeleteManyAddsOverflowMessage(t *testing.T) {
	root := t.TempDir()
	repos := make([]gitx.Repo, 0, 6)
	for i := 0; i < 6; i++ {
		repos = append(repos, gitx.Repo{CanonicalPath: filepath.Join(root, "repo", string(rune('a'+i)))})
	}

	action, err := PlanRepoDeleteMany(root, repos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(action.Prompt, "... and 1 more") {
		t.Fatalf("expected overflow summary, got %q", action.Prompt)
	}
}

func TestPlanWorktreeRemoveMainDelegatesToRepoDelete(t *testing.T) {
	root := t.TempDir()
	repo := gitx.Repo{CanonicalPath: filepath.Join(root, "repo")}
	wt := gitx.Worktree{Path: repo.CanonicalPath, IsMain: true}

	action, err := PlanWorktreeRemove(root, repo, wt, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != DeleteRepoDirectory {
		t.Fatalf("expected repo delete action, got %v", action.Kind)
	}
}

func TestPlanWorktreeRemove(t *testing.T) {
	root := t.TempDir()
	repo := gitx.Repo{CanonicalPath: filepath.Join(root, "repo")}
	wt := gitx.Worktree{Path: filepath.Join(root, "repo", "worktrees", "feature")}

	action, err := PlanWorktreeRemove(root, repo, wt, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != RemoveLinkedWorktree {
		t.Fatalf("unexpected kind: %v", action.Kind)
	}
	if !strings.Contains(action.Prompt, "--force") {
		t.Fatalf("expected force prompt, got %q", action.Prompt)
	}
}

func TestPlanBranchDelete(t *testing.T) {
	root := t.TempDir()
	repo := gitx.Repo{CanonicalPath: root}
	branch := gitx.Branch{Name: "feat/test"}

	action, err := PlanBranchDelete(root, repo, branch, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != DeleteBranch {
		t.Fatalf("unexpected kind: %v", action.Kind)
	}
}

func TestPlanBranchDeleteForce(t *testing.T) {
	root := t.TempDir()
	repo := gitx.Repo{CanonicalPath: root}
	branch := gitx.Branch{Name: "feat/test"}

	action, err := PlanBranchDelete(root, repo, branch, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != DeleteBranchForce {
		t.Fatalf("unexpected kind: %v", action.Kind)
	}
	if !strings.Contains(action.Prompt, "Force delete") {
		t.Fatalf("unexpected prompt: %q", action.Prompt)
	}
}

func TestPlanBranchDeleteRejectsDefault(t *testing.T) {
	root := t.TempDir()
	repo := gitx.Repo{CanonicalPath: root}
	_, err := PlanBranchDelete(root, repo, gitx.Branch{Name: "main", IsDefault: true}, false)
	if err == nil {
		t.Fatalf("expected default branch deletion to fail")
	}
}

func TestPlanStashDrop(t *testing.T) {
	root := t.TempDir()
	repo := gitx.Repo{CanonicalPath: root}
	stash := gitx.Stash{Ref: "stash@{0}"}

	action, err := PlanStashDrop(root, repo, stash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != DropStash {
		t.Fatalf("unexpected kind: %v", action.Kind)
	}
	if action.Target != stash.Ref {
		t.Fatalf("unexpected target: %s", action.Target)
	}
}

func TestExecuteDeleteRepoDirectoryRemovesTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(target, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "nested", "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	err := NewExecutor(&fakeRunner{}).Execute(context.Background(), PendingAction{
		Kind:   DeleteRepoDirectory,
		Root:   root,
		Target: target,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected target to be removed, stat err=%v", statErr)
	}
}

func TestExecuteDeleteRepoDirectoryRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	err := NewExecutor(&fakeRunner{}).Execute(context.Background(), PendingAction{
		Kind:   DeleteRepoDirectory,
		Root:   root,
		Target: outside,
	})
	if err == nil {
		t.Fatalf("expected outside target to fail")
	}
}

func TestExecuteDeleteRepoDirectoriesRemovesAllTargets(t *testing.T) {
	root := t.TempDir()
	targetA := filepath.Join(root, "a")
	targetB := filepath.Join(root, "b")
	for _, target := range []string{targetA, targetB} {
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", target, err)
		}
	}

	err := NewExecutor(&fakeRunner{}).Execute(context.Background(), PendingAction{
		Kind:    DeleteRepoDirectories,
		Root:    root,
		Targets: []string{targetA, targetB},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, target := range []string{targetA, targetB} {
		if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("expected %s removed, stat err=%v", target, statErr)
		}
	}
}

func TestExecuteDeleteRepoDirectoriesStopsOnFirstUnsafeTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "safe")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	err := NewExecutor(&fakeRunner{}).Execute(context.Background(), PendingAction{
		Kind:    DeleteRepoDirectories,
		Root:    root,
		Targets: []string{"/tmp/outside", target},
	})
	if err == nil {
		t.Fatalf("expected unsafe target to fail")
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("expected safe target to remain untouched, stat err=%v", statErr)
	}
}

func TestExecuteRemoveLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	target := filepath.Join(repo, "worktrees", "feature")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runner := &fakeRunner{}

	err := NewExecutor(runner).Execute(context.Background(), PendingAction{
		Kind:     RemoveLinkedWorktree,
		Root:     root,
		RepoPath: repo,
		Target:   target,
		Prompt:   "Remove linked worktree",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected one runner call, got %d", len(runner.calls))
	}
	call := runner.calls[0]
	if call.dir != repo || call.name != "git" || strings.Join(call.args, " ") != "worktree remove "+target {
		t.Fatalf("unexpected runner call: %#v", call)
	}
}

func TestExecuteRemoveLinkedWorktreeForce(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	target := filepath.Join(repo, "worktrees", "feature")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runner := &fakeRunner{}

	err := NewExecutor(runner).Execute(context.Background(), PendingAction{
		Kind:     RemoveLinkedWorktree,
		Root:     root,
		RepoPath: repo,
		Target:   target,
		Prompt:   "Force remove linked worktree via git worktree remove --force ?",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Join(runner.calls[0].args, " "); got != "worktree remove --force "+target {
		t.Fatalf("unexpected args: %q", got)
	}
}

func TestExecuteDeleteBranch(t *testing.T) {
	runner := &fakeRunner{}
	err := NewExecutor(runner).Execute(context.Background(), PendingAction{
		Kind:     DeleteBranch,
		RepoPath: "/tmp/repo",
		Target:   "feature",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Join(runner.calls[0].args, " "); got != "branch -d feature" {
		t.Fatalf("unexpected args: %q", got)
	}
}

func TestExecuteDeleteBranchForce(t *testing.T) {
	runner := &fakeRunner{}
	err := NewExecutor(runner).Execute(context.Background(), PendingAction{
		Kind:     DeleteBranchForce,
		RepoPath: "/tmp/repo",
		Target:   "feature",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Join(runner.calls[0].args, " "); got != "branch -D feature" {
		t.Fatalf("unexpected args: %q", got)
	}
}

func TestExecuteDropStash(t *testing.T) {
	runner := &fakeRunner{}
	err := NewExecutor(runner).Execute(context.Background(), PendingAction{
		Kind:     DropStash,
		RepoPath: "/tmp/repo",
		Target:   "stash@{0}",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Join(runner.calls[0].args, " "); got != "stash drop stash@{0}" {
		t.Fatalf("unexpected args: %q", got)
	}
}

func TestExecutePropagatesRunnerError(t *testing.T) {
	runner := &fakeRunner{err: errors.New("boom")}
	err := NewExecutor(runner).Execute(context.Background(), PendingAction{
		Kind:     DeleteBranch,
		RepoPath: "/tmp/repo",
		Target:   "feature",
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected runner error, got %v", err)
	}
}

func TestExecuteRejectsUnsupportedAction(t *testing.T) {
	err := NewExecutor(&fakeRunner{}).Execute(context.Background(), PendingAction{Kind: ConfirmQuit})
	if err == nil {
		t.Fatalf("expected unsupported action to fail")
	}
}

func TestMin(t *testing.T) {
	if got := min(2, 3); got != 2 {
		t.Fatalf("unexpected min: %d", got)
	}
	if got := min(4, 1); got != 1 {
		t.Fatalf("unexpected min: %d", got)
	}
}

func TestEnsureWithinRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "repo")
	if err := EnsureWithinRoot(root, target); err != nil {
		t.Fatalf("expected target to be allowed: %v", err)
	}
}

func TestEnsureWithinRootAllowsRootItself(t *testing.T) {
	root := t.TempDir()
	if err := EnsureWithinRoot(root, root); err != nil {
		t.Fatalf("expected root itself to be allowed: %v", err)
	}
}

func TestEnsureWithinRootRejectsOutside(t *testing.T) {
	root := t.TempDir()
	if err := EnsureWithinRoot(root, "/tmp/outside"); err == nil {
		t.Fatalf("expected outside path to fail")
	}
}

func TestEnsureWithinRootResolvesSymlinks(t *testing.T) {
	base := t.TempDir()
	rootTarget := filepath.Join(base, "real-root")
	if err := os.MkdirAll(rootTarget, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	rootLink := filepath.Join(base, "root-link")
	if err := os.Symlink(rootTarget, rootLink); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	target := filepath.Join(rootTarget, "repo")
	if err := EnsureWithinRoot(rootLink, target); err != nil {
		t.Fatalf("expected symlinked root to work: %v", err)
	}
}
