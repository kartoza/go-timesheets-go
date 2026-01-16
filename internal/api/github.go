package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// githubDebugLog is used to log GitHub API calls
var githubDebugLog *log.Logger

func init() {
	f, err := os.OpenFile("/tmp/kartoza-timesheet-debug.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		githubDebugLog = log.New(io.Discard, "", 0)
	} else {
		githubDebugLog = log.New(f, "GITHUB: ", log.LstdFlags|log.Lshortfile)
	}
}

// GitHubCommit represents a commit from the GitHub API
type GitHubCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Author struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
		Message string `json:"message"`
	} `json:"commit"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
}

// GitHubClient provides methods to interact with the GitHub API
type GitHubClient struct {
	httpClient *http.Client
	token      string // Optional personal access token for private repos
}

// NewGitHubClient creates a new GitHub API client
func NewGitHubClient(token string) *GitHubClient {
	return &GitHubClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token: token,
	}
}

// GetCommitsInTimeRange fetches commits from a GitHub repository within a time range
// owner: repository owner (e.g., "kartoza")
// repo: repository name (e.g., "go-timesheets-go")
// since: start time for commits
// until: end time for commits
// author: optional author filter (GitHub username or email)
func (g *GitHubClient) GetCommitsInTimeRange(owner, repo string, since, until time.Time, author string) ([]GitHubCommit, error) {
	// Build the API URL
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits", owner, repo)

	params := url.Values{}
	params.Set("since", since.Format(time.RFC3339))
	params.Set("until", until.Format(time.RFC3339))
	if author != "" {
		params.Set("author", author)
	}
	params.Set("per_page", "100") // Get up to 100 commits

	fullURL := apiURL + "?" + params.Encode()

	// Log the API call being made
	githubDebugLog.Printf("Fetching commits from GitHub API")
	githubDebugLog.Printf("  Repository: %s/%s", owner, repo)
	githubDebugLog.Printf("  Time range: %s to %s", since.Format("2006-01-02 15:04:05"), until.Format("2006-01-02 15:04:05"))
	githubDebugLog.Printf("  URL: %s", fullURL)
	if g.token != "" {
		githubDebugLog.Printf("  Auth: using token (redacted)")
	} else {
		githubDebugLog.Printf("  Auth: no token (public repos only)")
	}

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		githubDebugLog.Printf("  Error: failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	// Add authentication if token is available (needed for private repos)
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		githubDebugLog.Printf("  Error: failed to fetch commits: %v", err)
		return nil, fmt.Errorf("failed to fetch commits: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		githubDebugLog.Printf("  Error: repository not found (status 404)")
		return nil, fmt.Errorf("repository not found: %s/%s (may be private - configure GitHub token)", owner, repo)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		githubDebugLog.Printf("  Error: GitHub API error (status %d): %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var commits []GitHubCommit
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		githubDebugLog.Printf("  Error: failed to decode commits: %v", err)
		return nil, fmt.Errorf("failed to decode commits: %w", err)
	}

	githubDebugLog.Printf("  Result: found %d commits", len(commits))
	for i, commit := range commits {
		shortSHA := commit.SHA
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}
		message := commit.Commit.Message
		if idx := strings.Index(message, "\n"); idx != -1 {
			message = message[:idx]
		}
		if len(message) > 60 {
			message = message[:57] + "..."
		}
		githubDebugLog.Printf("    [%d] %s: %s (%s)", i+1, shortSHA, message, commit.Commit.Author.Name)
	}

	return commits, nil
}

// FormatCommitsAsDescription formats a list of commits as a description string
// suitable for use in a timesheet entry
func FormatCommitsAsDescription(commits []GitHubCommit) string {
	if len(commits) == 0 {
		return ""
	}

	var lines []string
	for _, commit := range commits {
		// Get the first line of the commit message (subject)
		message := commit.Commit.Message
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

// ParseRepoFromURL extracts owner and repo name from a GitHub URL
// Supports formats:
// - github.com/owner/repo
// - https://github.com/owner/repo
// - git@github.com:owner/repo.git
func ParseRepoFromURL(repoURL string) (owner, repo string, err error) {
	// Remove protocol prefix if present
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "http://")
	repoURL = strings.TrimPrefix(repoURL, "git@")

	// Remove .git suffix if present
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Handle github.com/owner/repo format
	if strings.HasPrefix(repoURL, "github.com/") {
		parts := strings.Split(strings.TrimPrefix(repoURL, "github.com/"), "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	// Handle github.com:owner/repo format (SSH)
	if strings.HasPrefix(repoURL, "github.com:") {
		parts := strings.Split(strings.TrimPrefix(repoURL, "github.com:"), "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	return "", "", fmt.Errorf("could not parse GitHub repository from URL: %s", repoURL)
}
