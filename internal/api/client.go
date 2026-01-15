package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/monitoring"
)

var debugLog *log.Logger

func init() {
	// Create debug log file
	f, err := os.OpenFile("/tmp/kartoza-timesheet-debug.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		debugLog = log.New(io.Discard, "", 0)
	} else {
		debugLog = log.New(f, "API: ", log.LstdFlags|log.Lshortfile)
	}
}

// Client represents an API client for the Django backend
type Client struct {
	baseURL    string
	httpClient *http.Client
	csrfToken  string
	username   string
	password   string
	authToken  string // API token for authentication
	metrics    *monitoring.Metrics
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

// GetUsername returns the currently authenticated username
func (c *Client) GetUsername() string {
	return c.username
}

// SetUsername sets the username (used when loading from saved token)
func (c *Client) SetUsername(username string) {
	c.username = username
}

// SetMetrics sets the metrics tracker for the client
func (c *Client) SetMetrics(metrics *monitoring.Metrics) {
	c.metrics = metrics
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
	// Track request start time and metrics
	startTime := time.Now()
	if c.metrics != nil {
		c.metrics.StartAPIRequest()
		defer c.metrics.EndAPIRequest()
	}

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
		debugLog.Printf("makeRequest: Using token auth (token length: %d)", len(c.authToken))
		req.Header.Set("Authorization", "Token "+c.authToken)
	} else {
		debugLog.Printf("makeRequest: No auth token, using session auth")
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
	duration := time.Since(startTime)

	// Record metrics
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	if c.metrics != nil {
		c.metrics.RecordAPIRequest(method, endpoint, statusCode, duration, err)
	}

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
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Label string `json:"label"`
}

// FlexibleInt is a type that can unmarshal from both int and string
type FlexibleInt struct {
	Value int
}

// UnmarshalJSON implements json.Unmarshaler
func (fi *FlexibleInt) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as int first
	var i int
	if err := json.Unmarshal(data, &i); err == nil {
		fi.Value = i
		return nil
	}

	// Try to unmarshal as string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		// Empty string or non-numeric = 0
		if s == "" {
			fi.Value = 0
			return nil
		}
		// Try to parse string as int
		if _, err := fmt.Sscanf(s, "%d", &fi.Value); err == nil {
			return nil
		}
	}

	fi.Value = 0
	return nil
}

// TimelogEntry represents a timesheet entry from the API
// This struct matches the complete API response from /api/timelog/
type TimelogEntry struct {
	ID            int          `json:"id"`
	Description   *string      `json:"description"` // nullable in API
	ActivityType  string       `json:"activity_type"`
	Owner         string       `json:"owner"`
	ProjectName   string       `json:"project_name"`
	Project       string       `json:"project"`
	ProjectID     int          `json:"project_id"`
	Task          string       `json:"task"`
	TaskName      string       `json:"task_name"`
	TaskID        FlexibleInt  `json:"task_id"`
	ActivityID    int          `json:"activity_id"`
	FromTime      string       `json:"from_time"`
	ToTime        string       `json:"to_time"`
	Hours         float64      `json:"hours"`
	Submitted     bool         `json:"submitted"`
	Timezone      string       `json:"timezone"`
	Parent        *int         `json:"parent"` // nullable in API
	TotalChildren FlexibleInt  `json:"total_children"`
	AllFromTime   string       `json:"all_from_time"`
	AllToTime     string       `json:"all_to_time"`
	AllHours      float64      `json:"all_hours"`
}

// GetFromTimeAsTime parses the from_time string to time.Time
func (t *TimelogEntry) GetFromTimeAsTime() (time.Time, error) {
	return parseTimeString(t.FromTime)
}

// GetToTimeAsTime parses the to_time string to time.Time
func (t *TimelogEntry) GetToTimeAsTime() (time.Time, error) {
	return parseTimeString(t.ToTime)
}

// GetDescriptionString safely returns the description as a string
func (t *TimelogEntry) GetDescriptionString() string {
	if t.Description == nil {
		return ""
	}
	return *t.Description
}

// GetHoursAsFloat returns the hours as float64
func (t *TimelogEntry) GetHoursAsFloat() float64 {
	return t.Hours
}

// Compatibility properties for backward compatibility with existing TUI code

// StartTime returns the parsed from_time as time.Time (for backward compatibility)
func (t *TimelogEntry) StartTime() time.Time {
	parsed, _ := t.GetFromTimeAsTime()
	return parsed
}

// EndTime returns the parsed to_time as time.Time (for backward compatibility)
func (t *TimelogEntry) EndTime() time.Time {
	if t.ToTime == "" {
		return time.Time{} // Zero time if not set
	}
	parsed, _ := t.GetToTimeAsTime()
	return parsed
}

