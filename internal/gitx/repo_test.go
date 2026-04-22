package gitx

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type inspectorRunner struct {
	mu        sync.Mutex
	responses map[string]runnerResponse
}

type runnerResponse struct {
	out string
	err error
}

func (r *inspectorRunner) Run(_ context.Context, dir string, name string, args ...string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := runnerKey(dir, name, args...)
	resp, ok := r.responses[key]
	if !ok {
		return "", fmt.Errorf("unexpected command: %s", key)
	}
	return resp.out, resp.err
}

func runnerKey(dir string, name string, args ...string) string {
	return dir + "|" + name + "|" + strings.Join(args, " ")
}

func TestSummarizeSafeToRemove(t *testing.T) {
	repo := Repo{
		DefaultBranch: "main",
		Worktrees: []Worktree{
			{Path: "/tmp/repo", IsMain: true, Branch: "main"},
		},
		Branches: []Branch{
			{Name: "main", IsDefault: true},
		},
	}
	summary := Summarize(repo, InspectOptions{
		SafeRemoveRequiresNoExtraBranches:   true,
		SafeRemoveRequiresNoLinkedWorktrees: true,
	})
	if !summary.SafeToRemove {
		t.Fatalf("expected repo to be safe")
	}
}

func TestSummarizeUnsafeWithExtraBranch(t *testing.T) {
	repo := Repo{
		DefaultBranch: "main",
		Worktrees: []Worktree{
			{Path: "/tmp/repo", IsMain: true, Branch: "main"},
		},
		Branches: []Branch{
			{Name: "main", IsDefault: true},
			{Name: "feat/x"},
		},
	}
	summary := Summarize(repo, InspectOptions{
		SafeRemoveRequiresNoExtraBranches:   true,
		SafeRemoveRequiresNoLinkedWorktrees: true,
	})
	if summary.SafeToRemove {
		t.Fatalf("expected repo to be unsafe")
	}
}

func TestSummarizeBranchStateCounts(t *testing.T) {
	merged := true
	repo := Repo{
		DefaultBranch: "main",
		Branches: []Branch{
			{Name: "main", IsDefault: true},
			{Name: "local-only"},
			{Name: "ahead", Upstream: "origin/ahead", Ahead: 2},
			{Name: "merged", Upstream: "origin/merged", MergedIntoDefault: &merged},
		},
	}
	summary := Summarize(repo, InspectOptions{})
	if summary.NonDefaultLocalBranchCount != 3 || summary.NoUpstreamBranchCount != 1 || summary.AheadBranchCount != 1 || summary.MergedBranchCount != 1 {
		t.Fatalf("unexpected summary counts: %#v", summary)
	}
}

