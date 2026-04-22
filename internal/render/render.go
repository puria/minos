package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/puria/minos/internal/gitx"
	uitypes "github.com/puria/minos/internal/model"
)

type Styles struct {
	App          lipgloss.Style
	Pane         lipgloss.Style
	PaneFocused  lipgloss.Style
	Header       lipgloss.Style
	Selected     lipgloss.Style
	SafeRepo     lipgloss.Style
	DirtyRepo    lipgloss.Style
	Muted        lipgloss.Style
	Dirty        lipgloss.Style
	Success      lipgloss.Style
	Danger       lipgloss.Style
	BadgeClean   lipgloss.Style
	BadgeDirty   lipgloss.Style
	BadgeInfo    lipgloss.Style
	BadgeWarn    lipgloss.Style
	BadgeBranch  lipgloss.Style
	BadgeMerged  lipgloss.Style
	BadgeWorking lipgloss.Style
	Modal        lipgloss.Style
	Overlay      lipgloss.Style
}

func NewStyles() Styles {
	base := lipgloss.NewStyle().Padding(0, 1)
	return Styles{
		App:          lipgloss.NewStyle().Padding(0, 1),
		Pane:         base.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")),
		PaneFocused:  base.Border(lipgloss.DoubleBorder()).BorderForeground(lipgloss.Color("39")),
		Header:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")),
		Selected:     lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("31")),
		SafeRepo:     lipgloss.NewStyle().Foreground(lipgloss.Color("22")).Background(lipgloss.Color("120")),
		DirtyRepo:    lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("166")),
		Muted:        lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Dirty:        lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		Success:      lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		Danger:       lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("160")).Bold(true),
		BadgeClean:   lipgloss.NewStyle().Foreground(lipgloss.Color("22")).Background(lipgloss.Color("120")).Padding(0, 1),
		BadgeDirty:   lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("166")).Padding(0, 1),
		BadgeInfo:    lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("33")).Padding(0, 1),
		BadgeWarn:    lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("220")).Padding(0, 1),
		BadgeBranch:  lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("60")).Padding(0, 1),
		BadgeMerged:  lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("29")).Padding(0, 1),
		BadgeWorking: lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Padding(0, 1).Bold(true),
		Modal:        lipgloss.NewStyle().Border(lipgloss.ThickBorder()).Padding(1, 2).BorderForeground(lipgloss.Color("160")).Background(lipgloss.Color("235")),
		Overlay:      lipgloss.NewStyle().Background(lipgloss.Color("235")),
	}
}

func RepoBadges(repo gitx.Repo, styles Styles) string {
	badges := make([]string, 0, 6)
	switch {
	case repo.Summary.SafeToRemove:
		badges = append(badges, styles.BadgeClean.Render("clean"))
	case repo.Summary.DirtyWorktreeCount > 0:
		badges = append(badges, styles.BadgeDirty.Render(fmt.Sprintf("%d dirty", repo.Summary.DirtyWorktreeCount)))
	}
	if repo.Summary.LinkedWorktreeCount > 0 {
		badges = append(badges, styles.BadgeInfo.Render(fmt.Sprintf("%d wt", repo.Summary.LinkedWorktreeCount+1)))
	}
	if repo.Summary.StashCount > 0 {
		badges = append(badges, styles.BadgeWarn.Render(fmt.Sprintf("%d st", repo.Summary.StashCount)))
	}
	if repo.Summary.NonDefaultLocalBranchCount > 0 {
		badges = append(badges, styles.BadgeBranch.Render(fmt.Sprintf("%d br", repo.Summary.NonDefaultLocalBranchCount)))
	}
	if repo.Summary.NoUpstreamBranchCount > 0 {
		badges = append(badges, styles.BadgeWarn.Render(fmt.Sprintf("%d no-up", repo.Summary.NoUpstreamBranchCount)))
	}
	if repo.Summary.AheadBranchCount > 0 {
		badges = append(badges, styles.BadgeInfo.Render(fmt.Sprintf("%d ahead", repo.Summary.AheadBranchCount)))
	}
	if repo.Summary.MergedBranchCount > 0 {
		badges = append(badges, styles.BadgeMerged.Render(fmt.Sprintf("%d merged", repo.Summary.MergedBranchCount)))
	}
	return strings.Join(badges, " ")
}

