package api

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// LocalGitCommit represents a commit from a local git repository
type LocalGitCommit struct {
	SHA     string
	Message string
	Author  string
	Date    time.Time
}

// LocalGitClient provides methods to interact with local git repositories
type LocalGitClient struct{}

// NewLocalGitClient creates a new local git client
func NewLocalGitClient() *LocalGitClient {
	return &LocalGitClient{}
}

// GetCommitsInTimeRange fetches commits from a local git repository within a time range
// repoPath: path to the git repository (can be the repo root or any subdirectory)
// since: start time for commits
// until: end time for commits
// author: optional author filter (name or email)
func (g *LocalGitClient) GetCommitsInTimeRange(repoPath string, since, until time.Time, author string) ([]LocalGitCommit, error) {
	// Build git log command
	// Format: SHA|Author|Date|Subject
	args := []string{
		"-C", repoPath,
		"log",
		"--format=%H|%an|%aI|%s",
		"--since=" + since.Format(time.RFC3339),
		"--until=" + until.Format(time.RFC3339),
	}

	if author != "" {
		args = append(args, "--author="+author)
	}

	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check if it's a git repository
		if strings.Contains(stderr.String(), "not a git repository") {
			return nil, fmt.Errorf("not a git repository: %s", repoPath)
		}
		return nil, fmt.Errorf("git log failed: %s (stderr: %s)", err, stderr.String())
	}

	// Parse output
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil, nil // No commits in range
	}

	lines := strings.Split(output, "\n")
	commits := make([]LocalGitCommit, 0, len(lines))

	for _, line := range lines {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}

		date, err := time.Parse(time.RFC3339, parts[2])
		if err != nil {
			// Try parsing without timezone
			date, _ = time.Parse("2006-01-02T15:04:05", parts[2])
		}

		commits = append(commits, LocalGitCommit{
			SHA:     parts[0],
			Author:  parts[1],
			Date:    date,
			Message: parts[3],
		})
	}

	return commits, nil
}

// IsGitRepository checks if the given path is inside a git repository
func IsGitRepository(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// FormatLocalCommitsAsDescription formats a list of local commits as a description string
// suitable for use in a timesheet entry
func FormatLocalCommitsAsDescription(commits []LocalGitCommit) string {
	if len(commits) == 0 {
		return ""
	}

	var lines []string
	for _, commit := range commits {
		// Get the first line of the commit message (subject)
		message := commit.Message
		if idx := strings.Index(message, "\n"); idx != -1 {
			message = message[:idx]
		}
		// Truncate if too long
		if len(message) > 80 {
			message = message[:77] + "..."
		}

		// Format: short SHA + message
		shortSHA := commit.SHA
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}

		lines = append(lines, fmt.Sprintf("- %s: %s", shortSHA, message))
	}

	// Add header
	header := fmt.Sprintf("Git commits (%d):", len(commits))

	return header + "\n" + strings.Join(lines, "\n")
}

// IsLocalPath checks if a string looks like a local filesystem path
// rather than a GitHub URL
func IsLocalPath(path string) bool {
	// If it starts with / or ~, it's a local path
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "~") {
		return true
	}

	// If it starts with ./ or ../, it's a relative local path
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return true
	}

	// If it contains github.com, it's not a local path
	if strings.Contains(path, "github.com") {
		return false
	}

	// If it contains :// it's likely a URL
	if strings.Contains(path, "://") {
		return false
	}

	// If it contains git@ it's an SSH URL
	if strings.Contains(path, "git@") {
		return false
	}

	// Check if path exists on filesystem (could be relative path without ./)
	// We'll be conservative and only treat explicit paths as local
	return false
}

// ExpandPath expands ~ to home directory
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := exec.Command("sh", "-c", "echo $HOME").Output()
		if err == nil {
			return strings.TrimSpace(string(home)) + path[1:]
		}
	}
	return path
}
