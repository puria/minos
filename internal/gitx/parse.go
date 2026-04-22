package gitx

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func ParseWorktreePorcelain(out string, mainPath string) ([]Worktree, error) {
	blocks := strings.Split(strings.TrimSpace(out), "\n\n")
	worktrees := make([]Worktree, 0, len(blocks))

	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		var wt Worktree
		scanner := bufio.NewScanner(strings.NewReader(block))
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "worktree "):
				wt.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			case strings.HasPrefix(line, "HEAD "):
				wt.Head = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
			case strings.HasPrefix(line, "branch "):
				branch := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
				wt.Branch = strings.TrimPrefix(branch, "refs/heads/")
			case strings.TrimSpace(line) == "bare":
			case strings.TrimSpace(line) == "detached":
			case strings.TrimSpace(line) == "locked":
			case strings.HasPrefix(line, "prunable "):
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		if wt.Path == "" {
			return nil, fmt.Errorf("invalid worktree block: %q", block)
		}
		wt.IsMain = samePath(wt.Path, mainPath)
		worktrees = append(worktrees, wt)
	}

	return worktrees, nil
}

func ParseStashList(out string) []Stash {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	stashes := make([]Stash, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		stash := Stash{Ref: parts[0]}
		if len(parts) == 2 {
			stash.Subject = parts[1]
		}
		stashes = append(stashes, stash)
	}
	return stashes
}

func ParseGitDirPointer(content string) (string, error) {
	line := strings.TrimSpace(content)
	if !strings.HasPrefix(line, "gitdir:") {
		return "", fmt.Errorf("invalid gitdir pointer")
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "gitdir:")), nil
}

func ParseTrackCounts(track string) (ahead int, behind int) {
	track = strings.TrimSpace(track)
	if track == "" || track == "[gone]" {
		return 0, 0
	}
	track = strings.TrimPrefix(track, "[")
	track = strings.TrimSuffix(track, "]")
	for _, part := range strings.Split(track, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "ahead ") {
			ahead, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(part, "ahead ")))
		}
		if strings.HasPrefix(part, "behind ") {
			behind, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(part, "behind ")))
		}
	}
	return ahead, behind
}

func ParseDisplayPath(root string, repoPath string) string {
	rel, err := filepath.Rel(root, repoPath)
	if err != nil {
		return repoPath
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return repoPath
	}
	return filepath.ToSlash(rel)
}

func ParseSubmoduleStatus(out string, repoPath string) []Submodule {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	submodules := make([]Submodule, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) < 42 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		commit := strings.TrimLeft(fields[0], "-+U ")
		relPath := fields[1]
		path := filepath.Join(repoPath, filepath.FromSlash(relPath))
		submodules = append(submodules, Submodule{
			Path:        path,
			DisplayPath: filepath.ToSlash(relPath),
			Commit:      commit,
		})
	}
	return submodules
}

func samePath(a string, b string) bool {
	aa, _ := filepath.Abs(a)
	bb, _ := filepath.Abs(b)
	return aa == bb
}
