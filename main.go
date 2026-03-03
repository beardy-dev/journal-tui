package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "journal: %v\n", err)
		os.Exit(1)
	}

	// Subcommands that require an existing config but skip setup.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "help", "--help", "-h":
			runHelp()
			return
		case "config":
			if cfg == nil {
				fmt.Fprintln(os.Stderr, "journal: no config found — run `journal` to set up")
				os.Exit(1)
			}
			runConfig(cfg)
			return
		case "sync":
			if cfg == nil {
				fmt.Fprintln(os.Stderr, "journal: no config found — run `journal` to set up")
				os.Exit(1)
			}
			runSync(cfg)
			return
		}
	}

	// First-run setup if no config exists yet.
	if cfg == nil {
		cfg, err = firstRunSetup()
		if err != nil {
			fmt.Fprintf(os.Stderr, "journal: %v\n", err)
			os.Exit(1)
		}
	}

	// Remaining subcommands that need config (and possibly setup).
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "list", "ls", "-l", "--list":
			p := tea.NewProgram(newListModel(cfg.RepoPath), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "journal: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// Default: open the write TUI.
	m := newModel(cfg.RepoPath)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "journal: %v\n", err)
		os.Exit(1)
	}
	if fm, ok := finalModel.(model); ok && fm.state == stateError {
		os.Exit(1)
	}
}

// firstRunSetup prompts for a repo path, validates or initialises it, then
// writes the config file.
func firstRunSetup() (*Config, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("journal repo path: ")
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	repoPath := strings.TrimSpace(line)
	if repoPath == "" {
		return nil, fmt.Errorf("repo path cannot be empty")
	}
	repoPath, err = normalizeRepoPath(repoPath)
	if err != nil {
		return nil, err
	}

	if err := ensureRepo(repoPath, reader); err != nil {
		return nil, err
	}

	cfg := &Config{RepoPath: repoPath}
	if err := writeConfig(cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("config saved to %s\n", configPath())
	return cfg, nil
}

// ensureRepo checks that path is a git repository. If the directory is missing
// or not a repo, it asks the user whether to initialise one.
func ensureRepo(path string, reader *bufio.Reader) error {
	_, statErr := os.Stat(path)
	exists := statErr == nil

	if exists && isGitRepo(path) {
		return nil // already a valid repo
	}

	if !exists {
		fmt.Printf("%s does not exist.\n", path)
	} else {
		fmt.Printf("%s is not a git repository.\n", path)
	}

	fmt.Print("Initialize it as a new journal repo? [y/N]: ")
	answer, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(answer)) != "y" {
		return fmt.Errorf("aborted")
	}

	return initRepo(path)
}

func normalizeRepoPath(path string) (string, error) {
	path = os.ExpandEnv(path)
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", fmt.Errorf("could not resolve home directory")
		}
		if path == "~" {
			path = home
		} else if strings.HasPrefix(path, "~/") {
			path = filepath.Join(home, path[2:])
		}
	}
	return filepath.Clean(path), nil
}

// runHelp prints available commands.
func runHelp() {
	fmt.Println(`Usage: journal [command]

Commands:
  (none)    write a new journal entry
  list      browse previous entries
  sync      push local commits to remote
  config    show current configuration
  help      show this help message`)
}

// runConfig prints the current configuration.
func runConfig(cfg *Config) {
	fmt.Printf("config file:  %s\n", configPath())
	fmt.Printf("repo_path:    %s\n", cfg.RepoPath)
}

// runSync offers to push local commits to the remote.
func runSync(cfg *Config) {
	// Try to count unpushed commits for a more informative prompt.
	n, err := commitsAhead(cfg.RepoPath)
	switch {
	case err != nil:
		// No upstream configured or other git error — ask anyway.
		fmt.Print("Push to remote? [y/N]: ")
	case n == 0:
		fmt.Println("Nothing to push.")
		return
	default:
		fmt.Printf("%d commit(s) to push. Continue? [y/N]: ", n)
	}

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(answer)) != "y" {
		fmt.Println("Aborted.")
		return
	}

	fmt.Print("Pushing… ")
	if err := git(cfg.RepoPath, "push"); err != nil {
		fmt.Fprintf(os.Stderr, "\nerror: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("done.")
}
