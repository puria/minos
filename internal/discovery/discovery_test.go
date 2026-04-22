package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/puria/minos/internal/gitx"
)

func TestDiscoverCandidatesDedupesLinkedWorktrees(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "github.com", "org", "repo")
	worktree := filepath.Join(root, "github.com", "org", "repo-fix")
	gitdir := filepath.Join(repo, ".git")
	worktreeGitdir := filepath.Join(gitdir, "worktrees", "repo-fix")

	if err := os.MkdirAll(worktreeGitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktreeGitdir, "commondir"), []byte("../.."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+worktreeGitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := DiscoverCandidates(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(paths))
	}
	if paths[0] != repo {
		t.Fatalf("unexpected canonical path: %s", paths[0])
	}
}

func TestFilterSubmoduleRepos(t *testing.T) {
	repos := []gitx.Repo{
		{
			CanonicalPath: "/src/parent",
			Submodules: []gitx.Submodule{
				{Path: "/src/parent/test/bats", DisplayPath: "test/bats"},
			},
		},
		{CanonicalPath: "/src/parent/test/bats"},
	}
	filtered := filterSubmoduleRepos(repos)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 repo after filtering, got %d", len(filtered))
	}
	if filtered[0].CanonicalPath != "/src/parent" {
		t.Fatalf("unexpected repo retained: %s", filtered[0].CanonicalPath)
	}
}
