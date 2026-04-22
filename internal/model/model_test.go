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
