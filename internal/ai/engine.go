package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TimesheetAssistant is the main AI engine for the timesheet app.
// It uses a multi-level fallback chain (adapted from kartoza-pg-ai QueryEngine):
//
//  1. Embedded LLM (llama.cpp) - requires "llamacpp" build tag
//  2. Ollama (local LLM server) - HTTP-based, no CGO needed
//  3. Neural network (TF-IDF similarity) - trained from user history
//  4. Rule-based fuzzy matching - always available
//
// For analytical queries (e.g., "how consistent was my timekeeping?"),
// the built-in TimesheetAnalyzer handles them directly without LLM.
type TimesheetAssistant struct {
	context      *TimesheetContext
	embeddedLLM  *EmbeddedLLM
	ollamaClient *OllamaClient
	nnTrainer    *NNTrainer
	matcher      *FuzzyMatcher
	analyzer     *TimesheetAnalyzer

	// Configuration
	useEmbedded bool
	useOllama   bool
	useNN       bool

	// Query log path
	queryLogPath string

	// CSV import data for LLM prompt enrichment and analytical queries
	historySummary *HistorySummary
	csvEntries    []EntryInfo

	mu sync.RWMutex
}

// NewTimesheetAssistant creates a new AI assistant with the full fallback chain
func NewTimesheetAssistant() *TimesheetAssistant {
	// Default query log location
	homeDir, _ := os.UserHomeDir()
	logPath := ""
	if homeDir != "" {
		logPath = filepath.Join(homeDir, ".config", "kartoza-timesheets", "ai-query.log")
	}

	a := &TimesheetAssistant{
		matcher:      NewFuzzyMatcher(),
		analyzer:     NewTimesheetAnalyzer(),
		useEmbedded:  true,
		useOllama:    true,
		useNN:        true,
		queryLogPath: logPath,
	}

	// Initialize neural network trainer
	trainer, err := NewNNTrainer()
	if err == nil {
		a.nnTrainer = trainer
	}

	// Try to find and load a default embedded model
	if EmbeddedLLMAvailable() {
		if modelPath := FindDefaultModel(); modelPath != "" {
			a.initEmbeddedLLM(modelPath)
		}
	}

	// Initialize Ollama client
	a.ollamaClient = NewOllamaClient("", "")

	return a
}

// initEmbeddedLLM initializes the embedded LLM with the given model path
func (a *TimesheetAssistant) initEmbeddedLLM(modelPath string) {
	cfg := DefaultEmbeddedLLMConfig()
	cfg.ModelPath = modelPath

	llm, err := NewEmbeddedLLM(cfg)
	if err != nil {
		return
	}

	if err := llm.LoadModel(modelPath); err != nil {
		return
	}

	a.embeddedLLM = llm
}

// SetContext updates the timesheet context (projects, tasks, activities, entries)
func (a *TimesheetAssistant) SetContext(ctx *TimesheetContext) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.context = ctx
}

// GetContext returns the current timesheet context
func (a *TimesheetAssistant) GetContext() *TimesheetContext {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.context
}

// SetQueryLogPath sets the file path for logging user queries
func (a *TimesheetAssistant) SetQueryLogPath(path string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.queryLogPath = path
}

// logQuery appends a timestamped query to the query log file
func (a *TimesheetAssistant) logQuery(query string) {
	a.mu.RLock()
	logPath := a.queryLogPath
	a.mu.RUnlock()

	if logPath == "" {
		return
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "%s\t%s\n", timestamp, query)
}

