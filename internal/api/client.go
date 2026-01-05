package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/kartoza/go-timesheets-go/internal/models"
)

// Client represents an API client for the Django backend
type Client struct {
	baseURL    string
	httpClient *http.Client
	csrfToken  string
	username   string
	password   string
	authToken  string // API token for authentication
}

// Config holds configuration for the API client
type Config struct {
	BaseURL   string `json:"base_url"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	AuthToken string `json:"auth_token,omitempty"` // Optional auth token
	Timeout   int    `json:"timeout_seconds"`
}

// LoginResponse represents the response from the login endpoint
type LoginResponse struct {
	Token    string `json:"token"`
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// NewClient creates a new API client
func NewClient(config Config) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	timeout := time.Duration(config.Timeout) * time.Second
	if config.Timeout == 0 {
		timeout = 30 * time.Second
	}

	client := &Client{
		baseURL:   strings.TrimSuffix(config.BaseURL, "/"),
		username:  config.Username,
		password:  config.Password,
		authToken: config.AuthToken,
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: timeout,
		},
	}

	return client, nil
}

// Login authenticates with username/password and returns an auth token
func (c *Client) Login(username, password string) (*LoginResponse, error) {
	loginURL := c.baseURL + "/api/login/"

	credentials := map[string]string{
		"username": username,
		"password": password,
	}

	jsonData, err := json.Marshal(credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}

	req, err := http.NewRequest("POST", loginURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return nil, fmt.Errorf("failed to decode login response: %w", err)
	}

	// Store the auth token
	c.authToken = loginResp.Token
	c.username = username
	c.password = password

	return &loginResp, nil
}

// SetAuthToken sets the authentication token
func (c *Client) SetAuthToken(token string) {
	c.authToken = token
}

// GetAuthToken returns the current auth token
func (c *Client) GetAuthToken() string {
	return c.authToken
}

// authenticate performs login and retrieves CSRF token
func (c *Client) authenticate() error {
	// Get CSRF token from login page
	loginURL := c.baseURL + "/accounts/login/"
	resp, err := c.httpClient.Get(loginURL)
	if err != nil {
		return fmt.Errorf("failed to get login page: %w", err)
	}
	defer resp.Body.Close()

	// Extract CSRF token from response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login page: %w", err)
	}

	csrfToken, err := extractCSRFToken(string(body))
	if err != nil {
		return fmt.Errorf("failed to extract CSRF token: %w", err)
	}

	c.csrfToken = csrfToken

	// Perform login
	loginData := url.Values{
		"username":      {c.username},
		"password":      {c.password},
		"csrfmiddlewaretoken": {c.csrfToken},
	}

	req, err := http.NewRequest("POST", loginURL, strings.NewReader(loginData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", loginURL)

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return fmt.Errorf("login failed with status: %d", resp.StatusCode)
	}

	return nil
}

// extractCSRFToken extracts CSRF token from HTML
func extractCSRFToken(html string) (string, error) {
	if strings.Contains(html, "csrfmiddlewaretoken") {
		// Simple extraction - find the token value
		start := strings.Index(html, `name="csrfmiddlewaretoken"`)
		if start == -1 {
			start = strings.Index(html, `name='csrfmiddlewaretoken'`)
		}
		if start != -1 {
			valueStart := strings.Index(html[start:], `value="`)
			if valueStart == -1 {
				valueStart = strings.Index(html[start:], `value='`)
				if valueStart != -1 {
					valueStart += start + 7
					valueEnd := strings.Index(html[valueStart:], `'`)
					if valueEnd != -1 {
						return html[valueStart : valueStart+valueEnd], nil
					}
				}
			} else {
				valueStart += start + 7
				valueEnd := strings.Index(html[valueStart:], `"`)
				if valueEnd != -1 {
					return html[valueStart : valueStart+valueEnd], nil
				}
			}
		}
	}

	return "", fmt.Errorf("CSRF token not found in HTML")
}

