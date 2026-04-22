package gitx

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

type Inspector struct {
	Runner Runner
}

func NewInspector(r Runner) Inspector {
	return Inspector{Runner: r}
}

func (i Inspector) Inspect(ctx context.Context, root string, repoPath string, opts InspectOptions) (Repo, error) {
	repoPath, _ = filepath.Abs(repoPath)
	repo := Repo{
		CanonicalPath: repoPath,
		DisplayPath:   ParseDisplayPath(root, repoPath),
	}

	defaultBranch, err := i.detectDefaultBranch(ctx, repoPath)
	if err != nil {
		repo.Errors = append(repo.Errors, err.Error())
	}
	repo.DefaultBranch = defaultBranch

	worktreeOut, err := i.Runner.Run(ctx, repoPath, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return repo, err
	}
	worktrees, err := ParseWorktreePorcelain(worktreeOut, repoPath)
	if err != nil {
		return repo, err
	}

	checkedOutByBranch := map[string]string{}
	for idx := range worktrees {
		worktrees[idx].DisplayLabel = displayWorktreeLabel(repoPath, worktrees[idx].Path)
		if worktrees[idx].Branch != "" && worktrees[idx].IsMain {
			repo.MainBranch = worktrees[idx].Branch
		}
	}
	var (
		worktreeMu sync.Mutex
		errsMu     sync.Mutex
	)
	worktreeGroup, worktreeCtx := errgroup.WithContext(ctx)
	worktreeGroup.SetLimit(min(8, max(1, len(worktrees))))
	for idx := range worktrees {
		idx := idx
		worktreeGroup.Go(func() error {
			statusOut, statusErr := i.Runner.Run(worktreeCtx, worktrees[idx].Path, "git", "status", "--short", "--branch")
			worktreeMu.Lock()
			defer worktreeMu.Unlock()
			if statusErr != nil {
				errsMu.Lock()
				repo.Errors = append(repo.Errors, statusErr.Error())
				errsMu.Unlock()
				return nil
			}
			worktrees[idx].StatusShort = strings.TrimSpace(statusOut)
			worktrees[idx].Dirty = worktreeDirty(worktrees[idx].StatusShort)
			worktrees[idx].Untracked = countUntracked(worktrees[idx].StatusShort)
			if worktrees[idx].Branch != "" {
				checkedOutByBranch[worktrees[idx].Branch] = worktrees[idx].Path
			}
			return nil
		})
	}
	if err := worktreeGroup.Wait(); err != nil {
		return repo, err
	}
	repo.Worktrees = worktrees

	var branchOut, stashOut, submoduleOut string
	mergedSet := map[string]bool{}
	metaGroup, metaCtx := errgroup.WithContext(ctx)
	metaGroup.Go(func() error {
		out, runErr := i.Runner.Run(metaCtx, repoPath, "git", "for-each-ref",
			"--format=%(refname:short)\t%(upstream:short)\t%(upstream:track)\t%(committerdate:short)\t%(contents:subject)",
			"refs/heads")
		if runErr != nil {
			return runErr
		}
		branchOut = out
		return nil
	})
	metaGroup.Go(func() error {
		out, runErr := i.Runner.Run(metaCtx, repoPath, "git", "stash", "list", "--format=%gd\t%gs")
		if runErr == nil {
			stashOut = out
			return nil
		}
		if !strings.Contains(runErr.Error(), "not a git repository") {
			errsMu.Lock()
			repo.Errors = append(repo.Errors, runErr.Error())
			errsMu.Unlock()
		}
		return nil
	})
	metaGroup.Go(func() error {
		out, runErr := i.Runner.Run(metaCtx, repoPath, "git", "submodule", "status")
		if runErr == nil {
			submoduleOut = out
			return nil
		}
		if !strings.Contains(runErr.Error(), "No submodule mapping found") {
			errsMu.Lock()
			repo.Errors = append(repo.Errors, runErr.Error())
			errsMu.Unlock()
		}
		return nil
	})
	if defaultBranch != "" {
		metaGroup.Go(func() error {
			mergedOut, mergedErr := i.Runner.Run(metaCtx, repoPath, "git", "branch", "--format=%(refname:short)", "--merged", defaultBranch)
			if mergedErr != nil {
				errsMu.Lock()
				repo.Errors = append(repo.Errors, mergedErr.Error())
				errsMu.Unlock()
				return nil
			}
			for _, line := range strings.Split(strings.TrimSpace(mergedOut), "\n") {
				line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "*"))
				if line != "" {
					mergedSet[line] = true
				}
			}
			return nil
		})
	}
	if err := metaGroup.Wait(); err != nil {
		return repo, err
	}

	for _, line := range strings.Split(strings.TrimSpace(branchOut), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		for len(parts) < 5 {
			parts = append(parts, "")
		}
		ahead, behind := ParseTrackCounts(parts[2])
		branch := Branch{
			Name:           parts[0],
			Upstream:       parts[1],
			Ahead:          ahead,
			Behind:         behind,
			IsDefault:      parts[0] == defaultBranch,
			CheckedOut:     checkedOutByBranch[parts[0]] != "",
			CheckedOutPath: checkedOutByBranch[parts[0]],
			RecentDate:     parts[3],
			RecentSubject:  parts[4],
		}
		if defaultBranch != "" && parts[0] != defaultBranch {
			merged := mergedSet[parts[0]]
			branch.MergedIntoDefault = &merged
		}
		repo.Branches = append(repo.Branches, branch)
	}
	slices.SortFunc(repo.Branches, func(a, b Branch) int {
		if a.IsDefault != b.IsDefault {
			if a.IsDefault {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})

	if stashOut != "" {
		repo.Stashes = ParseStashList(stashOut)
	}

	if submoduleOut != "" {
		repo.Submodules = ParseSubmoduleStatus(submoduleOut, repoPath)
		submoduleGroup, submoduleCtx := errgroup.WithContext(ctx)
		submoduleGroup.SetLimit(min(8, max(1, len(repo.Submodules))))
		for idx := range repo.Submodules {
			idx := idx
			submoduleGroup.Go(func() error {
				statusOut, statusErr := i.Runner.Run(submoduleCtx, repo.Submodules[idx].Path, "git", "status", "--short", "--branch")
				branchOut, branchErr := i.Runner.Run(submoduleCtx, repo.Submodules[idx].Path, "git", "symbolic-ref", "--quiet", "--short", "HEAD")
				worktreeMu.Lock()
				defer worktreeMu.Unlock()
				if statusErr == nil {
					repo.Submodules[idx].StatusShort = strings.TrimSpace(statusOut)
					repo.Submodules[idx].Dirty = worktreeDirty(repo.Submodules[idx].StatusShort)
					repo.Submodules[idx].Untracked = countUntracked(repo.Submodules[idx].StatusShort)
				}
				if branchErr == nil {
					repo.Submodules[idx].Branch = strings.TrimSpace(branchOut)
				}
				return nil
			})
		}
		if err := submoduleGroup.Wait(); err != nil {
			return repo, err
		}
	}

	repo.Summary = Summarize(repo, opts)
	return repo, nil
}

func (i Inspector) PopulateStashStat(ctx context.Context, repoPath string, ref string) (string, error) {
	out, err := i.Runner.Run(ctx, repoPath, "git", "stash", "show", "--stat", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (i Inspector) detectDefaultBranch(ctx context.Context, repoPath string) (string, error) {
	out, err := i.Runner.Run(ctx, repoPath, "git", "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	if err == nil {
		ref := strings.TrimSpace(out)
		return strings.TrimPrefix(ref, "refs/remotes/origin/"), nil
	}

	branchOut, branchErr := i.Runner.Run(ctx, repoPath, "git", "symbolic-ref", "--quiet", "--short", "HEAD")
	if branchErr == nil {
		return strings.TrimSpace(branchOut), nil
	}

	headsOut, headsErr := i.Runner.Run(ctx, repoPath, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if headsErr != nil {
		return "", fmt.Errorf("detect default branch: %w", headsErr)
	}
	heads := strings.Fields(headsOut)
	for _, candidate := range []string{"main", "master"} {
		if slices.Contains(heads, candidate) {
			return candidate, nil
		}
	}
	if len(heads) > 0 {
		return heads[0], nil
	}
	return "", nil
}

func Summarize(repo Repo, opts InspectOptions) RepoSummary {
	summary := RepoSummary{
		AllWorktreesClean: true,
		MainWorktreeClean: true,
		LocalBranchCount:  len(repo.Branches),
		StashCount:        len(repo.Stashes),
	}
	for _, wt := range repo.Worktrees {
		if wt.IsMain {
			summary.MainWorktreeClean = !wt.Dirty
		} else {
			summary.LinkedWorktreeCount++
		}
		if wt.Dirty {
			summary.AllWorktreesClean = false
			summary.DirtyWorktreeCount++
		}
	}
	for _, br := range repo.Branches {
		if !br.IsDefault {
			summary.NonDefaultLocalBranchCount++
			if br.Upstream == "" {
				summary.NoUpstreamBranchCount++
			}
			if br.Ahead > 0 {
				summary.AheadBranchCount++
			}
			if br.Behind > 0 {
				summary.BehindBranchCount++
			}
			if br.MergedIntoDefault != nil && *br.MergedIntoDefault {
				summary.MergedBranchCount++
			}
		}
	}

	safe := summary.AllWorktreesClean && summary.StashCount == 0
	if opts.SafeRemoveRequiresNoLinkedWorktrees {
		safe = safe && summary.LinkedWorktreeCount == 0
	}
	if opts.SafeRemoveRequiresNoExtraBranches {
		safe = safe && summary.NonDefaultLocalBranchCount == 0
	}
	summary.SafeToRemove = safe
	return summary
}

func worktreeDirty(status string) bool {
	lines := strings.Split(strings.TrimSpace(status), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "## ") {
			continue
		}
		return true
	}
	return false
}

func countUntracked(status string) int {
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "?? ") {
			count++
		}
	}
	return count
}

func displayWorktreeLabel(repoPath string, wtPath string) string {
	if samePath(repoPath, wtPath) {
		return "main"
	}
	rel, err := filepath.Rel(repoPath, wtPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return wtPath
	}
	return rel
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