// Duration returns hours as float64 (for backward compatibility)
func (t *TimelogEntry) Duration() float64 {
	return t.GetHoursAsFloat()
}

// Activity returns the activity type (for backward compatibility)
func (t *TimelogEntry) Activity() string {
	return t.ActivityType
}

// IsSubmitted returns the submitted status (for backward compatibility)
func (t *TimelogEntry) IsSubmitted() bool {
	return t.Submitted
}

// TimesheetCreateRequest represents the request body for creating a timesheet
type TimesheetCreateRequest struct {
	Description string                      `json:"description"` // HTML supported
	StartTime   string                      `json:"start_time"`  // datetime
	EndTime     *string                     `json:"end_time,omitempty"`
	Task        TimesheetObjectReference    `json:"task"`
	Project     TimesheetObjectReference    `json:"project"`
	Activity    TimesheetObjectReference    `json:"activity"`
	Timezone    string                      `json:"timezone"`
	Parent      int                         `json:"parent,omitempty"` // default: 0
	User        *TimesheetObjectReference   `json:"user,omitempty"`
}

// TimesheetObjectReference represents an object reference in the API (task, project, activity, user)
type TimesheetObjectReference struct {
	ID int `json:"id"`
}

// BreakTimesheetResponse represents the response from breaking a timesheet
type BreakTimesheetResponse struct {
	Detail string `json:"detail"`
}

// DeleteTimeLogRequest represents the request body for deleting a time log
type DeleteTimeLogRequest struct {
	ID string `json:"id"`
}

// parseTimeString attempts to parse time from various formats
func parseTimeString(timeStr string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	// Try different time formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999Z",
		"2006-01-02T15:04:05.999999",
		"2006-01-02",
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, timeStr); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
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

	debugLog.Printf("GetTimelogs: got %d entries", len(timelogs))
	for i, entry := range timelogs {
		if i < 3 { // Log first 3 entries
			debugLog.Printf("  Entry %d: ID=%d, ProjectName='%s', TaskName='%s', ToTime='%s'",
				i, entry.ID, entry.ProjectName, entry.TaskName, entry.ToTime)
		}
	}

	return timelogs, nil
}

