package discovery

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/puria/minos/internal/config"
	"github.com/puria/minos/internal/gitx"
)

type fakeInspector struct {
	repos map[string]gitx.Repo
	errs  map[string]error
}

func (f fakeInspector) Inspect(_ context.Context, root string, repoPath string, opts gitx.InspectOptions) (gitx.Repo, error) {
	if err := f.errs[repoPath]; err != nil {
		return gitx.Repo{}, err
	}
	return f.repos[repoPath], nil
}

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

func TestNewScannerAndScan(t *testing.T) {
	root := t.TempDir()
	repoA := filepath.Join(root, "a")
	repoB := filepath.Join(root, "b")
	for _, repo := range []string{repoA, repoB} {
		if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
			t.Fatalf("mkdir .git: %v", err)
		}
	}
	scanner := NewScanner(fakeInspector{
		repos: map[string]gitx.Repo{
			repoA: {CanonicalPath: repoA, DisplayPath: "b"},
		},
		errs: map[string]error{
			repoB: os.ErrNotExist,
		},
	})
	repos, err := scanner.Scan(context.Background(), config.Config{
		Root:                                root,
		GitWorkers:                          2,
		SafeRemoveRequiresNoExtraBranches:   true,
		SafeRemoveRequiresNoLinkedWorktrees: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	displayPaths := []string{repos[0].DisplayPath, repos[1].DisplayPath}
	if !sort.StringsAreSorted(displayPaths) {
		t.Fatalf("expected repos sorted by display path: %#v", displayPaths)
	}
	if len(repos[0].Errors)+len(repos[1].Errors) == 0 {
		t.Fatalf("expected one repo to carry inspect error")
	}
}