// Query processes a natural language query and returns the result.
// This is the main entry point for the AI assistant.
func (a *TimesheetAssistant) Query(query string) (*QueryResult, error) {
	a.mu.RLock()
	ctx := a.context
	a.mu.RUnlock()

	if ctx == nil {
		return nil, fmt.Errorf("no timesheet context loaded")
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	// Log every user query
	a.logQuery(query)

	// Step 1: Check if this is an analytical query - handle locally without LLM
	if a.analyzer.IsAnalyticalQuery(query) {
		// Merge API entries with CSV history for comprehensive analysis
		allEntries := ctx.Entries
		a.mu.RLock()
		csvEntries := a.csvEntries
		a.mu.RUnlock()
		if len(csvEntries) > 0 {
			allEntries = append(allEntries, csvEntries...)
		}
		result := a.analyzer.Analyze(query, allEntries)
		if result != nil {
			return result, nil
		}
	}

	// Step 2: Try embedded LLM
	if a.useEmbedded && a.embeddedLLM != nil && a.embeddedLLM.IsLoaded() {
		result, err := a.queryEmbeddedLLM(query, ctx)
		if err == nil && result != nil && len(result.Matches) > 0 {
			return result, nil
		}
	}

	// Step 3: Try Ollama
	if a.useOllama && a.ollamaClient != nil && a.ollamaClient.IsAvailable() {
		result, err := a.queryOllama(query, ctx)
		if err == nil && result != nil {
			return result, nil
		}
	}

	// Step 4: Try neural network
	if a.useNN && a.nnTrainer != nil && a.nnTrainer.IsTrained() {
		matches, err := a.nnTrainer.Predict(query, 5)
		if err == nil && len(matches) > 0 {
			// Resolve IDs from context
			resolvedMatches := resolveMatchIDs(matches, ctx)
			return &QueryResult{
				Type:    ResultMatches,
				Matches: resolvedMatches,
				Text:    formatMatchesText(resolvedMatches),
			}, nil
		}
	}

	// Step 5: Fall back to rule-based fuzzy matching
	matches := a.matcher.Match(query, ctx)
	if len(matches) > 0 {
		return &QueryResult{
			Type:    ResultMatches,
			Matches: matches,
			Text:    formatMatchesText(matches),
		}, nil
	}

	return &QueryResult{
		Type: ResultText,
		Text: "No matching projects, tasks, or activities found for your query. Try rephrasing or use more specific project/task names.",
	}, nil
}

// queryEmbeddedLLM queries the embedded llama.cpp model
func (a *TimesheetAssistant) queryEmbeddedLLM(query string, ctx *TimesheetContext) (*QueryResult, error) {
	// Build a prompt similar to the Ollama prompt
	prompt := buildLLMPrompt(query, ctx, a.historySummary)

	response, err := a.embeddedLLM.Generate(prompt)
	if err != nil {
		return nil, err
	}

	return parseLLMResponse(response, ctx), nil
}

// queryOllama queries the Ollama LLM server
func (a *TimesheetAssistant) queryOllama(query string, ctx *TimesheetContext) (*QueryResult, error) {
	response, err := a.ollamaClient.QueryTimesheets(query, ctx, a.historySummary)
	if err != nil {
		return nil, err
	}

	return parseLLMResponse(response, ctx), nil
}

// parseLLMResponse parses an LLM response into a QueryResult
func parseLLMResponse(response string, ctx *TimesheetContext) *QueryResult {
	// Try to parse as JSON matches first
	matches := ParseMatchResponse(response, ctx)
	if len(matches) > 0 {
		return &QueryResult{
			Type:    ResultMatches,
			Matches: matches,
			Text:    formatMatchesText(matches),
		}
	}

	// Fall back to plain text response
	return &QueryResult{
		Type: ResultText,
		Text: response,
	}
}

// buildLLMPrompt creates the prompt for either embedded or Ollama LLM
func buildLLMPrompt(query string, ctx *TimesheetContext, historySummary *HistorySummary) string {
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

	if ctx != nil && len(ctx.Projects) > 0 {
		sb.WriteString("AVAILABLE PROJECTS AND TASKS:\n")
		count := 0
		for _, p := range ctx.Projects {
			if count > 30 {
				sb.WriteString(fmt.Sprintf("... and %d more projects\n", len(ctx.Projects)-count))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", p.Name))
			for _, t := range p.Tasks {
				sb.WriteString(fmt.Sprintf("    Task: %s\n", t.Name))
			}
			count++
		}
		sb.WriteString("\n")
	}

	if ctx != nil && len(ctx.Activities) > 0 {
		sb.WriteString("AVAILABLE ACTIVITIES:\n")
		for _, act := range ctx.Activities {
			sb.WriteString(fmt.Sprintf("- %s\n", act.Name))
		}
		sb.WriteString("\n")
	}

	// Include historical work patterns if available
	if historySummary != nil {
		sb.WriteString(historySummary.FormatForPrompt())
	}

	sb.WriteString("USER QUESTION: " + query + "\n\n")
	sb.WriteString("RESPONSE:\n")

	return sb.String()
}

// resolveMatchIDs fills in IDs from the context for NN-predicted matches
func resolveMatchIDs(matches []TimesheetMatch, ctx *TimesheetContext) []TimesheetMatch {
	if ctx == nil {
		return matches
	}

	resolved := make([]TimesheetMatch, len(matches))
	copy(resolved, matches)

	for i := range resolved {
		m := &resolved[i]

		// Resolve project
		for _, p := range ctx.Projects {
			if strings.EqualFold(p.Name, m.ProjectName) {
				m.ProjectID = p.ID

				// Resolve task
				for _, t := range p.Tasks {
					if strings.EqualFold(t.Name, m.TaskName) {
						m.TaskID = t.ID
						break
					}
				}
				break
			}
		}

		// Resolve activity
		for _, a := range ctx.Activities {
			if strings.EqualFold(a.Name, m.ActivityName) {
				m.ActivityID = a.ID
				break
			}
		}
	}

	return resolved
}

// formatMatchesText creates a human-readable text from matches
func formatMatchesText(matches []TimesheetMatch) string {
	if len(matches) == 0 {
		return "No matches found."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d matching combination(s):\n", len(matches)))
	for i, m := range matches {
		sb.WriteString(fmt.Sprintf("\n%d. %s", i+1, m.ProjectName))
		if m.TaskName != "" {
			sb.WriteString(" / " + m.TaskName)
		}
		if m.ActivityName != "" {
			sb.WriteString(" / " + m.ActivityName)
		}
		sb.WriteString(fmt.Sprintf(" (%.0f%% confidence)", m.Confidence*100))
		if m.Reason != "" {
			sb.WriteString("\n   " + m.Reason)
		}
	}

	return sb.String()
}

// TrainFromHistory trains the neural network from timesheet history
func (a *TimesheetAssistant) TrainFromHistory(entries []EntryInfo) error {
	if a.nnTrainer == nil {
		return fmt.Errorf("neural network trainer not initialized")
	}
	return a.nnTrainer.Train(entries)
}

// TrainFromHistoryAsync trains the neural network asynchronously
func (a *TimesheetAssistant) TrainFromHistoryAsync(entries []EntryInfo, callback func(error)) {
	if a.nnTrainer == nil {
		if callback != nil {
			callback(fmt.Errorf("neural network trainer not initialized"))
		}
		return
	}
	a.nnTrainer.TrainAsync(entries, callback)
}

// TrainFromCSV imports CSV history data, trains the NN, and builds a history summary for LLM prompts.
func (a *TimesheetAssistant) TrainFromCSV(csvPath string) error {
	entries, err := ImportCSVHistory(csvPath)
	if err != nil {
		return fmt.Errorf("CSV import failed: %w", err)
	}

	if len(entries) == 0 {
		return fmt.Errorf("no entries found in CSV")
	}

	// Build history summary for LLM prompt enrichment
	summary := BuildHistorySummary(entries)
	a.mu.Lock()
	a.historySummary = summary
	a.csvEntries = entries
	a.mu.Unlock()

	// Merge with existing API entries if available
	a.mu.RLock()
	ctx := a.context
	a.mu.RUnlock()

	allEntries := entries
	if ctx != nil && len(ctx.Entries) > 0 {
		allEntries = append(allEntries, ctx.Entries...)
	}

	// Train the neural network on the combined dataset
	if a.nnTrainer != nil {
		return a.nnTrainer.Train(allEntries)
	}

	return nil
}

// TrainFromCSVAsync trains from CSV history asynchronously
func (a *TimesheetAssistant) TrainFromCSVAsync(csvPath string, callback func(int, error)) {
	go func() {
		entries, err := ImportCSVHistory(csvPath)
		if err != nil {
			if callback != nil {
				callback(0, fmt.Errorf("CSV import failed: %w", err))
			}
			return
		}

		if len(entries) == 0 {
			if callback != nil {
				callback(0, fmt.Errorf("no entries found in CSV"))
			}
			return
		}

		summary := BuildHistorySummary(entries)
		a.mu.Lock()
		a.historySummary = summary
		a.csvEntries = entries
		a.mu.Unlock()

		allEntries := entries
		a.mu.RLock()
		ctx := a.context
		a.mu.RUnlock()
		if ctx != nil && len(ctx.Entries) > 0 {
			allEntries = append(allEntries, ctx.Entries...)
		}

		if a.nnTrainer != nil {
			err = a.nnTrainer.Train(allEntries)
		}

		if callback != nil {
			callback(len(entries), err)
		}
	}()
}

// GetHistorySummary returns the history summary from CSV import
func (a *TimesheetAssistant) GetHistorySummary() *HistorySummary {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.historySummary
}

// SetUseOllama enables or disables Ollama
func (a *TimesheetAssistant) SetUseOllama(use bool) {
	a.useOllama = use
}

// SetUseNN enables or disables the neural network
func (a *TimesheetAssistant) SetUseNN(use bool) {
	a.useNN = use
}

// SetUseEmbedded enables or disables the embedded LLM
func (a *TimesheetAssistant) SetUseEmbedded(use bool) {
	a.useEmbedded = use
}

// SetOllamaModel changes the Ollama model
func (a *TimesheetAssistant) SetOllamaModel(model string) {
	if a.ollamaClient != nil {
		a.ollamaClient.SetModel(model)
	}
}

// SetEmbeddedModelPath loads a GGUF model for the embedded LLM
func (a *TimesheetAssistant) SetEmbeddedModelPath(path string) error {
	if !EmbeddedLLMAvailable() {
		return fmt.Errorf("embedded LLM not available (build with -tags llamacpp)")
	}

	if a.embeddedLLM == nil {
		cfg := DefaultEmbeddedLLMConfig()
		cfg.ModelPath = path
		llm, err := NewEmbeddedLLM(cfg)
		if err != nil {
			return err
		}
		a.embeddedLLM = llm
	}

	return a.embeddedLLM.LoadModel(path)
}

// GetStatus returns the status of all AI components
func (a *TimesheetAssistant) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"embedded_available": EmbeddedLLMAvailable(),
		"embedded_loaded":    false,
		"ollama_available":   false,
		"nn_trained":         false,
		"matcher_available":  true, // Always available
		"analyzer_available": true, // Always available
	}

	if a.embeddedLLM != nil {
		status["embedded_loaded"] = a.embeddedLLM.IsLoaded()
	}

	if a.ollamaClient != nil {
		status["ollama_available"] = a.ollamaClient.IsAvailable()
		status["ollama_model"] = a.ollamaClient.GetModel()
	}

	if a.nnTrainer != nil {
		status["nn_trained"] = a.nnTrainer.IsTrained()
		for k, v := range a.nnTrainer.GetStats() {
			status["nn_"+k] = v
		}
	}

	return status
}

// Close releases resources
func (a *TimesheetAssistant) Close() {
	if a.embeddedLLM != nil {
		a.embeddedLLM.Close()
	}
}
