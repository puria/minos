package model

import (
	"sort"
	"strings"

	"github.com/puria/minos/internal/gitx"
)

type SortMode int

const (
	SortPath SortMode = iota
	SortDirtyFirst
	SortStashesFirst
	SortWorktreesFirst
)

func (s SortMode) String() string {
	switch s {
	case SortDirtyFirst:
		return "dirty"
	case SortStashesFirst:
		return "stashes"
	case SortWorktreesFirst:
		return "worktrees"
	default:
		return "path"
	}
}

func NextSortMode(s SortMode) SortMode {
	return (s + 1) % 4
}

type EntityKind int

const (
	EntityWorktree EntityKind = iota
	EntityBranch
	EntityStash
	EntitySubmodule
)

type EntityRow struct {
	Kind  EntityKind
	Index int
	Title string
}

func FilterAndSort(repos []gitx.Repo, filter string, mode SortMode, showCleanOnly bool, showSafeOnly bool) []int {
	filter = strings.ToLower(strings.TrimSpace(filter))
	indexes := make([]int, 0, len(repos))
	for i, repo := range repos {
		if showCleanOnly && !repo.Summary.AllWorktreesClean {
			continue
		}
		if showSafeOnly && !repo.Summary.SafeToRemove {
			continue
		}
		if filter != "" {
			blob := strings.ToLower(repo.DisplayPath + " " + repo.DefaultBranch)
			if !strings.Contains(blob, filter) {
				continue
			}
		}
		indexes = append(indexes, i)
	}

	sort.Slice(indexes, func(a, b int) bool {
		left := repos[indexes[a]]
		right := repos[indexes[b]]
		switch mode {
		case SortDirtyFirst:
			if left.Summary.DirtyWorktreeCount != right.Summary.DirtyWorktreeCount {
				return left.Summary.DirtyWorktreeCount > right.Summary.DirtyWorktreeCount
			}
		case SortStashesFirst:
			if left.Summary.StashCount != right.Summary.StashCount {
				return left.Summary.StashCount > right.Summary.StashCount
			}
		case SortWorktreesFirst:
			if left.Summary.LinkedWorktreeCount != right.Summary.LinkedWorktreeCount {
				return left.Summary.LinkedWorktreeCount > right.Summary.LinkedWorktreeCount
			}
		}
		return left.DisplayPath < right.DisplayPath
	})

	return indexes
}

func BuildEntities(repo *gitx.Repo) []EntityRow {
	if repo == nil {
		return nil
	}
	rows := make([]EntityRow, 0, len(repo.Worktrees)+len(repo.Branches)+len(repo.Stashes)+len(repo.Submodules))
	for i, wt := range repo.Worktrees {
		rows = append(rows, EntityRow{Kind: EntityWorktree, Index: i, Title: wt.Branch})
	}
	for i, br := range repo.Branches {
		rows = append(rows, EntityRow{Kind: EntityBranch, Index: i, Title: br.Name})
	}
	for i, st := range repo.Stashes {
		rows = append(rows, EntityRow{Kind: EntityStash, Index: i, Title: st.Ref})
	}
	for i, sm := range repo.Submodules {
		rows = append(rows, EntityRow{Kind: EntitySubmodule, Index: i, Title: sm.DisplayPath})
	}
	return rows
}