// CreateTimesheet creates a new timesheet entry
func (c *Client) CreateTimesheet(entry models.TimeEntry) error {
	debugLog.Printf("CreateTimesheet called")
	debugLog.Printf("  ProjectID: %s", entry.ProjectID)
	debugLog.Printf("  ActivityID: %s", entry.ActivityID)
	debugLog.Printf("  Description: %s", entry.Description)
	debugLog.Printf("  StartTime: %s", entry.StartTime.Format(time.RFC3339))
	debugLog.Printf("  Duration: %f", entry.Duration)
	if entry.TaskID != nil {
		debugLog.Printf("  TaskID: %s", *entry.TaskID)
	}
	if entry.EndTime != nil {
		debugLog.Printf("  EndTime: %s", entry.EndTime.Format(time.RFC3339))
	}

	// Convert our internal model to API format
	// The Django API expects nested objects for project, task, activity
	apiEntry := map[string]interface{}{
		"project":     map[string]interface{}{"id": entry.ProjectID},
		"activity":    map[string]interface{}{"id": entry.ActivityID},
		"description": entry.Description,
		"start_time":  entry.StartTime.Format(time.RFC3339),
		"timezone":    "UTC",
	}

	if entry.TaskID != nil {
		apiEntry["task"] = map[string]interface{}{"id": *entry.TaskID}
	} else {
		apiEntry["task"] = map[string]interface{}{"id": "-"}
	}

	if entry.EndTime != nil {
		apiEntry["end_time"] = entry.EndTime.Format(time.RFC3339)
	}

	jsonBytes, _ := json.MarshalIndent(apiEntry, "", "  ")
	debugLog.Printf("API Request body:\n%s", string(jsonBytes))

	resp, err := c.makeRequest("POST", "/api/timesheet/", apiEntry)
	if err != nil {
		debugLog.Printf("makeRequest error: %v", err)
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	debugLog.Printf("Response status: %d", resp.StatusCode)
	debugLog.Printf("Response body: %s", string(bodyBytes))

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create timesheet entry (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Check if response is HTML (login redirect) instead of JSON
	contentType := resp.Header.Get("Content-Type")
	bodyStr := string(bodyBytes)
	if strings.Contains(contentType, "text/html") || strings.Contains(bodyStr, "<!DOCTYPE html>") || strings.Contains(bodyStr, "<html>") {
		debugLog.Printf("ERROR: Received HTML response instead of JSON - authentication may have failed")
		return fmt.Errorf("authentication failed: session may have expired. Please log out and log in again")
	}

	debugLog.Printf("CreateTimesheet successful")
	return nil
}

// UpdateTimesheet updates an existing timesheet entry
func (c *Client) UpdateTimesheet(entryID string, entry models.TimeEntry) error {
	// Convert our internal model to API format
	// The Django API expects nested objects for project, task, activity
	apiEntry := map[string]interface{}{
		"project":     map[string]interface{}{"id": entry.ProjectID},
		"activity":    map[string]interface{}{"id": entry.ActivityID},
		"description": entry.Description,
		"start_time":  entry.StartTime.Format(time.RFC3339),
		"timezone":    "UTC",
	}

	if entry.TaskID != nil {
		apiEntry["task"] = map[string]interface{}{"id": *entry.TaskID}
	} else {
		apiEntry["task"] = map[string]interface{}{"id": "-"}
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

// SubmitAllPending submits all unsubmitted time logs to the ERP system
// This endpoint automatically processes all pending entries for the authenticated user
func (c *Client) SubmitAllPending() error {
	resp, err := c.makeRequest("POST", "/api/submit-timesheet/", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to submit timesheets (status %d): %s", resp.StatusCode, string(bodyBytes))
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

// BreakTimesheet stops a running timesheet (timer)
func (c *Client) BreakTimesheet(timelogID int) (*BreakTimesheetResponse, error) {
	endpoint := fmt.Sprintf("/api/break-timesheet/%d/", timelogID)
	resp, err := c.makeRequest("POST", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to break timesheet (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var breakResp BreakTimesheetResponse
	if err := json.NewDecoder(resp.Body).Decode(&breakResp); err != nil {
		return nil, fmt.Errorf("failed to decode break timesheet response: %w", err)
	}

	return &breakResp, nil
}

// StopTimesheet stops a running timesheet by updating it with an end time
// This uses the PUT API to update the existing timesheet entry
func (c *Client) StopTimesheet(entry *TimelogEntry) error {
	return c.StopTimesheetWithDescription(entry, "")
}

// StopTimesheetWithDescription stops a running timesheet by updating it with an end time
// and optionally updates the description if newDescription is non-empty
func (c *Client) StopTimesheetWithDescription(entry *TimelogEntry, newDescription string) error {
	if entry == nil {
		return fmt.Errorf("entry cannot be nil")
	}

	// Parse the start time from the entry
	startTime, err := entry.GetFromTimeAsTime()
	if err != nil {
		return fmt.Errorf("failed to parse start time: %w", err)
	}

	// Use current time as end time
	endTime := time.Now().UTC()

	// Build the API request payload matching the UpdateTimesheet format
	apiEntry := map[string]interface{}{
		"project":    map[string]interface{}{"id": entry.ProjectID},
		"activity":   map[string]interface{}{"id": entry.ActivityID},
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
		"timezone":   "UTC",
	}

	// Handle description - use new description if provided, otherwise keep existing
	if newDescription != "" {
		apiEntry["description"] = newDescription
	} else if entry.Description != nil {
		apiEntry["description"] = *entry.Description
	} else {
		apiEntry["description"] = ""
	}

	// Handle task - can be nil or have a value
	taskID := entry.TaskID.Value
	if taskID > 0 {
		apiEntry["task"] = map[string]interface{}{"id": taskID}
	} else {
		apiEntry["task"] = map[string]interface{}{"id": "-"}
	}

	endpoint := fmt.Sprintf("/api/timesheet/%d/", entry.ID)
	resp, err := c.makeRequest("PUT", endpoint, apiEntry)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to stop timesheet (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteTimeLog deletes a time log entry
func (c *Client) DeleteTimeLog(id string) error {
	req := DeleteTimeLogRequest{ID: id}
	resp, err := c.makeRequest("POST", "/api/delete-time-log/", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete time log (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetActiveTimesheet returns the currently running timesheet (if any)
func (c *Client) GetActiveTimesheet() (*TimelogEntry, error) {
	timelogs, err := c.GetTimelogs()
	if err != nil {
		return nil, err
	}

	// Find the first entry where to_time is empty (running timer)
	for _, entry := range timelogs {
		if entry.ToTime == "" {
			return &entry, nil
		}
	}

	return nil, nil // No active timesheet
}

// GetUnsubmittedTimesheets returns timesheets that are stopped but not submitted
func (c *Client) GetUnsubmittedTimesheets() ([]TimelogEntry, error) {
	timelogs, err := c.GetTimelogs()
	if err != nil {
		return nil, err
	}

	var unsubmitted []TimelogEntry
	for _, entry := range timelogs {
		// Entry must have both from_time and to_time (stopped) and not be submitted
		if entry.ToTime != "" && !entry.Submitted {
			unsubmitted = append(unsubmitted, entry)
		}
	}

	return unsubmitted, nil
}