// makeRequest makes an authenticated HTTP request
func (c *Client) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	url := c.baseURL + endpoint

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Use token-based auth if available, otherwise fall back to session auth
	if c.authToken != "" {
		req.Header.Set("Authorization", "Token "+c.authToken)
	} else {
		// Ensure we're authenticated via session
		if c.csrfToken == "" {
			if err := c.authenticate(); err != nil {
				return nil, fmt.Errorf("authentication failed: %w", err)
			}
		}
		req.Header.Set("X-CSRFToken", c.csrfToken)
		req.Header.Set("Referer", c.baseURL)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	// Handle authentication errors
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()

		// If using token auth and it failed, return error (token might be invalid)
		if c.authToken != "" {
			return nil, fmt.Errorf("authentication failed: token may be invalid or expired")
		}

		// Try to re-authenticate and retry once for session auth
		c.csrfToken = ""
		if err := c.authenticate(); err != nil {
			return nil, fmt.Errorf("re-authentication failed: %w", err)
		}

		// Retry the request
		req.Header.Set("X-CSRFToken", c.csrfToken)
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to retry request: %w", err)
		}
	}

	return resp, nil
}

// ProjectListItem represents a project from the autocomplete API
type ProjectListItem struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// ActivityListItem represents an activity from the API
type ActivityListItem struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// TaskListItem represents a task from the API
type TaskListItem struct {
	ID           int     `json:"id"`
	Label        string  `json:"label"`
	Value        string  `json:"value"`
	ActualTime   float64 `json:"actual_time"`
	ExpectedTime float64 `json:"expected_time"`
}

