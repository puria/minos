# minos

`minos` is a fullscreen terminal UI for reviewing a large source tree that contains many Git repositories and linked worktrees. It is built for cleanup: scan a root like `~/src`, inspect worktrees, branches, and stashes, then remove repos or worktrees safely with explicit confirmation.

## Features

- Three-pane TUI for repositories, repo entities, and status/details
- Recursive discovery of normal repos and linked worktrees
- Conservative safe-to-remove classification
- Refresh, filter, sorting, and keyboard navigation
- Confirmation modal for destructive actions
- Safe branch deletion, stash drop, and `git worktree remove`
- Optional external summarizer integration for selected worktrees

## Install

### With mise

```bash
mise use -g github:puria/minos@latest
```

### With Go

```bash
go install github.com/puria/minos/cmd/minos@latest
```

## Build

```bash
go build ./cmd/minos
```

Or with the project task:

```bash
task build
```

Run against the default `~/src` root:

```bash
./bin/minos
```

Run against a custom root:

```bash
./bin/minos ~/src
./bin/minos --root ~/src
```

## Keybindings

- `q`: quit
- `tab` / `shift-tab`: move focus across panes
- `j` / `k` or arrows: move selection or scroll the status pane
- `/`: open filter input
- `s`: cycle sorting modes
- `r`: refresh discovery and repo summaries
- `d`: delete/remove selected entity with confirmation
- `D`: force action when applicable, still with confirmation
- `a`, `e`, or `.`: explain selected repo/worktree/branch with the optional external summarizer
- `?`: toggle help

## Safety Model

Safe-to-remove is conservative by default. A repo is only classified safe if:

- all discovered worktrees are clean
- no stashes exist
- no linked worktrees exist
- no local branches exist beyond the detected default branch

The last two checks are configurable with:

- `--safe-remove-requires-no-extra-branches=false`
- `--safe-remove-requires-no-linked-worktrees=false`

All destructive actions require an explicit confirmation modal. Path deletions are checked to remain within the configured root before any filesystem operation happens.

## Deletion Behavior

- Selecting a repo in the left pane and pressing `d` removes the repo directory from disk after confirmation
- Selecting a linked worktree in the middle pane and pressing `d` runs `git worktree remove`
- Pressing `D` on a linked worktree confirms a force remove path
- Selecting a branch and pressing `d` runs `git branch -d`
- Pressing `D` on a branch runs `git branch -D` after a separate confirmation
- Selecting a stash and pressing `d` runs `git stash drop`

## Optional Summarizer

`minos` can call any external command that reads a prompt from stdin and writes a summary to stdout. By default it uses `codex exec --skip-git-repo-check --color never --ephemeral -`.

Examples:

```bash
minos --summarizer-cmd='codex'
minos --summarizer-cmd='claude -p'
```

When `a` is pressed on a selected worktree, the app sends a compact Git context to the command and renders the result in the status pane. No AI command is required for the rest of the application.

## Taskfile

- `task build`
- `task run`
- `task test`
- `task lint`
- `task vuln`

## Demo

If you want a quick recording:

```bash
asciinema rec minos.cast --command './minos --root ~/src'
```
