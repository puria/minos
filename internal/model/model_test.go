package model

import (
	"testing"

	"github.com/puria/minos/internal/gitx"
)

func TestFilterAndSortDirtyFirst(t *testing.T) {
	repos := []gitx.Repo{
		{DisplayPath: "a", Summary: gitx.RepoSummary{AllWorktreesClean: true, DirtyWorktreeCount: 0}},
		{DisplayPath: "b", Summary: gitx.RepoSummary{AllWorktreesClean: false, DirtyWorktreeCount: 2}},
	}
	indexes := FilterAndSort(repos, "", SortDirtyFirst, false, false)
	if len(indexes) != 2 || indexes[0] != 1 {
		t.Fatalf("unexpected ordering: %#v", indexes)
	}
}

func TestFilterAndSortShowCleanOnly(t *testing.T) {
	repos := []gitx.Repo{
		{DisplayPath: "a", Summary: gitx.RepoSummary{AllWorktreesClean: true, SafeToRemove: false}},
		{DisplayPath: "b", Summary: gitx.RepoSummary{AllWorktreesClean: false, SafeToRemove: false}},
	}
	indexes := FilterAndSort(repos, "", SortPath, true, false)
	if len(indexes) != 1 || indexes[0] != 0 {
		t.Fatalf("unexpected filtering: %#v", indexes)
	}
}

func TestBuildEntitiesIncludesSubmodules(t *testing.T) {
	repo := &gitx.Repo{
		Submodules: []gitx.Submodule{{DisplayPath: "test/bats"}},
	}
	rows := BuildEntities(repo)
	if len(rows) != 1 || rows[0].Kind != EntitySubmodule {
		t.Fatalf("unexpected entities: %#v", rows)
	}
}

func TestSortModeStringAndNextSortMode(t *testing.T) {
	cases := []struct {
		mode SortMode
		want string
		next SortMode
	}{
		{SortPath, "path", SortDirtyFirst},
		{SortDirtyFirst, "dirty", SortStashesFirst},
		{SortStashesFirst, "stashes", SortWorktreesFirst},
		{SortWorktreesFirst, "worktrees", SortPath},
	}
	for _, tc := range cases {
		if got := tc.mode.String(); got != tc.want {
			t.Fatalf("unexpected string for %v: %q", tc.mode, got)
		}
		if got := NextSortMode(tc.mode); got != tc.next {
			t.Fatalf("unexpected next mode for %v: %v", tc.mode, got)
		}
	}
}

func TestFilterAndSortShowSafeOnlyAndFilter(t *testing.T) {
	repos := []gitx.Repo{
		{DisplayPath: "a", DefaultBranch: "main", Summary: gitx.RepoSummary{SafeToRemove: false}},
		{DisplayPath: "b", DefaultBranch: "feat/test", Summary: gitx.RepoSummary{SafeToRemove: true}},
	}
	indexes := FilterAndSort(repos, "feat", SortPath, false, true)
	if len(indexes) != 1 || indexes[0] != 1 {
		t.Fatalf("unexpected filtering: %#v", indexes)
	}
}

func TestFilterAndSortWorktreesFirst(t *testing.T) {
	repos := []gitx.Repo{
		{DisplayPath: "a", Summary: gitx.RepoSummary{LinkedWorktreeCount: 1}},
		{DisplayPath: "b", Summary: gitx.RepoSummary{LinkedWorktreeCount: 3}},
	}
	indexes := FilterAndSort(repos, "", SortWorktreesFirst, false, false)
	if len(indexes) != 2 || indexes[0] != 1 {
		t.Fatalf("unexpected ordering: %#v", indexes)
	}
}

func TestBuildEntitiesNilRepo(t *testing.T) {
	if rows := BuildEntities(nil); rows != nil {
		t.Fatalf("expected nil rows, got %#v", rows)
	}
}
