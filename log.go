package main

import (
	"fmt"
	"strings"
	"time"
)

type Entry struct {
	Hash      string
	Timestamp time.Time
	Location  string
	Body      string
}

func loadEntries(repoPath string) ([]Entry, error) {
	// Each record: <hash>\x00<subject>\x00<body>\x1f
	out, err := gitOutput(repoPath, "log", "--format=format:%H%x00%s%x00%b%x1f")
	if err != nil {
		if isNoCommitsError(err) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("reading log: %w", err)
	}

	var entries []Entry
	for _, record := range strings.Split(string(out), "\x1f") {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		parts := strings.SplitN(record, "\x00", 3)
		if len(parts) < 2 {
			continue
		}
		subject := strings.TrimSpace(parts[1])
		t, err := time.Parse(time.RFC3339, subject)
		if err != nil {
			continue // not a journal commit
		}
		body := ""
		if len(parts) > 2 {
			body = strings.TrimSpace(parts[2])
		}
		entries = append(entries, Entry{
			Hash:      strings.TrimSpace(parts[0]),
			Timestamp: t,
			Location:  readLocation(repoPath, strings.TrimSpace(parts[0]), subject),
			Body:      body,
		})
	}
	return entries, nil
}

func isNoCommitsError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not have any commits yet")
}

func readLocation(repoPath, hash, timestamp string) string {
	if hash == "" || timestamp == "" {
		return ""
	}
	data, err := gitOutput(repoPath, "show", hash+":"+timestamp+".txt")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, ">>> (") && strings.HasSuffix(line, ")") {
			return line[5 : len(line)-1]
		}
	}
	return ""
}
