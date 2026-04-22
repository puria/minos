package discovery

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/puria/minos/internal/config"
	"github.com/puria/minos/internal/gitx"
)

type RepoInspector interface {
	Inspect(ctx context.Context, root string, repoPath string, opts gitx.InspectOptions) (gitx.Repo, error)
}

type Scanner struct {
	Inspector RepoInspector
}

func NewScanner(inspector RepoInspector) Scanner {
	return Scanner{Inspector: inspector}
}

func (s Scanner) Scan(ctx context.Context, cfg config.Config) ([]gitx.Repo, error) {
	paths, err := DiscoverCandidates(cfg.Root)
	if err != nil {
		return nil, err
	}

	sem := make(chan struct{}, cfg.GitWorkers)
	results := make([]gitx.Repo, 0, len(paths))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, path := range paths {
		path := path
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			repo, inspectErr := s.Inspector.Inspect(ctx, cfg.Root, path, gitx.InspectOptions{
				SafeRemoveRequiresNoExtraBranches:   cfg.SafeRemoveRequiresNoExtraBranches,
				SafeRemoveRequiresNoLinkedWorktrees: cfg.SafeRemoveRequiresNoLinkedWorktrees,
			})
			if inspectErr != nil {
				repo = gitx.Repo{
					CanonicalPath: path,
					DisplayPath:   gitx.ParseDisplayPath(cfg.Root, path),
					Errors:        []string{inspectErr.Error()},
				}
			}

			mu.Lock()
			results = append(results, repo)
			mu.Unlock()
		}()
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].DisplayPath < results[j].DisplayPath
	})
	return filterSubmoduleRepos(results), nil
}

func DiscoverCandidates(root string) ([]string, error) {
	root, _ = filepath.Abs(root)
	found := map[string]struct{}{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == ".git" {
			wtRoot := filepath.Dir(path)
			canonical, err := CanonicalRepoPath(wtRoot)
			if err == nil {
				found[canonical] = struct{}{}
			}
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(found))
	for path := range found {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func CanonicalRepoPath(worktreeRoot string) (string, error) {
	worktreeRoot, err := filepath.Abs(worktreeRoot)
	if err != nil {
		return "", err
	}
	gitPath := filepath.Join(worktreeRoot, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		return worktreeRoot, nil
	}

	content, err := os.ReadFile(gitPath)
	if err != nil {
		return "", err
	}
	gitdir, err := gitx.ParseGitDirPointer(string(content))
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(worktreeRoot, gitdir)
	}
	gitdir = filepath.Clean(gitdir)

	commondirBytes, err := os.ReadFile(filepath.Join(gitdir, "commondir"))
	if err != nil {
		return worktreeRoot, nil
	}
	commondir := strings.TrimSpace(string(commondirBytes))
	if !filepath.IsAbs(commondir) {
		commondir = filepath.Join(gitdir, commondir)
	}
	commonGitDir := filepath.Clean(commondir)
	return filepath.Dir(commonGitDir), nil
}

func filterSubmoduleRepos(repos []gitx.Repo) []gitx.Repo {
	submodulePaths := make(map[string]struct{})
	for _, repo := range repos {
		for _, submodule := range repo.Submodules {
			submodulePaths[submodule.Path] = struct{}{}
		}
	}

	filtered := make([]gitx.Repo, 0, len(repos))
	for _, repo := range repos {
		if _, ok := submodulePaths[repo.CanonicalPath]; ok {
			continue
		}
		filtered = append(filtered, repo)
	}
	return filtered
}