// TimelogEntry represents a timesheet entry from the API
type TimelogEntry struct {
	ID          int       `json:"id"`
	ProjectName string    `json:"project_name"`
	TaskName    string    `json:"task_name"`
	Activity    string    `json:"activity"`
	Description string    `json:"description"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Duration    float64   `json:"duration"`
	IsSubmitted bool      `json:"is_submitted"`
}

// GetProjects fetches the list of projects with optional search query
func (c *Client) GetProjects(query string) ([]ProjectListItem, error) {
	endpoint := "/api/project-list/"
	if query != "" {
		endpoint = endpoint + "?q=" + url.QueryEscape(query)
	}

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var projects []ProjectListItem
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, fmt.Errorf("failed to decode projects response: %w", err)
	}

	return projects, nil
}

// GetActivities fetches the list of activities
func (c *Client) GetActivities() ([]ActivityListItem, error) {
	resp, err := c.makeRequest("GET", "/api/activity-list/", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var activities []ActivityListItem
	if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
		return nil, fmt.Errorf("failed to decode activities response: %w", err)
	}

	return activities, nil
}

// GetTasks fetches the list of tasks for a specific project
func (c *Client) GetTasks(projectID string) ([]TaskListItem, error) {
	endpoint := fmt.Sprintf("/api/task-list/%s/", projectID)
	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var tasks []TaskListItem
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, fmt.Errorf("failed to decode tasks response: %w", err)
	}

	return tasks, nil
}

// GetTimelogs fetches timesheet entries
func (c *Client) GetTimelogs() ([]TimelogEntry, error) {
	resp, err := c.makeRequest("GET", "/api/timelog/", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var timelogs []TimelogEntry
	if err := json.NewDecoder(resp.Body).Decode(&timelogs); err != nil {
		return nil, fmt.Errorf("failed to decode timelogs response: %w", err)
	}

	return timelogs, nil
}

// CreateTimesheet creates a new timesheet entry
func (c *Client) CreateTimesheet(entry models.TimeEntry) error {
	// Convert our internal model to API format
	apiEntry := map[string]interface{}{
		"project":     entry.ProjectID,
		"activity":    entry.ActivityID,
		"description": entry.Description,
		"start_time":  entry.StartTime.Format(time.RFC3339),
		"duration":    entry.Duration,
	}

	if entry.TaskID != nil {
		apiEntry["task"] = *entry.TaskID
	}

	if entry.EndTime != nil {
		apiEntry["end_time"] = entry.EndTime.Format(time.RFC3339)
	}

	resp, err := c.makeRequest("POST", "/api/timesheet/", apiEntry)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create timesheet entry (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// UpdateTimesheet updates an existing timesheet entry
func (c *Client) UpdateTimesheet(entryID string, entry models.TimeEntry) error {
	// Convert our internal model to API format
	apiEntry := map[string]interface{}{
		"project":     entry.ProjectID,
		"activity":    entry.ActivityID,
		"description": entry.Description,
		"start_time":  entry.StartTime.Format(time.RFC3339),
		"duration":    entry.Duration,
	}

	if entry.TaskID != nil {
		apiEntry["task"] = *entry.TaskID
	}

	if entry.EndTime != nil {
		apiEntry["end_time"] = entry.EndTime.Format(time.RFC3339)
	}

	endpoint := fmt.Sprintf("/api/timesheet/%s/", entryID)
	resp, err := c.makeRequest("PUT", endpoint, apiEntry)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update timesheet entry (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteTimesheet deletes a timesheet entry
func (c *Client) DeleteTimesheet(entryID string) error {
	endpoint := fmt.Sprintf("/api/timesheet/%s/", entryID)
	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete timesheet entry (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// CreateProject creates a new project
func (c *Client) CreateProject(project models.Project) (*models.Project, error) {
	apiProject := map[string]interface{}{
		"name":        project.Name,
		"description": project.Description,
		"is_active":   project.IsActive,
	}

	resp, err := c.makeRequest("POST", "/api/project/", apiProject)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create project (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var createdProject models.Project
	if err := json.NewDecoder(resp.Body).Decode(&createdProject); err != nil {
		return nil, fmt.Errorf("failed to decode created project: %w", err)
	}

	return &createdProject, nil
}

// UpdateProject updates an existing project
func (c *Client) UpdateProject(projectID string, project models.Project) error {
	apiProject := map[string]interface{}{
		"name":        project.Name,
		"description": project.Description,
		"is_active":   project.IsActive,
	}

	endpoint := fmt.Sprintf("/api/project/%s/", projectID)
	resp, err := c.makeRequest("PUT", endpoint, apiProject)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update project (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteProject deletes a project
func (c *Client) DeleteProject(projectID string) error {
	endpoint := fmt.Sprintf("/api/project/%s/", projectID)
	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete project (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// CreateTask creates a new task
func (c *Client) CreateTask(task models.Task) (*models.Task, error) {
	apiTask := map[string]interface{}{
		"project_id":    task.ProjectID,
		"name":          task.Name,
		"description":   task.Description,
		"expected_time": task.ExpectedTime,
		"is_active":     task.IsActive,
	}

	resp, err := c.makeRequest("POST", "/api/task/", apiTask)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create task (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var createdTask models.Task
	if err := json.NewDecoder(resp.Body).Decode(&createdTask); err != nil {
		return nil, fmt.Errorf("failed to decode created task: %w", err)
	}

	return &createdTask, nil
}

// UpdateTask updates an existing task
func (c *Client) UpdateTask(taskID string, task models.Task) error {
	apiTask := map[string]interface{}{
		"project_id":    task.ProjectID,
		"name":          task.Name,
		"description":   task.Description,
		"expected_time": task.ExpectedTime,
		"actual_time":   task.ActualTime,
		"is_active":     task.IsActive,
	}

	endpoint := fmt.Sprintf("/api/task/%s/", taskID)
	resp, err := c.makeRequest("PUT", endpoint, apiTask)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update task (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteTask deletes a task
func (c *Client) DeleteTask(taskID string) error {
	endpoint := fmt.Sprintf("/api/task/%s/", taskID)
	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete task (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// CreateActivity creates a new activity
func (c *Client) CreateActivity(activity models.Activity) (*models.Activity, error) {
	apiActivity := map[string]interface{}{
		"name": activity.Name,
	}

	resp, err := c.makeRequest("POST", "/api/activity/", apiActivity)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create activity (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var createdActivity models.Activity
	if err := json.NewDecoder(resp.Body).Decode(&createdActivity); err != nil {
		return nil, fmt.Errorf("failed to decode created activity: %w", err)
	}

	return &createdActivity, nil
}

// UpdateActivity updates an existing activity
func (c *Client) UpdateActivity(activityID string, activity models.Activity) error {
	apiActivity := map[string]interface{}{
		"name": activity.Name,
	}

	endpoint := fmt.Sprintf("/api/activity/%s/", activityID)
	resp, err := c.makeRequest("PUT", endpoint, apiActivity)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update activity (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteActivity deletes an activity
func (c *Client) DeleteActivity(activityID string) error {
	endpoint := fmt.Sprintf("/api/activity/%s/", activityID)
	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete activity (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// SubmitTimesheet submits a batch of timesheet entries
func (c *Client) SubmitTimesheet(submission models.TimesheetSubmission) error {
	apiSubmission := map[string]interface{}{
		"entries":      submission.Entries,
		"period_start": submission.PeriodStart.Format(time.RFC3339),
		"period_end":   submission.PeriodEnd.Format(time.RFC3339),
		"total_hours":  submission.TotalHours,
	}

	resp, err := c.makeRequest("POST", "/api/timesheet/submit/", apiSubmission)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to submit timesheet (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// HealthCheck tests if the API is reachable and authentication works
func (c *Client) HealthCheck() error {
	// Try to fetch activities as a simple health check
	_, err := c.GetActivities()
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	return nil
}