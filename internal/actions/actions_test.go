package actions

import (
	"path/filepath"
	"testing"

	"github.com/puria/minos/internal/gitx"
)

func TestEnsureWithinRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "repo")
	if err := EnsureWithinRoot(root, target); err != nil {
		t.Fatalf("expected target to be allowed: %v", err)
	}
}

func TestEnsureWithinRootRejectsOutside(t *testing.T) {
	root := t.TempDir()
	if err := EnsureWithinRoot(root, "/tmp/outside"); err == nil {
		t.Fatalf("expected outside path to fail")
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

func TestPlanRepoDeleteMany(t *testing.T) {
	root := t.TempDir()
	repoA := filepath.Join(root, "a")
	repoB := filepath.Join(root, "b")
	action, err := PlanRepoDeleteMany(root, []gitx.Repo{
		{CanonicalPath: repoA},
		{CanonicalPath: repoB},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != DeleteRepoDirectories {
		t.Fatalf("unexpected kind: %v", action.Kind)
	}
	if len(action.Targets) != 2 {
		t.Fatalf("unexpected targets: %#v", action.Targets)
	}
}
