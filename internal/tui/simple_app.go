package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/service"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// Message types
type tickMsg time.Time
type activeEntryMsg *models.ActiveTimeEntry
type projectsLoadedMsg []models.Project
type activitiesLoadedMsg []models.Activity
type entriesLoadedMsg struct {
	entries []models.TimeEntry
	total   float64
}
type errorMsg error

// SimpleApp represents a simplified TUI application
type SimpleApp struct {
	service        *service.TimesheetService
	width          int
	height         int
	activeEntry    *models.ActiveTimeEntry
	projects       []models.Project
	activities     []models.Activity
	currentView    string // "entry", "list", "submit"
	selectedProject string
	selectedActivity string
	selectedTask   *string
	description    string
	entries        []models.TimeEntry
	form           *huh.Form
	err            error
}

// NewSimpleApp creates a simplified app
func NewSimpleApp() (*SimpleApp, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	
	dataDir := filepath.Join(homeDir, ".kartoza-timesheet")
	storage, err := storage.New(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	service := service.New(storage, "default-user")

	app := &SimpleApp{
		service:     service,
		currentView: "entry",
	}

	app.initForm()
	return app, nil
}

func (a *SimpleApp) Init() tea.Cmd {
	return tea.Batch(
		a.loadData(),
		a.loadActiveEntry(),
		a.form.Init(),
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

func (a *SimpleApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		case "1":
			a.currentView = "entry"
		case "2":
			a.currentView = "list"
			cmds = append(cmds, a.loadTodaysEntries())
		case "3":
			a.currentView = "submit"
		case "enter":
			if a.currentView == "entry" {
				if a.activeEntry != nil {
					cmds = append(cmds, a.stopTimeEntry())
				} else if a.canStartEntry() {
					cmds = append(cmds, a.startTimeEntry())
				}
			}
		}

		// Update form
		if a.form != nil {
			form, cmd := a.form.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				a.form = f
				cmds = append(cmds, cmd)
				a.updateFormValues()
			}
		}

	case tickMsg:
		cmds = append(cmds, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}))

	case activeEntryMsg:
		a.activeEntry = (*models.ActiveTimeEntry)(msg)

	case projectsLoadedMsg:
		a.projects = []models.Project(msg)
		a.updateFormOptions()

	case activitiesLoadedMsg:
		a.activities = []models.Activity(msg)
		a.updateFormOptions()

	case entriesLoadedMsg:
		a.entries = msg.entries

	case errorMsg:
		a.err = error(msg)
	}

	return a, tea.Batch(cmds...)
}

func (a *SimpleApp) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var content string
	switch a.currentView {
	case "entry":
		content = a.renderEntryView()
	case "list":
		content = a.renderListView()
	case "submit":
		content = a.renderSubmitView()
	}

	nav := a.renderNav()
	
	if a.err != nil {
		errorMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Render(fmt.Sprintf("Error: %v", a.err))
		content = lipgloss.JoinVertical(lipgloss.Top, content, errorMsg)
	}

	return lipgloss.JoinVertical(lipgloss.Top, nav, content)
}

func (a *SimpleApp) renderNav() string {
	activeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("4")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)
	
	inactiveStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("8")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	var entry, listing, submission string
	
	if a.currentView == "entry" {
		entry = activeStyle.Render("1. Entry")
	} else {
		entry = inactiveStyle.Render("1. Entry")
	}
	
	if a.currentView == "list" {
		listing = activeStyle.Render("2. Listing")
	} else {
		listing = inactiveStyle.Render("2. Listing")
	}
	
	if a.currentView == "submit" {
		submission = activeStyle.Render("3. Submission")
	} else {
		submission = inactiveStyle.Render("3. Submission")
	}

	status := "● Idle"
	if a.activeEntry != nil {
		status = "● Recording"
	}

	nav := lipgloss.JoinHorizontal(lipgloss.Top, entry, listing, submission)
	return fmt.Sprintf("%s    %s", nav, status)
}

