package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaClient handles communication with a local Ollama instance.
// Uses plain HTTP to avoid adding the ollama/api dependency.
type OllamaClient struct {
	endpoint string
	model    string
	timeout  time.Duration
	client   *http.Client
}

// ollamaRequest is the JSON body for /api/generate
type ollamaRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// ollamaResponse is the JSON response from /api/generate (non-streaming)
type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(endpoint, model string) *OllamaClient {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3.2"
	}

	return &OllamaClient{
		endpoint: endpoint,
		model:    model,
		timeout:  90 * time.Second,
		client:   &http.Client{Timeout: 90 * time.Second},
	}
}

// IsAvailable checks whether Ollama is running and reachable
func (c *OllamaClient) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// GetModel returns the current model name
func (c *OllamaClient) GetModel() string {
	return c.model
}

// SetModel changes the model
func (c *OllamaClient) SetModel(model string) {
	c.model = model
}

// QueryTimesheets sends a natural language query to Ollama with timesheet context
// and returns a structured response
func (c *OllamaClient) QueryTimesheets(query string, ctx *TimesheetContext) (string, error) {
	prompt := c.buildTimesheetPrompt(query, ctx)
	return c.generate(prompt)
}

// generate sends a prompt to Ollama and returns the response text
func (c *OllamaClient) generate(prompt string) (string, error) {
	reqBody := ollamaRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0.3,
			"num_predict": 800,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpCtx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, "POST", c.endpoint+"/api/generate", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return strings.TrimSpace(result.Response), nil
}

// buildTimesheetPrompt creates the prompt with timesheet context
func (c *OllamaClient) buildTimesheetPrompt(query string, ctx *TimesheetContext) string {
	var sb strings.Builder

	sb.WriteString("You are a timesheet assistant for Kartoza, a geospatial open source company. ")
	sb.WriteString("Help users find the right project/task/activity combination for logging time, ")
	sb.WriteString("answer questions about their timesheet data, provide time tracking advice, ")
	sb.WriteString("and analyze productivity patterns.\n\n")

	sb.WriteString(DomainKnowledge)
	sb.WriteString("\n")

	sb.WriteString("SUPPORTED QUERY TYPES (you can answer all of these):\n")
	sb.WriteString(GetQueryReference())
	sb.WriteString("\n")

	sb.WriteString("RULES:\n")
	sb.WriteString("1. For project/task/activity matching questions, respond with a JSON array of matches:\n")
	sb.WriteString("   [{\"project\":\"Name\",\"task\":\"Name\",\"activity\":\"Name\",\"reason\":\"why\"}]\n")
	sb.WriteString("2. For analytical questions about the user's data, respond with plain text analysis using the data provided below.\n")
	sb.WriteString("3. For general time tracking questions, use your domain expertise to give practical advice.\n")
	sb.WriteString("4. Be concise and direct.\n\n")

	// Add available projects/tasks/activities
	if ctx != nil && len(ctx.Projects) > 0 {
		sb.WriteString("AVAILABLE PROJECTS AND TASKS:\n")
		for _, p := range ctx.Projects {
			sb.WriteString(fmt.Sprintf("- %s (ID:%d)\n", p.Name, p.ID))
			for _, t := range p.Tasks {
				sb.WriteString(fmt.Sprintf("    Task: %s (ID:%d)\n", t.Name, t.ID))
			}
		}
		sb.WriteString("\n")
	}

	if ctx != nil && len(ctx.Activities) > 0 {
		sb.WriteString("AVAILABLE ACTIVITIES:\n")
		for _, a := range ctx.Activities {
			sb.WriteString(fmt.Sprintf("- %s (ID:%d)\n", a.Name, a.ID))
		}
		sb.WriteString("\n")
	}

	// Add recent history summary for analytics queries
	if ctx != nil && len(ctx.Entries) > 0 {
		sb.WriteString("RECENT TIMESHEET ENTRIES (last 50):\n")
		count := len(ctx.Entries)
		if count > 50 {
			count = 50
		}
		for i := 0; i < count; i++ {
			e := ctx.Entries[i]
			sb.WriteString(fmt.Sprintf("- %s: %s / %s / %s - %.1fh",
				e.StartTime.Format("2006-01-02"),
				e.ProjectName, e.TaskName, e.ActivityName, e.Hours))
			if e.Description != "" {
				desc := e.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				sb.WriteString(fmt.Sprintf(" \"%s\"", desc))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("USER QUESTION: " + query + "\n\n")
	sb.WriteString("RESPONSE:\n")

	return sb.String()
}

// ParseMatchResponse attempts to parse an LLM response as project/task/activity matches
func ParseMatchResponse(response string, ctx *TimesheetContext) []TimesheetMatch {
	// Try to extract JSON array from the response
	response = strings.TrimSpace(response)

	// Find JSON array in the response
	startIdx := strings.Index(response, "[")
	endIdx := strings.LastIndex(response, "]")
	if startIdx < 0 || endIdx < 0 || endIdx <= startIdx {
		return nil
	}

	jsonStr := response[startIdx : endIdx+1]

	var rawMatches []struct {
		Project  string `json:"project"`
		Task     string `json:"task"`
		Activity string `json:"activity"`
		Reason   string `json:"reason"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawMatches); err != nil {
		return nil
	}

	if ctx == nil {
		return nil
	}

	// Resolve names to IDs using the context
	var matches []TimesheetMatch
	for _, raw := range rawMatches {
		match := TimesheetMatch{
			ProjectName:  raw.Project,
			TaskName:     raw.Task,
			ActivityName: raw.Activity,
			Reason:       raw.Reason,
			Confidence:   0.8, // LLM matches get a default confidence
		}

		// Resolve project ID
		for _, p := range ctx.Projects {
			if strings.EqualFold(p.Name, raw.Project) || strings.Contains(strings.ToLower(p.Name), strings.ToLower(raw.Project)) {
				match.ProjectID = p.ID
				match.ProjectName = p.Name

				// Resolve task ID
				for _, t := range p.Tasks {
					if strings.EqualFold(t.Name, raw.Task) || strings.Contains(strings.ToLower(t.Name), strings.ToLower(raw.Task)) {
						match.TaskID = t.ID
						match.TaskName = t.Name
						break
					}
				}
				break
			}
		}

		// Resolve activity ID
		for _, a := range ctx.Activities {
			if strings.EqualFold(a.Name, raw.Activity) || strings.Contains(strings.ToLower(a.Name), strings.ToLower(raw.Activity)) {
				match.ActivityID = a.ID
				match.ActivityName = a.Name
				break
			}
		}

		matches = append(matches, match)
	}

	return matches
}
