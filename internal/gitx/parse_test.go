package gitx

import "testing"

func TestParseWorktreePorcelain(t *testing.T) {
	out := `worktree /src/github.com/org/repo
HEAD abcdef
branch refs/heads/main

worktree /src/github.com/org/repo-fix
HEAD fedcba
branch refs/heads/fix
`
	worktrees, err := ParseWorktreePorcelain(out, "/src/github.com/org/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(worktrees))
	}
	if !worktrees[0].IsMain {
		t.Fatalf("expected first worktree to be main")
	}
	if worktrees[1].Branch != "fix" {
		t.Fatalf("unexpected branch: %q", worktrees[1].Branch)
	}
}

func TestParseStashList(t *testing.T) {
	stashes := ParseStashList("stash@{0}\tWIP on feat/foo\nstash@{1}\ttemp fix\n")
	if len(stashes) != 2 {
		t.Fatalf("expected 2 stashes, got %d", len(stashes))
	}
	if stashes[1].Subject != "temp fix" {
		t.Fatalf("unexpected subject: %q", stashes[1].Subject)
	}
}

func TestParseTrackCounts(t *testing.T) {
	ahead, behind := ParseTrackCounts("[ahead 2, behind 1]")
	if ahead != 2 || behind != 1 {
		t.Fatalf("unexpected counts: %d %d", ahead, behind)
	}
}

func TestParseSubmoduleStatus(t *testing.T) {
	submodules := ParseSubmoduleStatus(" a751f3d3da4b7db830612322a068a18379c78d09 test/bats (v1.11.0)\n", "/src/repo")
	if len(submodules) != 1 {
		t.Fatalf("expected 1 submodule, got %d", len(submodules))
	}
	if submodules[0].DisplayPath != "test/bats" {
		t.Fatalf("unexpected submodule path: %q", submodules[0].DisplayPath)
	}
}
