package screens

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kartoza/go-timesheets-go/internal/ai"
	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/gui/util"
)

// AIAssistantScreen provides a natural language interface for finding
// project/task/activity combinations and analyzing timesheet data.
type AIAssistantScreen struct {
	Container fyne.CanvasObject

	apiClient *api.Client
	window    fyne.Window
	assistant *ai.TimesheetAssistant

	// Widgets
	queryEntry    *widget.Entry
	askButton     *widget.Button
	statusLabel   *canvas.Text
	resultsScroll *container.Scroll
	resultsList   *fyne.Container

	// Callbacks
	OnBack       func()
	OnStartTimer func(projectID, taskID, activityID int, projectName, taskName, activityName string)
}

// NewAIAssistantScreen creates a new AI assistant screen
func NewAIAssistantScreen(apiClient *api.Client, window fyne.Window) *AIAssistantScreen {
	s := &AIAssistantScreen{
		apiClient: apiClient,
		window:    window,
		assistant: ai.NewTimesheetAssistant(),
	}
	s.build()
	return s
}

func (s *AIAssistantScreen) build() {
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	darkBg := color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}
	_ = darkBg

	// Header
	titleText := canvas.NewText("AI Assistant", goldColor)
	titleText.TextSize = 20
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.Alignment = fyne.TextAlignCenter

	subtitleText := canvas.NewText("Ask questions about your timesheets", color.NRGBA{R: 0xAA, G: 0xAA, B: 0xAA, A: 0xFF})
	subtitleText.TextSize = 12
	subtitleText.Alignment = fyne.TextAlignCenter

	header := container.NewVBox(titleText, subtitleText)

	// Query input
	s.queryEntry = widget.NewMultiLineEntry()
	s.queryEntry.SetPlaceHolder("e.g., Which project should I log GeoServer Docker work to?\nor: How consistent was my timekeeping?")
	s.queryEntry.SetMinRowsVisible(3)
	s.queryEntry.OnSubmitted = func(_ string) {
		s.executeQuery()
	}

	s.askButton = widget.NewButton("Ask", func() {
		s.executeQuery()
	})
	s.askButton.Importance = widget.HighImportance

	queryBox := container.NewBorder(nil, nil, nil, s.askButton, s.queryEntry)

	// Status label
	s.statusLabel = canvas.NewText("Loading AI status...", color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xFF})
	s.statusLabel.TextSize = 11

	// Results area
	s.resultsList = container.NewVBox()
	s.resultsScroll = container.NewVScroll(s.resultsList)
	s.resultsScroll.SetMinSize(fyne.NewSize(400, 300))

	// Example queries
	examplesLabel := canvas.NewText("Example queries:", color.NRGBA{R: 0x99, G: 0x99, B: 0x99, A: 0xFF})
	examplesLabel.TextSize = 11

	examples := []string{
		"Which project for GeoServer Docker?",
		"Which projects did I work on last week?",
		"How consistent was my timekeeping?",
		"Which project did I spend the most time on?",
	}

	exampleButtons := container.NewGridWrap(fyne.NewSize(220, 30))
	for _, ex := range examples {
		exText := ex
		btn := widget.NewButton(exText, func() {
			s.queryEntry.SetText(exText)
			s.executeQuery()
		})
		btn.Importance = widget.LowImportance
		exampleButtons.Add(btn)
	}

	// Build layout
	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		queryBox,
		s.statusLabel,
		widget.NewSeparator(),
		s.resultsScroll,
		layout.NewSpacer(),
		examplesLabel,
		exampleButtons,
	)

	s.Container = container.NewPadded(content)
}

// Refresh updates the AI context with current data from the API
func (s *AIAssistantScreen) Refresh() {
	// Load context asynchronously
	util.RunAsync(func() (*ai.TimesheetContext, error) {
		return s.loadContext()
	}, func(ctx *ai.TimesheetContext, err error) {
		if err != nil {
			s.statusLabel.Text = "Failed to load data: " + err.Error()
			s.statusLabel.Refresh()
			return
		}

		s.assistant.SetContext(ctx)

		// Train NN in background if we have entries
		if len(ctx.Entries) > 0 {
			s.assistant.TrainFromHistoryAsync(ctx.Entries, nil)
		}

		// Update status
		s.updateStatus()
	})
}

func (s *AIAssistantScreen) loadContext() (*ai.TimesheetContext, error) {
	ctx := &ai.TimesheetContext{}

	// Load projects
	projects, err := s.apiClient.GetProjects("")
	if err != nil {
		return nil, fmt.Errorf("failed to load projects: %w", err)
	}

	for _, p := range projects {
		proj := ai.ProjectInfo{
			ID:   p.ID,
			Name: p.Label,
		}

		// Load tasks for this project
		tasks, err := s.apiClient.GetTasks(fmt.Sprintf("%d", p.ID))
		if err == nil {
			for _, t := range tasks {
				proj.Tasks = append(proj.Tasks, ai.TaskInfo{
					ID:   t.ID,
					Name: t.Label,
				})
			}
		}

		ctx.Projects = append(ctx.Projects, proj)
	}

	// Load activities
	activities, err := s.apiClient.GetActivities()
	if err == nil {
		for _, a := range activities {
			ctx.Activities = append(ctx.Activities, ai.ActivityInfo{
				ID:   a.ID,
				Name: a.Label,
			})
		}
	}

	// Load recent entries
	entries, err := s.apiClient.GetTimelogs()
	if err == nil {
		for _, e := range entries {
			startTime, _ := e.GetFromTimeAsTime()
			endTime, _ := e.GetToTimeAsTime()
			ctx.Entries = append(ctx.Entries, ai.EntryInfo{
				ProjectName:  e.ProjectName,
				TaskName:     e.TaskName,
				ActivityName: e.ActivityType,
				StartTime:    startTime,
				EndTime:      endTime,
				Hours:        e.Hours,
				Description:  derefStr(e.Description),
				Submitted:    e.Submitted,
			})
		}
	}

	return ctx, nil
}

