package actions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/puria/minos/internal/gitx"
)

type Kind int

const (
	DeleteRepoDirectory Kind = iota
	DeleteRepoDirectories
	ConfirmQuit
	RemoveLinkedWorktree
	DeleteBranch
	DeleteBranchForce
	DropStash
)

type PendingAction struct {
	Kind     Kind
	Root     string
	RepoPath string
	Target   string
	Targets  []string
	Prompt   string
}

type Executor struct {
	Runner gitx.Runner
}

func NewExecutor(r gitx.Runner) Executor {
	return Executor{Runner: r}
}

func PlanRepoDelete(root string, repo gitx.Repo) (PendingAction, error) {
	if err := EnsureWithinRoot(root, repo.CanonicalPath); err != nil {
		return PendingAction{}, err
	}
	return PendingAction{
		Kind:     DeleteRepoDirectory,
		Root:     root,
		RepoPath: repo.CanonicalPath,
		Target:   repo.CanonicalPath,
		Prompt:   fmt.Sprintf("Delete directory %s ?", repo.CanonicalPath),
	}, nil
}

func PlanRepoDeleteMany(root string, repos []gitx.Repo) (PendingAction, error) {
	if len(repos) == 0 {
		return PendingAction{}, fmt.Errorf("no repos selected")
	}
	targets := make([]string, 0, len(repos))
	display := make([]string, 0, min(len(repos), 5))
	for i, repo := range repos {
		if err := EnsureWithinRoot(root, repo.CanonicalPath); err != nil {
			return PendingAction{}, err
		}
		targets = append(targets, repo.CanonicalPath)
		if i < 5 {
			display = append(display, repo.CanonicalPath)
		}
	}
	prompt := fmt.Sprintf("Delete %d directories?\n\n%s", len(targets), strings.Join(display, "\n"))
	if len(targets) > len(display) {
		prompt += fmt.Sprintf("\n... and %d more", len(targets)-len(display))
	}
	return PendingAction{
		Kind:    DeleteRepoDirectories,
		Root:    root,
		Targets: targets,
		Prompt:  prompt,
	}, nil
}

func PlanWorktreeRemove(root string, repo gitx.Repo, wt gitx.Worktree, force bool) (PendingAction, error) {
	if wt.IsMain {
		return PlanRepoDelete(root, repo)
	}
	if err := EnsureWithinRoot(root, wt.Path); err != nil {
		return PendingAction{}, err
	}
	kind := RemoveLinkedWorktree
	prompt := fmt.Sprintf("Remove linked worktree %s via git worktree remove ?", wt.Path)
	if force {
		prompt = fmt.Sprintf("Force remove linked worktree %s via git worktree remove --force ?", wt.Path)
	}
	return PendingAction{
		Kind:     kind,
		Root:     root,
		RepoPath: repo.CanonicalPath,
		Target:   wt.Path,
		Prompt:   prompt,
	}, nil
}

func PlanBranchDelete(root string, repo gitx.Repo, branch gitx.Branch, force bool) (PendingAction, error) {
	if branch.IsDefault {
		return PendingAction{}, fmt.Errorf("refusing to delete default branch %q", branch.Name)
	}
	if err := EnsureWithinRoot(root, repo.CanonicalPath); err != nil {
		return PendingAction{}, err
	}
	kind := DeleteBranch
	prompt := fmt.Sprintf("Delete local branch %s ?", branch.Name)
	if force {
		kind = DeleteBranchForce
		prompt = fmt.Sprintf("Force delete local branch %s ?", branch.Name)
	}
	return PendingAction{
		Kind:     kind,
		Root:     root,
		RepoPath: repo.CanonicalPath,
		Target:   branch.Name,
		Prompt:   prompt,
	}, nil
}

func PlanStashDrop(root string, repo gitx.Repo, stash gitx.Stash) (PendingAction, error) {
	if err := EnsureWithinRoot(root, repo.CanonicalPath); err != nil {
		return PendingAction{}, err
	}
	return PendingAction{
		Kind:     DropStash,
		Root:     root,
		RepoPath: repo.CanonicalPath,
		Target:   stash.Ref,
		Prompt:   fmt.Sprintf("Drop stash %s ?", stash.Ref),
	}, nil
}

func (e Executor) Execute(ctx context.Context, action PendingAction) error {
	switch action.Kind {
	case DeleteRepoDirectory:
		if err := EnsureWithinRoot(action.Root, action.Target); err != nil {
			return err
		}
		return os.RemoveAll(action.Target)
	case DeleteRepoDirectories:
		for _, target := range action.Targets {
			if err := EnsureWithinRoot(action.Root, target); err != nil {
				return err
			}
			if err := os.RemoveAll(target); err != nil {
				return err
			}
		}
		return nil
	case RemoveLinkedWorktree:
		if err := EnsureWithinRoot(action.Root, action.Target); err != nil {
			return err
		}
		args := []string{"worktree", "remove", action.Target}
		if strings.Contains(action.Prompt, "--force") {
			args = []string{"worktree", "remove", "--force", action.Target}
		}
		_, err := e.Runner.Run(ctx, action.RepoPath, "git", args...)
		return err
	case DeleteBranch:
		_, err := e.Runner.Run(ctx, action.RepoPath, "git", "branch", "-d", action.Target)
		return err
	case DeleteBranchForce:
		_, err := e.Runner.Run(ctx, action.RepoPath, "git", "branch", "-D", action.Target)
		return err
	case DropStash:
		_, err := e.Runner.Run(ctx, action.RepoPath, "git", "stash", "drop", action.Target)
		return err
	default:
		return fmt.Errorf("unsupported action")
	}
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func EnsureWithinRoot(root string, target string) error {
	rootResolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	targetResolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		targetResolved, err = filepath.Abs(target)
		if err != nil {
			return err
		}
	}
	if targetResolved == rootResolved {
		return nil
	}
	prefix := rootResolved + string(os.PathSeparator)
	if !strings.HasPrefix(targetResolved, prefix) {
		return fmt.Errorf("path %s is outside root %s", targetResolved, rootResolved)
	}
	return nil
}
