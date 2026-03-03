package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
)

func TestNormalizeRepoPathExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	home = filepath.Clean(home)

	got, err := normalizeRepoPath("~/journal")
	if err != nil {
		t.Fatalf("normalizeRepoPath: %v", err)
	}
	want := filepath.Join(home, "journal")
	gotAbs, err := filepath.Abs(got)
	if err != nil {
		t.Fatalf("abs got: %v", err)
	}
	if gotAbs != want {
		t.Fatalf("got %q, want %q", gotAbs, want)
	}
}

func TestNormalizeRepoPathExpandsEnv(t *testing.T) {
	t.Setenv("JOURNAL_TEST_PATH", "/tmp/journal-test")
	got, err := normalizeRepoPath("$JOURNAL_TEST_PATH/entries")
	if err != nil {
		t.Fatalf("normalizeRepoPath: %v", err)
	}
	if got != "/tmp/journal-test/entries" {
		t.Fatalf("got %q", got)
	}
}

func TestLocationString(t *testing.T) {
	tests := []struct {
		name string
		loc  Location
		want string
	}{
		{name: "empty", loc: Location{}, want: ""},
		{name: "city only", loc: Location{City: "Austin"}, want: "Austin"},
		{name: "region only", loc: Location{Region: "TX"}, want: "TX"},
		{name: "both", loc: Location{City: "Austin", Region: "TX"}, want: "Austin, TX"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.loc.String(); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListViewportClampsToWindow(t *testing.T) {
	m := listModel{windowWidth: 3, windowHeight: 6}
	if got := m.vpWidth(); got != 1 {
		t.Fatalf("vpWidth got %d, want 1", got)
	}
	if got := m.vpHeight(); got != 1 {
		t.Fatalf("vpHeight got %d, want 1", got)
	}
}

func TestFormatComposeHeader(t *testing.T) {
	now := time.Date(2026, time.March, 2, 14, 30, 0, 0, time.FixedZone("CST", -6*60*60))

	header := formatComposeHeader(now, Location{}, false, spinner.New())
	if header != formatDate(now) {
		t.Fatalf("header got %q", header)
	}

	header = formatComposeHeader(now, Location{City: "Bentonville", Region: "AR"}, false, spinner.New())
	if !strings.Contains(header, "Bentonville, AR") {
		t.Fatalf("location missing from header: %q", header)
	}

	header = formatComposeHeader(now, Location{}, true, spinner.New())
	if !strings.Contains(header, "locating...") {
		t.Fatalf("locating indicator missing from header: %q", header)
	}
}