func (s *AIAssistantScreen) updateStatus() {
	status := s.assistant.GetStatus()

	var parts []string
	if v, ok := status["embedded_loaded"].(bool); ok && v {
		parts = append(parts, "LLM: embedded")
	}
	if v, ok := status["ollama_available"].(bool); ok && v {
		model := "llama3.2"
		if m, ok := status["ollama_model"].(string); ok && m != "" {
			model = m
		}
		parts = append(parts, fmt.Sprintf("Ollama: %s", model))
	}
	if v, ok := status["nn_trained"].(bool); ok && v {
		parts = append(parts, "NN: trained")
	}
	parts = append(parts, "Fuzzy: ready")

	s.statusLabel.Text = "AI backends: " + strings.Join(parts, " | ")
	s.statusLabel.Refresh()
}

func (s *AIAssistantScreen) executeQuery() {
	query := strings.TrimSpace(s.queryEntry.Text)
	if query == "" {
		return
	}

	// Disable input while processing
	s.askButton.Disable()
	s.queryEntry.Disable()
	s.statusLabel.Text = "Thinking..."
	s.statusLabel.Refresh()

	// Clear previous results
	s.resultsList.Objects = nil
	s.resultsList.Refresh()

	util.RunAsync(func() (*ai.QueryResult, error) {
		return s.assistant.Query(query)
	}, func(result *ai.QueryResult, err error) {
		s.askButton.Enable()
		s.queryEntry.Enable()
		s.updateStatus()

		if err != nil {
			s.showError(err.Error())
			return
		}

		s.showResult(result)
	})
}

func (s *AIAssistantScreen) showResult(result *ai.QueryResult) {
	s.resultsList.Objects = nil

	switch result.Type {
	case ai.ResultMatches:
		s.showMatches(result.Matches)
	case ai.ResultAnalysis:
		s.showAnalysis(result.Analysis)
	case ai.ResultText:
		s.showText(result.Text)
	case ai.ResultError:
		s.showError(result.Text)
	}

	s.resultsList.Refresh()
}

func (s *AIAssistantScreen) showMatches(matches []ai.TimesheetMatch) {
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}

	headerText := canvas.NewText(fmt.Sprintf("Found %d match(es) - tap to start timer:", len(matches)), goldColor)
	headerText.TextSize = 14
	headerText.TextStyle = fyne.TextStyle{Bold: true}
	s.resultsList.Add(headerText)
	s.resultsList.Add(widget.NewSeparator())

	for _, m := range matches {
		match := m // capture for closure

		// Build display text
		title := match.ProjectName
		if match.TaskName != "" {
			title += " / " + match.TaskName
		}
		if match.ActivityName != "" {
			title += " / " + match.ActivityName
		}

		confidence := fmt.Sprintf("%.0f%% confidence", match.Confidence*100)
		reason := match.Reason

		// Card for each match
		titleLabel := widget.NewLabel(title)
		titleLabel.TextStyle = fyne.TextStyle{Bold: true}
		titleLabel.Wrapping = fyne.TextWrapWord

		detailText := confidence
		if reason != "" {
			detailText += " - " + reason
		}
		detailLabel := widget.NewLabel(detailText)
		detailLabel.TextStyle = fyne.TextStyle{Italic: true}
		detailLabel.Wrapping = fyne.TextWrapWord

		startBtn := widget.NewButton("Start Timer", func() {
			if s.OnStartTimer != nil {
				s.OnStartTimer(match.ProjectID, match.TaskID, match.ActivityID,
					match.ProjectName, match.TaskName, match.ActivityName)
			}
		})
		startBtn.Importance = widget.HighImportance

		card := container.NewVBox(
			titleLabel,
			detailLabel,
			startBtn,
			widget.NewSeparator(),
		)
		s.resultsList.Add(card)
	}
}

func (s *AIAssistantScreen) showAnalysis(analysis *ai.AnalysisResult) {
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}

	titleText := canvas.NewText(analysis.Title, goldColor)
	titleText.TextSize = 16
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	s.resultsList.Add(titleText)
	s.resultsList.Add(widget.NewSeparator())

	if analysis.Summary != "" {
		summaryLabel := widget.NewLabel(analysis.Summary)
		summaryLabel.Wrapping = fyne.TextWrapWord
		s.resultsList.Add(summaryLabel)
		s.resultsList.Add(widget.NewSeparator())
	}

	for _, d := range analysis.Details {
		row := container.NewHBox(
			widget.NewLabelWithStyle(d.Label, fyne.TextAlignLeading, fyne.TextStyle{}),
			layout.NewSpacer(),
			widget.NewLabelWithStyle(d.Value, fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		)
		s.resultsList.Add(row)
	}
}

func (s *AIAssistantScreen) showText(text string) {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	s.resultsList.Add(label)
}

func (s *AIAssistantScreen) showError(msg string) {
	errColor := color.NRGBA{R: 0xFF, G: 0x44, B: 0x44, A: 0xFF}
	errText := canvas.NewText("Error: "+msg, errColor)
	errText.TextSize = 13
	s.resultsList.Add(errText)
	s.resultsList.Refresh()
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
