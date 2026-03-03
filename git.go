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
	out, err := gitOutput(repoPath, "rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, err
	}
	return n, nil
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
