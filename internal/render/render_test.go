package render

import (
	"strings"
	"testing"

	"github.com/puria/minos/internal/gitx"
	uitypes "github.com/puria/minos/internal/model"
)

func TestRightPaneBranchSnapshot(t *testing.T) {
	repo := &gitx.Repo{
		Branches: []gitx.Branch{{Name: "feat/test", Upstream: "origin/feat/test", Ahead: 2, RecentDate: "2026-04-22", RecentSubject: "test commit"}},
	}
	entity := &uitypes.EntityRow{Kind: uitypes.EntityBranch, Index: 0}
	out := RightPane(repo, entity)
	if !strings.Contains(out, "Branch: feat/test") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRightPaneSubmoduleSnapshot(t *testing.T) {
	repo := &gitx.Repo{
		Submodules: []gitx.Submodule{{DisplayPath: "test/bats", Path: "/src/repo/test/bats", Branch: "main", Commit: "abc123", StatusShort: "## main"}},
	}
	entity := &uitypes.EntityRow{Kind: uitypes.EntitySubmodule, Index: 0}
	out := RightPane(repo, entity)
	if !strings.Contains(out, "Submodule: test/bats") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRepoBadgesShowsBranchBadges(t *testing.T) {
	out := RepoBadges(gitx.Repo{
		DisplayPath: "org/repo",
		Summary: gitx.RepoSummary{
			NonDefaultLocalBranchCount: 3,
			NoUpstreamBranchCount:      1,
			AheadBranchCount:           2,
			MergedBranchCount:          1,
		},
	}, NewStyles())
	for _, want := range []string{"3 br", "1 no-up", "2 ahead", "1 merged"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in %q", want, out)
		}
	}
}