func TestInspect(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "repo")
	featurePath := filepath.Join(repoPath, "worktrees", "feature")
	submodulePath := filepath.Join(repoPath, "test", "bats")
	runner := &inspectorRunner{
		responses: map[string]runnerResponse{
			runnerKey(repoPath, "git", "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"): {out: "refs/remotes/origin/main\n"},
			runnerKey(repoPath, "git", "worktree", "list", "--porcelain"): {out: fmt.Sprintf(`worktree %s
HEAD abc
branch refs/heads/main

worktree %s
HEAD def
branch refs/heads/feature
`, repoPath, featurePath)},
			runnerKey(repoPath, "git", "status", "--short", "--branch"):    {out: "## main\n"},
			runnerKey(featurePath, "git", "status", "--short", "--branch"): {out: "## feature\n?? new.txt\n M file.go\n"},
			runnerKey(repoPath, "git", "for-each-ref", "--format=%(refname:short)\t%(upstream:short)\t%(upstream:track)\t%(committerdate:short)\t%(contents:subject)", "refs/heads"): {out: "main\torigin/main\t\t2026-04-23\tmain work\nfeature\torigin/feature\t[ahead 2, behind 1]\t2026-04-22\tfeature work\n"},
			runnerKey(repoPath, "git", "stash", "list", "--format=%gd\t%gs"):                      {out: "stash@{0}\tWIP feature\n"},
			runnerKey(repoPath, "git", "submodule", "status"):                                     {out: " a751f3d3da4b7db830612322a068a18379c78d09 test/bats (v1.11.0)\n"},
			runnerKey(repoPath, "git", "branch", "--format=%(refname:short)", "--merged", "main"): {out: "main\nfeature\n"},
			runnerKey(submodulePath, "git", "status", "--short", "--branch"):                      {out: "## main\n?? generated.txt\n"},
			runnerKey(submodulePath, "git", "symbolic-ref", "--quiet", "--short", "HEAD"):         {out: "main\n"},
		},
	}

	repo, err := NewInspector(runner).Inspect(context.Background(), root, repoPath, InspectOptions{
		SafeRemoveRequiresNoExtraBranches:   true,
		SafeRemoveRequiresNoLinkedWorktrees: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.DefaultBranch != "main" || repo.MainBranch != "main" {
		t.Fatalf("unexpected branch info: %#v", repo)
	}
	if len(repo.Worktrees) != 2 || !repo.Worktrees[1].Dirty || repo.Worktrees[1].Untracked != 1 {
		t.Fatalf("unexpected worktrees: %#v", repo.Worktrees)
	}
	if len(repo.Branches) != 2 || !repo.Branches[1].CheckedOut || repo.Branches[1].CheckedOutPath != featurePath {
		t.Fatalf("unexpected branches: %#v", repo.Branches)
	}
	if len(repo.Stashes) != 1 || repo.Stashes[0].Ref != "stash@{0}" {
		t.Fatalf("unexpected stashes: %#v", repo.Stashes)
	}
	if len(repo.Submodules) != 1 || repo.Submodules[0].Branch != "main" || !repo.Submodules[0].Dirty {
		t.Fatalf("unexpected submodules: %#v", repo.Submodules)
	}
	if repo.Summary.DirtyWorktreeCount != 1 || repo.Summary.LinkedWorktreeCount != 1 {
		t.Fatalf("unexpected summary: %#v", repo.Summary)
	}
}

func TestPopulateStashStat(t *testing.T) {
	repoPath := t.TempDir()
	runner := &inspectorRunner{
		responses: map[string]runnerResponse{
			runnerKey(repoPath, "git", "stash", "show", "--stat", "stash@{0}"): {out: " file.go | 2 +- \n"},
		},
	}
	out, err := NewInspector(runner).PopulateStashStat(context.Background(), repoPath, "stash@{0}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "file.go | 2 +-" {
		t.Fatalf("unexpected stat output: %q", out)
	}
}

func TestDetectDefaultBranchFallbacks(t *testing.T) {
	repoPath := t.TempDir()
	runner := &inspectorRunner{
		responses: map[string]runnerResponse{
			runnerKey(repoPath, "git", "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"): {err: errors.New("missing origin head")},
			runnerKey(repoPath, "git", "symbolic-ref", "--quiet", "--short", "HEAD"):          {out: "feature\n"},
		},
	}
	got, err := NewInspector(runner).detectDefaultBranch(context.Background(), repoPath)
	if err != nil || got != "feature" {
		t.Fatalf("unexpected default branch: %q err=%v", got, err)
	}
}

func TestDetectDefaultBranchFallsBackToMainAndFirstHead(t *testing.T) {
	repoPath := t.TempDir()
	runner := &inspectorRunner{
		responses: map[string]runnerResponse{
			runnerKey(repoPath, "git", "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"):     {err: errors.New("missing origin head")},
			runnerKey(repoPath, "git", "symbolic-ref", "--quiet", "--short", "HEAD"):              {err: errors.New("detached")},
			runnerKey(repoPath, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads"): {out: "topic\nmain\n"},
		},
	}
	got, err := NewInspector(runner).detectDefaultBranch(context.Background(), repoPath)
	if err != nil || got != "main" {
		t.Fatalf("unexpected default branch: %q err=%v", got, err)
	}

	runner.responses[runnerKey(repoPath, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads")] = runnerResponse{out: "topic\nzzz\n"}
	got, err = NewInspector(runner).detectDefaultBranch(context.Background(), repoPath)
	if err != nil || got != "topic" {
		t.Fatalf("unexpected first-head fallback: %q err=%v", got, err)
	}
}

func TestDetectDefaultBranchPropagatesHeadLookupError(t *testing.T) {
	repoPath := t.TempDir()
	runner := &inspectorRunner{
		responses: map[string]runnerResponse{
			runnerKey(repoPath, "git", "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"):     {err: errors.New("missing origin head")},
			runnerKey(repoPath, "git", "symbolic-ref", "--quiet", "--short", "HEAD"):              {err: errors.New("detached")},
			runnerKey(repoPath, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads"): {err: errors.New("boom")},
		},
	}
	if _, err := NewInspector(runner).detectDefaultBranch(context.Background(), repoPath); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWorktreeHelpers(t *testing.T) {
	if !worktreeDirty("## main\n M file.go") {
		t.Fatalf("expected dirty worktree")
	}
	if worktreeDirty("## main\n") {
		t.Fatalf("expected clean worktree")
	}
	if got := countUntracked("## main\n?? a\n M b\n?? c"); got != 2 {
		t.Fatalf("unexpected untracked count: %d", got)
	}
	if got := displayWorktreeLabel("/tmp/repo", "/tmp/repo"); got != "main" {
		t.Fatalf("unexpected main label: %q", got)
	}
	if got := displayWorktreeLabel("/tmp/repo", "/tmp/repo/worktrees/feature"); got != "worktrees/feature" {
		t.Fatalf("unexpected relative label: %q", got)
	}
	if min(1, 2) != 1 || max(1, 2) != 2 {
		t.Fatalf("unexpected min/max")
	}
}
