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

	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "help", "--help", "-h":
			runHelp()
			return
		case "config":
			if cfg == nil {
				fmt.Fprintln(os.Stderr, "journal: no config found — run `journal add` or `journal` to set up")
				os.Exit(1)
			}
			runConfig(cfg)
			return
		case "list":
			if cfg == nil {
				fmt.Fprintln(os.Stderr, "journal: no journals configured — run `journal add <name>`")
				os.Exit(1)
			}
			runJournals(cfg)
			return
		case "add":
			if cfg == nil {
				cfg = &Config{}
			}
			if err := runAddJournal(cfg, args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "journal: %v\n", err)
				os.Exit(1)
			}
			return
		case "use":
			if cfg == nil {
				fmt.Fprintln(os.Stderr, "journal: no journals configured — run `journal add <name>`")
				os.Exit(1)
			}
			if err := runUseJournal(cfg, args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "journal: %v\n", err)
				os.Exit(1)
			}
			return
		case "sync":
			if cfg == nil {
				fmt.Fprintln(os.Stderr, "journal: no journals configured — run `journal add <name>`")
				os.Exit(1)
			}
			if err := runSync(cfg, args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "journal: %v\n", err)
				os.Exit(1)
			}
			return
		case "log":
			if cfg == nil {
				fmt.Fprintln(os.Stderr, "journal: no journals configured — run `journal add <name>`")
				os.Exit(1)
			}
			journalName := ""
			if len(args) > 1 {
				journalName = args[1]
			}
			_, repoPath, err := cfg.journal(journalName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "journal: %v\n", err)
				os.Exit(1)
			}
			p := tea.NewProgram(newListModel(repoPath), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "journal: %v\n", err)
				os.Exit(1)
			}
			return
		default:
			fmt.Fprintf(os.Stderr, "journal: unknown command %q\n", args[0])
			fmt.Fprintln(os.Stderr, "Run `journal help` to see available commands.")
			os.Exit(1)
		}
	}

	if cfg == nil || len(cfg.Journals) == 0 {
		cfg, err = firstRunSetup()
		if err != nil {
			fmt.Fprintf(os.Stderr, "journal: %v\n", err)
			os.Exit(1)
		}
	}

	_, repoPath, err := cfg.activeJournal()
	if err != nil {
		fmt.Fprintf(os.Stderr, "journal: %v\n", err)
		os.Exit(1)
	}

	m := newModel(repoPath)
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

// firstRunSetup prompts for a journal name and repo path, validates or
// initialises the repo, then writes the config file.
func firstRunSetup() (*Config, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("journal name [default]: ")
	nameLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	journalName := strings.TrimSpace(nameLine)
	if journalName == "" {
		journalName = "default"
	}

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

	cfg := &Config{}
	if err := cfg.addJournal(journalName, repoPath); err != nil {
		return nil, err
	}
	cfg.ActiveJournal = journalName
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
	fmt.Println(`Usage: journal [command] [journal-name]

Commands:
  (none)         write a new entry in active journal
  list           list configured journals
  log [name]     browse entries (active journal if omitted)
  sync [name]    push local commits (active journal if omitted)
  sync status [name|all]
                 show push/pull status (active journal if omitted)
  add <name>     add a named journal
  use <name>     set active journal
  config         show current configuration
  help           show this help message`)
}

// runConfig prints the current configuration.
func runConfig(cfg *Config) {
	cfg.migrateLegacy()
	fmt.Printf("config file:      %s\n", configPath())
	fmt.Printf("active_journal:   %s\n", cfg.ActiveJournal)
	for _, name := range cfg.journalNames() {
		fmt.Printf("journal.%s:      %s\n", name, cfg.Journals[name])
	}
}

func runJournals(cfg *Config) {
	cfg.migrateLegacy()
	if len(cfg.Journals) == 0 {
		fmt.Println("No journals configured.")
		return
	}
	for _, name := range cfg.journalNames() {
		marker := " "
		if name == cfg.ActiveJournal {
			marker = "*"
		}
		fmt.Printf("%s %s -> %s\n", marker, name, cfg.Journals[name])
	}
}