func (a *SimpleApp) renderEntryView() string {
	timer := "00:00:00"
	if a.activeEntry != nil {
		elapsed := time.Since(a.activeEntry.StartTime)
		hours := int(elapsed.Hours())
		minutes := int(elapsed.Minutes()) % 60
		seconds := int(elapsed.Seconds()) % 60
		timer = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}

	timerStyle := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Bold(true).
		Foreground(lipgloss.Color("4")).
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Margin(1, 0)

	sections := []string{
		timerStyle.Render(timer),
	}

	if a.activeEntry != nil {
		activeInfo := fmt.Sprintf("📊 Project: %s\n⚡ Activity: %s",
			a.activeEntry.ProjectName, a.activeEntry.ActivityName)
		if a.activeEntry.TaskName != "" {
			activeInfo += fmt.Sprintf("\n📝 Task: %s", a.activeEntry.TaskName)
		}
		
		activeStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("2")).
			Padding(1).
			Margin(1, 0)

		sections = append(sections, activeStyle.Render(activeInfo))
		sections = append(sections, "Press Enter to STOP")
	} else {
		if a.form != nil {
			sections = append(sections, a.form.View())
		}
		if a.canStartEntry() {
			sections = append(sections, "Press Enter to START")
		}
	}

	return lipgloss.JoinVertical(lipgloss.Top, sections...)
}

func (a *SimpleApp) renderListView() string {
	var content []string
	
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("4")).
		Render("📅 Today's Time Entries")
	
	content = append(content, header)

	if len(a.entries) == 0 {
		content = append(content, "No entries for today")
	} else {
		var totalHours float64
		for _, entry := range a.entries {
			totalHours += entry.Duration
			
			timeStr := entry.StartTime.Format("15:04")
			if entry.EndTime != nil {
				timeStr += " - " + entry.EndTime.Format("15:04")
			}
			
			entryStr := fmt.Sprintf("%s | %s | %s | %.2fh",
				timeStr, entry.ProjectName, entry.ActivityName, entry.Duration)
			
			if entry.TaskName != "" {
				entryStr += fmt.Sprintf(" | %s", entry.TaskName)
			}
			
			content = append(content, entryStr)
		}
		
		summary := fmt.Sprintf("\n📈 Total: %.2f hours", totalHours)
		content = append(content, summary)
	}

	return lipgloss.JoinVertical(lipgloss.Top, content...)
}

func (a *SimpleApp) renderSubmitView() string {
	return "📤 Timesheet Submission\n\n[Feature not implemented in simplified version]"
}

func (a *SimpleApp) initForm() {
	a.form = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Key("project").
				Title("Project").
				Options(huh.NewOption("Loading...", "")).
				Value(&a.selectedProject),
			
			huh.NewSelect[string]().
				Key("activity").
				Title("Activity").
				Options(huh.NewOption("Loading...", "")).
				Value(&a.selectedActivity),
			
			huh.NewInput().
				Key("description").
				Title("Description (Optional)").
				Value(&a.description),
		),
	)
}

func (a *SimpleApp) updateFormValues() {
	a.selectedProject = a.form.GetString("project")
	a.selectedActivity = a.form.GetString("activity")
	a.description = a.form.GetString("description")
}

func (a *SimpleApp) updateFormOptions() {
	// This is simplified - in a full implementation, we would recreate the form
	// with new options when data is loaded
}

func (a *SimpleApp) canStartEntry() bool {
	return a.selectedProject != "" && a.selectedActivity != ""
}

// Command creators
func (a *SimpleApp) loadData() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			projects, err := a.service.GetProjects()
			if err != nil {
				return errorMsg(err)
			}
			return projectsLoadedMsg(projects)
		},
		func() tea.Msg {
			activities, err := a.service.GetActivities()
			if err != nil {
				return errorMsg(err)
			}
			return activitiesLoadedMsg(activities)
		},
	)
}

func (a *SimpleApp) loadActiveEntry() tea.Cmd {
	return func() tea.Msg {
		entry, err := a.service.GetActiveTimeEntry()
		if err != nil {
			return errorMsg(err)
		}
		return activeEntryMsg(entry)
	}
}

func (a *SimpleApp) loadTodaysEntries() tea.Cmd {
	return func() tea.Msg {
		entries, err := a.service.GetTodaysEntries()
		if err != nil {
			return errorMsg(err)
		}
		
		var total float64
		for _, entry := range entries {
			total += entry.Duration
		}
		
		return entriesLoadedMsg{entries: entries, total: total}
	}
}

func (a *SimpleApp) startTimeEntry() tea.Cmd {
	return func() tea.Msg {
		entry, err := a.service.StartTimeEntry(
			a.selectedProject,
			a.selectedActivity,
			a.selectedTask,
			a.description,
		)
		if err != nil {
			return errorMsg(err)
		}
		return activeEntryMsg(entry.ToActiveTimeEntry())
	}
}

func (a *SimpleApp) stopTimeEntry() tea.Cmd {
	return func() tea.Msg {
		if err := a.service.StopActiveTimeEntry(); err != nil {
			return errorMsg(err)
		}
		return activeEntryMsg(nil)
	}
}

func (a *SimpleApp) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()
	return err
}