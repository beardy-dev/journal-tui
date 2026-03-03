# journal-tui

A terminal journal app for Linux. Write entries from the command line — each one becomes a git commit in a private repository, keeping your journal versioned, backed up, and accessible anywhere.

Built as a replacement for an iOS Shortcuts workflow, using the same file format so entries from both sources live together naturally in the same repo.

## Features

- Minimal TUI for writing entries (Bubbletea + Lipgloss)
- Each entry is a git commit — your history *is* your journal
- Automatically tags entries with timestamp and location (via IP geolocation)
- Browse and scroll through previous entries in the terminal
- Syncs with a remote git repository (GitHub, Gitea, etc.)

## Requirements

- Go 1.22+
- `git` installed and available on your PATH
- A git repository for your journal (local, or with a remote for sync)

## Installation

```bash
git clone https://github.com/yourusername/journal-tui
cd journal-tui
go build -o journal .
```

Optionally, make it available system-wide:

```bash
ln -s "$PWD/journal" ~/.local/bin/journal
```

Make sure `~/.local/bin` is on your PATH.

## Setup

Run `journal` for the first time and it will prompt for a journal name and repository path:

```
journal name [default]: personal
journal repo path: /home/you/journal
```

If the directory doesn't exist or isn't a git repository yet, you'll be asked if you want to initialise one:

```
/home/you/journal does not exist.
Initialize it as a new journal repo? [y/N]: y
Initialized empty Git repository in /home/you/journal/.git/
```

Configuration is saved to `~/.config/journal/config.toml`.

## Usage

```
journal                write a new entry in active journal
journal list           list configured journals
journal log            browse active journal entries
journal log work       browse a specific named journal
journal sync           push active journal commits
journal sync work      push a specific named journal
journal sync status    show active journal push/pull status
journal sync status work
                      show a specific named journal push/pull status
journal sync status all
                      show push/pull status for all journals
journal add work       add a named journal
journal use work       set active journal
journal config         show current configuration
journal help           show available commands
```

### Writing an entry

Running `journal` with no arguments opens the writing interface immediately. The header shows the current date/time right away, then updates with your approximate location once lookup returns.

```
  journal
  Mon, March 2 · 2:30 PM · Bentonville, AR

  > _




  ctrl+s commit  ·  esc quit
```

Press `ctrl+s` to commit the entry. The app will pull, commit, and push automatically. Press `esc` to quit without saving.

While composing, you can also manage journals in-app:
- `ctrl+o` open journal picker (switch active journal with `enter`)
- `ctrl+l` open logs for the active or selected journal
- `ctrl+r` refresh sync status

### Browsing entries

```bash
journal log
# or
journal log work
```

Navigate with arrow keys or `j`/`k`. Press `enter` to read an entry in full, `esc` to go back, `q` to quit.

### Syncing

```bash
journal sync
# or
journal sync work
# or
journal sync status
# or
journal sync status work
# or
journal sync status all
```

`journal sync` shows how many local commits haven't been pushed yet and asks for confirmation before pushing.

`journal sync status` shows sync state without changing anything:
- current branch and upstream
- commits to push
- commits to pull

## Entry format

Each entry creates one git commit and one `.txt` file in the repository root.

**Commit message:**
```
2026-03-02T14:30:00-06:00

Your journal entry text here.
```

**File** (`2026-03-02T14:30:00-06:00.txt`):
```
new_entry_added:
>>> 2026-03-02T14:30:00-06:00
>>> (Bentonville, AR)
```

This format matches the iOS Shortcuts + Working Copy workflow, so entries written on either platform coexist in the same repository.

## Configuration

`~/.config/journal/config.toml`:

```toml
active_journal = "personal"

[journals]
personal = "/home/you/journal"
work = "/home/you/work-journal"
```

The config directory and file are created with restricted permissions (`0700`/`0600`) so the repo path is not readable by other users on the system.

## Privacy

On each new entry, the app makes a single HTTPS geolocation lookup (currently using `ipapi.co`, with `ipinfo.io` as fallback) to determine approximate city and region from your IP address. No API key is required. If the request fails, the entry is still saved with a blank location. No other network requests are made during normal use.

## Dependencies

| Package | Purpose |
|---|---|
| [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) | TUI framework |
| [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles) | Textarea, spinner, viewport components |
| [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) | Terminal styling |
| [BurntSushi/toml](https://github.com/BurntSushi/toml) | Config file parsing |
| [muesli/reflow](https://github.com/muesli/reflow) | Word wrapping in entry viewer |

## License

MIT

---

Vibe coded with [Claude Code](https://claude.ai/code) and [Codex](https://openai.com/codex/)
