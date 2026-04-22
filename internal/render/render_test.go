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

func TestRepoBadgesShowsCleanAndDirty(t *testing.T) {
	clean := RepoBadges(gitx.Repo{Summary: gitx.RepoSummary{SafeToRemove: true}}, NewStyles())
	if !strings.Contains(clean, "clean") {
		t.Fatalf("expected clean badge, got %q", clean)
	}
	dirty := RepoBadges(gitx.Repo{Summary: gitx.RepoSummary{DirtyWorktreeCount: 2}}, NewStyles())
	if !strings.Contains(dirty, "2 dirty") {
		t.Fatalf("expected dirty badge, got %q", dirty)
	}
}

func TestEntityRowVariants(t *testing.T) {
	repo := gitx.Repo{
		Worktrees:  []gitx.Worktree{{DisplayLabel: "main", Branch: "main", Dirty: true}},
		Branches:   []gitx.Branch{{Name: "feat/test", Upstream: "origin/feat/test", Ahead: 1, Behind: 2, CheckedOut: true}},
		Stashes:    []gitx.Stash{{Ref: "stash@{0}", Subject: "wip"}},
		Submodules: []gitx.Submodule{{DisplayPath: "test/bats", Commit: "abc123"}},
	}
	cases := []struct {
		entity uitypes.EntityRow
		want   string
	}{
		{uitypes.EntityRow{Kind: uitypes.EntityWorktree, Index: 0}, "WT"},
		{uitypes.EntityRow{Kind: uitypes.EntityBranch, Index: 0}, "current"},
		{uitypes.EntityRow{Kind: uitypes.EntityStash, Index: 0}, "wip"},
		{uitypes.EntityRow{Kind: uitypes.EntitySubmodule, Index: 0}, "abc123"},
	}
	for _, tc := range cases {
		if got := EntityRow(repo, tc.entity); !strings.Contains(got, tc.want) {
			t.Fatalf("expected %q in %q", tc.want, got)
		}
	}
}

func TestRightPaneFallbacks(t *testing.T) {
	if got := RightPane(nil, nil); !strings.Contains(got, "No repositories found") {
		t.Fatalf("unexpected nil repo pane: %q", got)
	}
	repo := &gitx.Repo{CanonicalPath: "/tmp/repo", DefaultBranch: "main"}
	if got := RightPane(repo, nil); !strings.Contains(got, "Path: /tmp/repo") {
		t.Fatalf("unexpected repo pane: %q", got)
	}
}

func TestRightPaneWorktreeAndStash(t *testing.T) {
	repo := &gitx.Repo{
		Worktrees: []gitx.Worktree{{DisplayLabel: "main", Path: "/tmp/repo", Branch: "main", StatusShort: "## main"}},
		Stashes:   []gitx.Stash{{Ref: "stash@{0}", Subject: "wip", Stat: " file.go | 2 +-"}},
	}
	if got := RightPane(repo, &uitypes.EntityRow{Kind: uitypes.EntityWorktree, Index: 0}); !strings.Contains(got, "Worktree: main") {
		t.Fatalf("unexpected worktree pane: %q", got)
	}
	if got := RightPane(repo, &uitypes.EntityRow{Kind: uitypes.EntityStash, Index: 0}); !strings.Contains(got, "file.go") {
		t.Fatalf("unexpected stash pane: %q", got)
	}
}
