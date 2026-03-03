package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	ActiveJournal string            `toml:"active_journal,omitempty"`
	Journals      map[string]string `toml:"journals,omitempty"`
	RepoPath      string            `toml:"repo_path,omitempty"` // legacy single-journal key
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		return filepath.Join(".config", "journal", "config.toml")
	}
	return filepath.Join(home, ".config", "journal", "config.toml")
}

func loadConfig() (*Config, error) {
	path := configPath()
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, nil // signal: first run
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	cfg.migrateLegacy()
	return &cfg, nil
}

func writeConfig(cfg *Config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func (c *Config) migrateLegacy() {
	if c.Journals == nil {
		c.Journals = map[string]string{}
	}
	if c.RepoPath != "" {
		if _, exists := c.Journals["default"]; !exists {
			c.Journals["default"] = c.RepoPath
		}
		if c.ActiveJournal == "" {
			c.ActiveJournal = "default"
		}
		c.RepoPath = ""
	}
	if c.ActiveJournal == "" && len(c.Journals) == 1 {
		for name := range c.Journals {
			c.ActiveJournal = name
		}
	}
}

func (c *Config) activeJournal() (string, string, error) {
	c.migrateLegacy()
	if len(c.Journals) == 0 {
		return "", "", fmt.Errorf("no journals configured")
	}
	if c.ActiveJournal == "" {
		return "", "", fmt.Errorf("no active journal set; run `journal use <name>`")
	}
	repoPath, ok := c.Journals[c.ActiveJournal]
	if !ok {
		return "", "", fmt.Errorf("active journal %q does not exist; run `journal journals`", c.ActiveJournal)
	}
	return c.ActiveJournal, repoPath, nil
}

func (c *Config) journal(name string) (string, string, error) {
	c.migrateLegacy()
	name = strings.TrimSpace(name)
	if name == "" {
		return c.activeJournal()
	}
	repoPath, ok := c.Journals[name]
	if !ok {
		return "", "", fmt.Errorf("journal %q not found", name)
	}
	return name, repoPath, nil
}

func (c *Config) addJournal(name, repoPath string) error {
	c.migrateLegacy()
	name = strings.TrimSpace(name)
	repoPath = strings.TrimSpace(repoPath)
	if name == "" {
		return fmt.Errorf("journal name cannot be empty")
	}
	if repoPath == "" {
		return fmt.Errorf("repo path cannot be empty")
	}
	if _, exists := c.Journals[name]; exists {
		return fmt.Errorf("journal %q already exists", name)
	}
	c.Journals[name] = repoPath
	if c.ActiveJournal == "" {
		c.ActiveJournal = name
	}
	return nil
}

func (c *Config) setActiveJournal(name string) error {
	c.migrateLegacy()
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("journal name cannot be empty")
	}
	if _, ok := c.Journals[name]; !ok {
		return fmt.Errorf("journal %q not found", name)
	}
	c.ActiveJournal = name
	return nil
}

func (c *Config) journalNames() []string {
	c.migrateLegacy()
	names := make([]string, 0, len(c.Journals))
	for name := range c.Journals {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
