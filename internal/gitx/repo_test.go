package gitx

import "testing"

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