func runAddJournal(cfg *Config, args []string) error {
	reader := bufio.NewReader(os.Stdin)
	name := ""
	if len(args) > 0 {
		name = strings.TrimSpace(args[0])
	}
	if name == "" {
		fmt.Print("journal name: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		name = strings.TrimSpace(line)
	}
	if name == "" {
		return fmt.Errorf("journal name cannot be empty")
	}

	repoPath := ""
	if len(args) > 1 {
		repoPath = strings.TrimSpace(args[1])
	}
	if repoPath == "" {
		fmt.Print("journal repo path: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		repoPath = strings.TrimSpace(line)
	}
	if repoPath == "" {
		return fmt.Errorf("repo path cannot be empty")
	}

	repoPath, err := normalizeRepoPath(repoPath)
	if err != nil {
		return err
	}
	if err := ensureRepo(repoPath, reader); err != nil {
		return err
	}
	if err := cfg.addJournal(name, repoPath); err != nil {
		return err
	}
	if err := writeConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("Added journal %q\n", name)
	return nil
}

func runUseJournal(cfg *Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: journal use <name>")
	}
	name := strings.TrimSpace(args[0])
	if err := cfg.setActiveJournal(name); err != nil {
		return err
	}
	if err := writeConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("Active journal set to %q\n", name)
	return nil
}

// runSync offers to push local commits to the remote.
func runSync(cfg *Config, args []string) error {
	if len(args) > 0 && args[0] == "status" {
		return runSyncStatus(cfg, args[1:])
	}

	journalName := ""
	if len(args) > 0 {
		journalName = args[0]
	}
	name, repoPath, err := cfg.journal(journalName)
	if err != nil {
		return err
	}

	// Try to count unpushed commits for a more informative prompt.
	n, err := commitsAhead(repoPath)
	switch {
	case err != nil:
		// No upstream configured or other git error — ask anyway.
		fmt.Printf("Push journal %q to remote? [y/N]: ", name)
	case n == 0:
		fmt.Println("Nothing to push.")
		return nil
	default:
		fmt.Printf("%d commit(s) to push for %q. Continue? [y/N]: ", n, name)
	}

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(answer)) != "y" {
		fmt.Println("Aborted.")
		return nil
	}

	fmt.Printf("Pushing %q… ", name)
	if err := git(repoPath, "push"); err != nil {
		return fmt.Errorf("push: %w", err)
	}
	fmt.Println("done.")
	return nil
}

func runSyncStatus(cfg *Config, args []string) error {
	target, all, err := resolveSyncStatusTarget(args)
	if err != nil {
		return err
	}

	if all {
		names := cfg.journalNames()
		for i, name := range names {
			repoPath := cfg.Journals[name]
			status, err := getSyncStatus(repoPath)
			if err != nil {
				fmt.Printf("%s: error: %v\n", name, err)
			} else {
				printSyncStatus(name, status)
			}
			if i < len(names)-1 {
				fmt.Println()
			}
		}
		return nil
	}

	name, repoPath, err := cfg.journal(target)
	if err != nil {
		return err
	}
	status, err := getSyncStatus(repoPath)
	if err != nil {
		return err
	}
	printSyncStatus(name, status)
	return nil
}

func resolveSyncStatusTarget(args []string) (target string, all bool, err error) {
	if len(args) > 1 {
		return "", false, fmt.Errorf("usage: journal sync status [name|all]")
	}
	if len(args) == 0 {
		return "", false, nil
	}
	target = strings.TrimSpace(args[0])
	return target, target == "all", nil
}

func printSyncStatus(name string, status SyncStatus) {
	fmt.Printf("journal: %s\n", name)
	if status.Branch != "" {
		fmt.Printf("branch: %s\n", status.Branch)
	}
	if !status.HasUpstream {
		fmt.Println("upstream: none configured")
		fmt.Println("status: cannot determine push/pull counts")
		return
	}
	fmt.Printf("upstream: %s\n", status.Upstream)
	fmt.Printf("to push: %d\n", status.Ahead)
	fmt.Printf("to pull: %d\n", status.Behind)
	switch {
	case status.Ahead == 0 && status.Behind == 0:
		fmt.Println("status: up to date")
	case status.Ahead > 0 && status.Behind > 0:
		fmt.Println("status: needs push and pull")
	case status.Ahead > 0:
		fmt.Println("status: needs push")
	default:
		fmt.Println("status: needs pull")
	}
}
