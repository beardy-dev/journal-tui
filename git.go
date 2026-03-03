package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type SyncStatus struct {
	Branch      string
	Upstream    string
	HasUpstream bool
	Ahead       int
	Behind      int
}

func commitEntry(repoPath, entry string, loc Location) error {
	now := time.Now()
	timestamp := now.Format(time.RFC3339)
	filename := timestamp + ".txt"

	// File content matching iOS shortcut format
	content := fmt.Sprintf("new_entry_added:\n>>> %s\n", timestamp)
	if location := loc.String(); location != "" {
		content += fmt.Sprintf(">>> (%s)\n", location)
	}

	// Pull latest changes first
	if err := git(repoPath, "pull", "--rebase"); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}

	// Write the file
	fullPath := filepath.Join(repoPath, filename)
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	// Stage it
	if err := git(repoPath, "add", filename); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Commit: subject line is the timestamp, body is the entry text
	commitMsg := timestamp + "\n\n" + entry
	if err := git(repoPath, "commit", "-m", commitMsg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	// Push
	if err := git(repoPath, "push"); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

func git(repoPath string, args ...string) error {
	cmdArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, string(out))
	}
	return nil
}

// isGitRepo returns true if path is an existing git repository.
func isGitRepo(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// initRepo runs `git init <path>`, creating the directory if needed.
func initRepo(path string) error {
	cmd := exec.Command("git", "init", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init: %w\n%s", err, out)
	}
	return nil
}

// commitsAhead returns the number of local commits not yet pushed to the
// upstream branch. Returns an error if no upstream is configured.
func commitsAhead(repoPath string) (int, error) {
	status, err := getSyncStatus(repoPath)
	if err != nil {
		return 0, err
	}
	if !status.HasUpstream {
		return 0, fmt.Errorf("no upstream configured for current branch")
	}
	return status.Ahead, nil
}

func getSyncStatus(repoPath string) (SyncStatus, error) {
	branchOut, err := gitOutput(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return SyncStatus{}, fmt.Errorf("resolve branch: %w", err)
	}
	branch := strings.TrimSpace(string(branchOut))

	upstreamOut, err := gitOutput(repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return SyncStatus{
			Branch:      branch,
			HasUpstream: false,
		}, nil
	}
	upstream := strings.TrimSpace(string(upstreamOut))

	// Refresh remote refs so pull status reflects remote state.
	if err := git(repoPath, "fetch", "--quiet"); err != nil {
		return SyncStatus{}, fmt.Errorf("git fetch: %w", err)
	}

	aheadOut, err := gitOutput(repoPath, "rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		return SyncStatus{}, fmt.Errorf("count commits to push: %w", err)
	}
	behindOut, err := gitOutput(repoPath, "rev-list", "--count", "HEAD..@{u}")
	if err != nil {
		return SyncStatus{}, fmt.Errorf("count commits to pull: %w", err)
	}
	ahead, err := strconv.Atoi(strings.TrimSpace(string(aheadOut)))
	if err != nil {
		return SyncStatus{}, fmt.Errorf("parse ahead count: %w", err)
	}
	behind, err := strconv.Atoi(strings.TrimSpace(string(behindOut)))
	if err != nil {
		return SyncStatus{}, fmt.Errorf("parse behind count: %w", err)
	}

	return SyncStatus{
		Branch:      branch,
		Upstream:    upstream,
		HasUpstream: true,
		Ahead:       ahead,
		Behind:      behind,
	}, nil
}

func gitOutput(repoPath string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%w: %s", err, exitErr.Stderr)
		}
		return nil, err
	}
	return out, nil
}