func EntityRow(repo gitx.Repo, entity uitypes.EntityRow) string {
	switch entity.Kind {
	case uitypes.EntityWorktree:
		wt := repo.Worktrees[entity.Index]
		state := "clean"
		if wt.Dirty {
			state = "dirty"
		}
		branch := wt.Branch
		if branch == "" {
			branch = "(detached)"
		}
		return fmt.Sprintf("WT  %-18s %-12s %s", wt.DisplayLabel, branch, state)
	case uitypes.EntityBranch:
		br := repo.Branches[entity.Index]
		tracking := " ="
		switch {
		case br.Upstream == "":
			tracking = " no-upstream"
		case br.Ahead > 0 || br.Behind > 0:
			tracking = fmt.Sprintf(" ↑%d ↓%d", br.Ahead, br.Behind)
		}
		if br.CheckedOut {
			tracking += " current"
		}
		if br.MergedIntoDefault != nil && *br.MergedIntoDefault {
			tracking += " merged"
		}
		return fmt.Sprintf("BR  %-18s%s", br.Name, tracking)
	case uitypes.EntitySubmodule:
		sm := repo.Submodules[entity.Index]
		state := "clean"
		if sm.Dirty {
			state = "dirty"
		}
		branch := sm.Branch
		if branch == "" {
			branch = sm.Commit
		}
		return fmt.Sprintf("SM  %-18s %-12s %s", sm.DisplayPath, branch, state)
	default:
		st := repo.Stashes[entity.Index]
		return fmt.Sprintf("ST  %-12s %s", st.Ref, st.Subject)
	}
}

func RightPane(repo *gitx.Repo, entity *uitypes.EntityRow) string {
	if repo == nil {
		return "No repositories found."
	}
	if entity == nil {
		return fmt.Sprintf("Path: %s\nDefault branch: %s\nWorktrees: %d\nBranches: %d\nStashes: %d",
			repo.CanonicalPath, emptyDash(repo.DefaultBranch), len(repo.Worktrees), len(repo.Branches), len(repo.Stashes))
	}

	switch entity.Kind {
	case uitypes.EntityWorktree:
		wt := repo.Worktrees[entity.Index]
		return fmt.Sprintf("Worktree: %s\nPath: %s\nBranch: %s\nDirty: %t\nUntracked: %d\n\n%s",
			wt.DisplayLabel, wt.Path, emptyDash(wt.Branch), wt.Dirty, wt.Untracked, wt.StatusShort)
	case uitypes.EntityBranch:
		br := repo.Branches[entity.Index]
		merged := "unknown"
		if br.MergedIntoDefault != nil {
			if *br.MergedIntoDefault {
				merged = "yes"
			} else {
				merged = "no"
			}
		}
		return fmt.Sprintf("Branch: %s\nUpstream: %s\nAhead: %d\nBehind: %d\nChecked out: %t\nChecked out path: %s\nMerged into default: %s\nRecent: %s %s",
			br.Name, emptyDash(br.Upstream), br.Ahead, br.Behind, br.CheckedOut, emptyDash(br.CheckedOutPath), merged, br.RecentDate, br.RecentSubject)
	case uitypes.EntitySubmodule:
		sm := repo.Submodules[entity.Index]
		return fmt.Sprintf("Submodule: %s\nPath: %s\nBranch: %s\nCommit: %s\nDirty: %t\nUntracked: %d\n\n%s",
			sm.DisplayPath, sm.Path, emptyDash(sm.Branch), emptyDash(sm.Commit), sm.Dirty, sm.Untracked, sm.StatusShort)
	default:
		st := repo.Stashes[entity.Index]
		if st.Stat == "" {
			return fmt.Sprintf("Stash: %s\nSubject: %s", st.Ref, st.Subject)
		}
		return fmt.Sprintf("Stash: %s\nSubject: %s\n\n%s", st.Ref, st.Subject, st.Stat)
	}
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
