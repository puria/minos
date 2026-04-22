package gitx

type Repo struct {
	CanonicalPath string
	DisplayPath   string
	DefaultBranch string
	MainBranch    string
	Worktrees     []Worktree
	Branches      []Branch
	Stashes       []Stash
	Submodules    []Submodule
	Summary       RepoSummary
	Errors        []string
}

type Worktree struct {
	Path         string
	Head         string
	Branch       string
	IsMain       bool
	IsCurrent    bool
	Dirty        bool
	StatusShort  string
	Untracked    int
	DisplayLabel string
}

type Branch struct {
	Name              string
	Upstream          string
	Ahead             int
	Behind            int
	IsDefault         bool
	CheckedOut        bool
	CheckedOutPath    string
	MergedIntoDefault *bool
	RecentSubject     string
	RecentDate        string
}

type Stash struct {
	Ref     string
	Subject string
	Stat    string
}

type Submodule struct {
	Path        string
	DisplayPath string
	Commit      string
	Branch      string
	StatusShort string
	Dirty       bool
	Untracked   int
}

type RepoSummary struct {
	CurrentBranchCount         int
	AllWorktreesClean          bool
	MainWorktreeClean          bool
	LinkedWorktreeCount        int
	DirtyWorktreeCount         int
	LocalBranchCount           int
	NonDefaultLocalBranchCount int
	NoUpstreamBranchCount      int
	AheadBranchCount           int
	BehindBranchCount          int
	MergedBranchCount          int
	StashCount                 int
	SafeToRemove               bool
}

type InspectOptions struct {
	SafeRemoveRequiresNoExtraBranches   bool
	SafeRemoveRequiresNoLinkedWorktrees bool
}
