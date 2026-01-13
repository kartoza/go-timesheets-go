package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/glamour"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/service"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// Message types
type tickMsg time.Time
type statusUpdateMsg string
type activeEntryMsg *models.ActiveTimeEntry
type projectsLoadedMsg struct {
	Projects    []api.ProjectListItem
	CacheKey    string
	AllProjects []api.ProjectListItem // Full API response for caching
	Query       string                // Original query for filtering
}
type projectSearchErrorMsg struct {
	Err      error
	CacheKey string
}
type tasksLoadedMsg []api.TaskListItem
type activitiesLoadedMsg []api.ActivityListItem
type historyLoadedMsg []api.TimelogEntry
type errorMsg error
type runningTimerDetectedMsg struct {
	entry    api.TimelogEntry
	project  api.ProjectListItem
	task     api.TaskListItem
	activity api.ActivityListItem
}
type timesheetSubmittedMsg struct {
	isRunning bool
	history   []api.TimelogEntry
}

// Tab represents different views
type Tab int

const (
	TabProjects Tab = iota
	TabProjectDetails
	TabActivities
	TabCurrent
	TabHistory
)

// TimesheetApp represents a simplified TUI application
type TimesheetApp struct {
	service         *service.TimesheetService
	apiClient       *api.Client
	username        string
	width           int
	height          int
	activeEntry     *models.ActiveTimeEntry
	currentTab      Tab

	// Data
	projects        []api.ProjectListItem
	tasks           []api.TaskListItem
	activities      []api.ActivityListItem
	history         []api.TimelogEntry

	// Selection state
	selectedProject    *api.ProjectListItem
	selectedTask       *api.TaskListItem
	selectedActivity   *api.ActivityListItem
	lastUsedActivityID int

	// Cursor positions for navigation
	projectCursor   int
	taskCursor      int
	activityCursor  int
	historyCursor   int

	// History tab pagination
	historyPage      int   // Current page (0-indexed)
	historyPageSize  int   // Items per page
	allHistory       []api.TimelogEntry // All history entries
	historyCursorPage int  // Cursor within current page

	// Search inputs
	projectSearchInput textinput.Model
	searchMode         bool // true when user is typing in search box

	// Project search cache - key is first 2 letters, value is cached projects
	projectCache map[string][]api.ProjectListItem
	// Track in-flight API requests to prevent duplicates
	projectSearchInFlight map[string]bool

	// Task cache - keyed by project ID
	taskCache          map[int][]api.TaskListItem
	taskCacheInFlight  map[int]bool

	// Activity cache - activities don't change often, global cache
	activityCache         []api.ActivityListItem
	activityCacheValid    bool
	activityCacheInFlight bool

	// Cached monthly hours to avoid recalculating on every render
	cachedMonthlyHours float64
	cachedMonth        time.Month
	cachedYear         int

	// Cached status to avoid API call on every render
	cachedStatus string
	lastStatusUpdate time.Time

	// Timer control form inputs (Tab 4)
	descriptionInput textinput.Model
	dateInput        textinput.Model
	startTimeInput   textinput.Model
	endTimeInput     textinput.Model
	focusedFormField int  // 0=description, 1=date, 2=start, 3=end, 4=submit
	formMode         bool // true when editing form
	editingEntryID   int  // ID of entry being edited, 0 for new entry
	showCalendar     bool // true when calendar popup is shown

	err            error
	successMessage string
}

// NewTimesheetApp creates a simplified app
func NewTimesheetApp() (*TimesheetApp, error) {
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

	// Try to load API client with saved token
	var apiClient *api.Client
	token, err := config.LoadToken()
	if err == nil && token != nil {
		apiClient, err = api.NewClient(api.Config{
			BaseURL:   token.BaseURL,
			AuthToken: token.Token,
			Timeout:   30,
		})
		if err != nil {
			// Failed to create client, continue without API
			apiClient = nil
		}
	}

	// Initialize project search input
	projectSearchInput := textinput.New()
	projectSearchInput.Placeholder = "Type project name (min 2 chars)..."
	projectSearchInput.CharLimit = 100
	projectSearchInput.Width = 50
	projectSearchInput.Focus() // Focus the input so it accepts typing

	// Initialize timer form inputs
	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "Description..."
	descriptionInput.CharLimit = 200
	descriptionInput.Width = 60

	dateInput := textinput.New()
	dateInput.Placeholder = "YYYY-MM-DD"
	dateInput.CharLimit = 10
	dateInput.Width = 15
	dateInput.SetValue(time.Now().Format("2006-01-02"))

	startTimeInput := textinput.New()
	startTimeInput.Placeholder = "HH:MM"
	startTimeInput.CharLimit = 5
	startTimeInput.Width = 10

	endTimeInput := textinput.New()
	endTimeInput.Placeholder = "HH:MM"
	endTimeInput.CharLimit = 5
	endTimeInput.Width = 10

	app := &TimesheetApp{
		service:               service,
		apiClient:             apiClient,
		currentTab:            TabProjects,
		projectSearchInput:    projectSearchInput,
		searchMode:            true, // Start in search mode on Tab 1
		descriptionInput:      descriptionInput,
		dateInput:             dateInput,
		startTimeInput:        startTimeInput,
		endTimeInput:          endTimeInput,
		historyPageSize:       30,
		historyPage:           0,
		projectCache:          make(map[string][]api.ProjectListItem), // Initialize project cache
		projectSearchInFlight: make(map[string]bool),                  // Initialize in-flight tracker
		taskCache:             make(map[int][]api.TaskListItem),       // Initialize task cache
		taskCacheInFlight:     make(map[int]bool),                     // Initialize task in-flight tracker
		activityCacheValid:    false,                                  // Activities not yet cached
		activityCacheInFlight: false,                                  // No activity request in flight
		cachedStatus:          "Idle",                                 // Initialize status cache
	}

	// Load session state to restore previous selections
	app.loadSessionState()

	return app, nil
}

// NewTimesheetAppWithClient creates a new timesheet app with a provided API client
func NewTimesheetAppWithClient(apiClient *api.Client, username string) (*TimesheetApp, error) {
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

	// Initialize project search input
	projectSearchInput := textinput.New()
	projectSearchInput.Placeholder = "Type project name (min 2 chars)..."
	projectSearchInput.CharLimit = 100
	projectSearchInput.Width = 50
	projectSearchInput.Focus() // Focus the input so it accepts typing

	// Initialize timer form inputs
	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "Description..."
	descriptionInput.CharLimit = 200
	descriptionInput.Width = 60

	dateInput := textinput.New()
	dateInput.Placeholder = "YYYY-MM-DD"
	dateInput.CharLimit = 10
	dateInput.Width = 15
	dateInput.SetValue(time.Now().Format("2006-01-02"))

	startTimeInput := textinput.New()
	startTimeInput.Placeholder = "HH:MM"
	startTimeInput.CharLimit = 5
	startTimeInput.Width = 10

	endTimeInput := textinput.New()
	endTimeInput.Placeholder = "HH:MM"
	endTimeInput.CharLimit = 5
	endTimeInput.Width = 10

	app := &TimesheetApp{
		service:               service,
		apiClient:             apiClient,
		username:              username,
		width:                 80,  // Default width
		height:                24,  // Default height
		currentTab:            TabProjects,
		projectSearchInput:    projectSearchInput,
		searchMode:            true, // Start in search mode on Tab 1
		descriptionInput:      descriptionInput,
		dateInput:             dateInput,
		startTimeInput:        startTimeInput,
		endTimeInput:          endTimeInput,
		historyPageSize:       30,
		historyPage:           0,
		projectCache:          make(map[string][]api.ProjectListItem), // Initialize project cache
		projectSearchInFlight: make(map[string]bool),                  // Initialize in-flight tracker
		taskCache:             make(map[int][]api.TaskListItem),       // Initialize task cache
		taskCacheInFlight:     make(map[int]bool),                     // Initialize task in-flight tracker
		activityCacheValid:    false,                                  // Activities not yet cached
		activityCacheInFlight: false,                                  // No activity request in flight
		cachedStatus:          "Idle",                                 // Initialize status cache
	}

	// Load session state to restore previous selections
	app.loadSessionState()

	return app, nil
}

// NewTimesheetAppWithTab creates a new timesheet app starting on a specific tab
func NewTimesheetAppWithTab(initialTab Tab) (*TimesheetApp, error) {
	app, err := NewTimesheetApp()
	if err != nil {
		return nil, err
	}

	// Override the initial tab
	app.currentTab = initialTab

	return app, nil
}

// NewTimesheetAppWithClientAndTab creates a new timesheet app with a provided API client and initial tab
func NewTimesheetAppWithClientAndTab(apiClient *api.Client, username string, initialTab Tab) (*TimesheetApp, error) {
	app, err := NewTimesheetAppWithClient(apiClient, username)
	if err != nil {
		return nil, err
	}

	// Override the initial tab
	app.currentTab = initialTab

	return app, nil
}

func (a *TimesheetApp) Init() tea.Cmd {
	return tea.Batch(
		// Don't load anything initially - wait for user interaction
		// Activities will be loaded when user selects a task
		// History will be loaded when user switches to history tab
		a.loadActiveEntry(),
		a.fetchStatusAsync(), // Fetch initial status asynchronously
		textinput.Blink,
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

func (a *TimesheetApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case tea.KeyMsg:
		// Handle Tab 1 search mode separately
		if a.currentTab == TabProjects && a.searchMode {
			switch msg.String() {
			case "ctrl+c":
				return a, tea.Quit
			case "esc":
				// If in search mode with no projects, go back to main menu
				if len(a.projects) == 0 {
					return a, func() tea.Msg {
						return backToMenuMsg{}
					}
				}
				// Exit search mode, go to list navigation
				a.searchMode = false
				a.projectSearchInput.Blur()
			case "tab":
				// Switch from search to list navigation
				if len(a.projects) > 0 {
					a.searchMode = false
					a.projectSearchInput.Blur()
				}
			case "enter":
				// If there's exactly one result or cursor on first result, select it
				if len(a.projects) > 0 {
					a.searchMode = false
					a.projectSearchInput.Blur()
				}
			default:
				// Update search input
				var cmd tea.Cmd
				a.projectSearchInput, cmd = a.projectSearchInput.Update(msg)
				cmds = append(cmds, cmd)

				// Trigger search if >= 2 characters
				query := a.projectSearchInput.Value()
				if len(query) >= 2 {
					// Check cache first for immediate response
					cacheKey := strings.ToLower(query)
					if len(cacheKey) > 2 {
						cacheKey = cacheKey[:2]
					}

					if cachedProjects, hasCached := a.projectCache[cacheKey]; hasCached {
						// Use cached data synchronously - no async command needed!
						a.projects = a.filterProjectsLocally(cachedProjects, query)
						a.projectCursor = 0
					} else {
						// Cache miss - need to fetch from API
						// But only if we're not already fetching this cache key
						if !a.projectSearchInFlight[cacheKey] {
							a.projectSearchInFlight[cacheKey] = true
							cmds = append(cmds, a.searchProjects(query))
						}
						// If request is in-flight, do nothing - wait for it to complete
					}
				} else if len(query) == 0 {
					// Clear results if query is empty
					a.projects = nil
					a.projectCursor = 0
				}
			}
			return a, tea.Batch(cmds...)
		}

		// Handle Tab 4 form mode
		if a.currentTab == TabCurrent && a.formMode {
			switch msg.String() {
			case "ctrl+c":
				return a, tea.Quit
			case "esc":
				// Exit form mode
				a.formMode = false
				a.blurAllFormFields()
				return a, nil
			case "tab":
				// Move to next field
				a.blurAllFormFields()
				a.focusedFormField = (a.focusedFormField + 1) % 5
				a.focusFormField()
				return a, nil
			case "shift+tab":
				// Move to previous field
				a.blurAllFormFields()
				a.focusedFormField = (a.focusedFormField - 1 + 5) % 5
				a.focusFormField()
				return a, nil
			case "enter":
				// Submit form if on submit button
				if a.focusedFormField == 4 {
					cmd := a.submitTimeEntry()
					a.formMode = false
					a.blurAllFormFields()
					return a, cmd
				}
				// Move to next field on enter
				a.blurAllFormFields()
				a.focusedFormField = (a.focusedFormField + 1) % 5
				a.focusFormField()
				return a, nil
			default:
				// Update the focused input field
				var cmd tea.Cmd
				switch a.focusedFormField {
				case 0:
					a.descriptionInput, cmd = a.descriptionInput.Update(msg)
				case 1:
					a.dateInput, cmd = a.dateInput.Update(msg)
				case 2:
					a.startTimeInput, cmd = a.startTimeInput.Update(msg)
				case 3:
					a.endTimeInput, cmd = a.endTimeInput.Update(msg)
				}
				return a, cmd
			}
		}

		// Normal navigation mode
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit

		case "ctrl+l":
			// Logout - delete token and quit
			config.DeleteToken()
			return a, tea.Quit

		case "ctrl+r":
			// Clear project cache
			if a.currentTab == TabProjects {
				a.clearProjectCache()
			}

		case "1":
			a.clearMessages()
			a.currentTab = TabProjects
			a.selectedProject = nil
			a.selectedTask = nil
			a.selectedActivity = nil
			a.searchMode = true
			a.projectSearchInput.Focus()
			a.saveSessionState() // Save session state on tab switch
			cmds = append(cmds, textinput.Blink)

		case "2":
			if a.selectedProject != nil {
				a.clearMessages()
				a.currentTab = TabProjectDetails
				a.saveSessionState() // Save session state on tab switch
			}

		case "3":
			if a.selectedTask != nil {
				a.clearMessages()
				a.currentTab = TabActivities
				a.saveSessionState() // Save session state on tab switch
			}

		case "4":
			if a.selectedActivity != nil {
				a.clearMessages()
				a.currentTab = TabCurrent
				a.saveSessionState() // Save session state on tab switch
			}

		case "h", "5":
			// History tab - always available
			a.clearMessages()
			a.currentTab = TabHistory
			a.historyPage = 0
			a.historyCursorPage = 0
			a.saveSessionState() // Save session state on tab switch
			cmds = append(cmds, a.loadAllHistory())

		case "n":
			// New entry (only on Tab 4)
			if a.currentTab == TabCurrent {
				a.clearMessages()
				a.enterNewEntryMode()
				cmds = append(cmds, textinput.Blink)
			}

		case "e":
			// Edit selected entry (only on Tab 4)
			if a.currentTab == TabCurrent && len(a.history) > 0 && !a.history[a.historyCursor].IsSubmitted() {
				a.clearMessages()
				a.enterEditMode()
				cmds = append(cmds, textinput.Blink)
			}

		case "backspace", "esc":
			// Navigate back or enter search mode on Tab 1
			switch a.currentTab {
			case TabProjects:
				if !a.searchMode {
					// Go back to main menu
					return a, func() tea.Msg {
						return backToMenuMsg{}
					}
				}
			case TabProjectDetails:
				a.currentTab = TabProjects
				a.selectedProject = nil
				a.selectedTask = nil
				a.saveSessionState() // Save session state on tab navigation
			case TabActivities:
				a.currentTab = TabProjectDetails
				a.selectedActivity = nil
				a.saveSessionState() // Save session state on tab navigation
			case TabCurrent:
				a.currentTab = TabActivities
				a.saveSessionState() // Save session state on tab navigation
			case TabHistory:
				a.currentTab = TabProjects
				a.saveSessionState() // Save session state on tab navigation
			}

		case "up", "k":
			a.moveCursorUp()

		case "down", "j":
			a.moveCursorDown()

		case "enter", " ":
			cmd := a.handleSelection()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case "/":
			// Enter search mode on Tab 1
			if a.currentTab == TabProjects {
				a.searchMode = true
				a.projectSearchInput.Focus()
				cmds = append(cmds, textinput.Blink)
			}

		case "left", "pgup":
			// Previous page in history tab
			if a.currentTab == TabHistory && a.historyPage > 0 {
				a.historyPage--
				a.historyCursorPage = 0
			}

		case "right", "pgdown":
			// Next page in history tab
			if a.currentTab == TabHistory {
				totalPages := (len(a.allHistory) + a.historyPageSize - 1) / a.historyPageSize
				if a.historyPage < totalPages-1 {
					a.historyPage++
					a.historyCursorPage = 0
				}
			}
		}

	case tickMsg:
		cmds = append(cmds, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}))

		// Update status from API every 5 minutes to avoid blocking renders
		if time.Since(a.lastStatusUpdate) > 5*time.Minute {
			a.lastStatusUpdate = time.Now()
			cmds = append(cmds, a.fetchStatusAsync())
		}

	case statusUpdateMsg:
		// Update cached status from background fetch
		a.cachedStatus = string(msg)

	case activeEntryMsg:
		a.activeEntry = (*models.ActiveTimeEntry)(msg)

	case projectsLoadedMsg:
		// Clear in-flight flag for this cache key
		if msg.CacheKey != "" {
			delete(a.projectSearchInFlight, msg.CacheKey)

			// Cache the full API response (only if we got new data from API)
			if len(msg.AllProjects) > 0 {
				a.projectCache[msg.CacheKey] = msg.AllProjects
			}
		}

		// Update displayed projects with filtered results
		a.projects = msg.Projects
		a.projectCursor = 0

	case tasksLoadedMsg:
		tasks := []api.TaskListItem(msg)

		// Cache the tasks for the selected project
		if a.selectedProject != nil {
			a.taskCache[a.selectedProject.ID] = tasks
			delete(a.taskCacheInFlight, a.selectedProject.ID)
		}

		a.tasks = tasks
		a.taskCursor = 0

	case activitiesLoadedMsg:
		activities := []api.ActivityListItem(msg)

		// Cache activities globally
		a.activityCache = activities
		a.activityCacheValid = true
		a.activityCacheInFlight = false

		a.activities = activities
		// Set cursor to last used activity if available
		if a.lastUsedActivityID > 0 {
			for i, act := range a.activities {
				if act.ID == a.lastUsedActivityID {
					a.activityCursor = i
					break
				}
			}
		}

	case historyLoadedMsg:
		// Check if this is for the history tab (all history) or current tab (filtered history)
		if a.currentTab == TabHistory {
			a.allHistory = []api.TimelogEntry(msg)
			// Already sorted in loadAllHistory
		} else {
			a.history = []api.TimelogEntry(msg)
			// Sort by start time, most recent first
			sort.Slice(a.history, func(i, j int) bool {
				return a.history[i].StartTime().After(a.history[j].StartTime())
			})
		}
		a.historyCursor = 0
		// Exit form mode if it was active (after successful submission)
		a.formMode = false
		a.blurAllFormFields()
		a.saveSessionState() // Save session state after successful submission

		// Invalidate monthly hours cache when history changes
		a.invalidateMonthlyHoursCache()

		// Refresh status since history changed (likely due to timesheet action)
		cmds = append(cmds, a.fetchStatusAsync())

	case timesheetSubmittedMsg:
		a.history = msg.history
		// Sort by start time, most recent first
		sort.Slice(a.history, func(i, j int) bool {
			return a.history[i].StartTime().After(a.history[j].StartTime())
		})
		a.historyCursor = 0
		// Exit form mode
		a.formMode = false
		a.blurAllFormFields()
		a.saveSessionState()

		// Invalidate monthly hours cache when history changes
		a.invalidateMonthlyHoursCache()

		// Refresh status since we just created/updated a timesheet
		cmds = append(cmds, a.fetchStatusAsync())

		// Clear any previous error and set success message based on running status
		a.err = nil
		if msg.isRunning {
			a.successMessage = "✓ Timesheet submitted and timer is running"
		} else {
			a.successMessage = "✓ Timesheet submitted and closed"
			// Clear active entry when timer is stopped
			a.activeEntry = nil
		}

	case runningTimerDetectedMsg:
		// Populate selections from running timer
		a.selectedProject = &msg.project
		a.selectedTask = &msg.task
		a.selectedActivity = &msg.activity

		// Populate data lists
		a.projects = []api.ProjectListItem{msg.project}
		a.tasks = []api.TaskListItem{msg.task}
		a.activities = []api.ActivityListItem{msg.activity}

		// Add the running entry to history
		a.history = []api.TimelogEntry{msg.entry}

		// Switch to Tab 4
		a.currentTab = TabCurrent
		a.searchMode = false

		// Save session state
		a.saveSessionState()

	case projectSearchErrorMsg:
		// Clear in-flight flag on error
		if msg.CacheKey != "" {
			delete(a.projectSearchInFlight, msg.CacheKey)
		}
		a.err = msg.Err

	case errorMsg:
		a.err = error(msg)
	}

	return a, tea.Batch(cmds...)
}

func (a *TimesheetApp) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var content string

	switch a.currentTab {
	case TabProjects:
		content = a.renderProjectsView()
	case TabProjectDetails:
		content = a.renderProjectDetailsView()
	case TabActivities:
		content = a.renderActivitiesView()
	case TabCurrent:
		content = a.renderCurrentView()
	case TabHistory:
		content = a.renderHistoryView()
	}

	userHeader := a.renderUserHeader()
	nav := a.renderNav()

	if a.err != nil {
		errorMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Render(fmt.Sprintf("Error: %v", a.err))
		content = lipgloss.JoinVertical(lipgloss.Top, content, errorMsg)
	}

	if a.successMessage != "" {
		successMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#51CF66")).
			Bold(true).
			Render(a.successMessage)
		content = lipgloss.JoinVertical(lipgloss.Top, content, successMsg)
	}

	return lipgloss.JoinVertical(lipgloss.Top, userHeader, nav, content)
}

func (a *TimesheetApp) renderNav() string {
	activeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#DF9E2F")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1).
		Bold(true)

	inactiveStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("8")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	disabledStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("240")).
		Padding(0, 1)

	var tab1, tab2, tab3, tab4, tab5 string

	// Tab 1: Projects
	if a.currentTab == TabProjects {
		tab1 = activeStyle.Render("1. Projects")
	} else {
		tab1 = inactiveStyle.Render("1. Projects")
	}

	// Tab 2: Project Details (enabled only if project selected)
	if a.selectedProject != nil {
		if a.currentTab == TabProjectDetails {
			tab2 = activeStyle.Render("2. Task")
		} else {
			tab2 = inactiveStyle.Render("2. Task")
		}
	} else {
		tab2 = disabledStyle.Render("2. Task")
	}

	// Tab 3: Activities (enabled only if task selected)
	if a.selectedTask != nil {
		if a.currentTab == TabActivities {
			tab3 = activeStyle.Render("3. Activity")
		} else {
			tab3 = inactiveStyle.Render("3. Activity")
		}
	} else {
		tab3 = disabledStyle.Render("3. Activity")
	}

	// Tab 4: Current (enabled only if activity selected)
	if a.selectedActivity != nil {
		if a.currentTab == TabCurrent {
			tab4 = activeStyle.Render("4. Current")
		} else {
			tab4 = inactiveStyle.Render("4. Current")
		}
	} else {
		tab4 = disabledStyle.Render("4. Current")
	}

	// Tab 5: History (always available with 'h' key)
	if a.currentTab == TabHistory {
		tab5 = activeStyle.Render("5. History [h]")
	} else {
		tab5 = inactiveStyle.Render("5. History [h]")
	}

	status := "● Idle"

	// Check for running entries in history
	var runningEntry *api.TimelogEntry
	for i := range a.history {
		if a.history[i].EndTime().IsZero() {
			runningEntry = &a.history[i]
			break
		}
	}

	// Display running status from history or active entry
	if runningEntry != nil {
		// Skip entries with zero/invalid start times
		if !runningEntry.StartTime().IsZero() {
			elapsed := time.Since(runningEntry.StartTime())
			hours := int(elapsed.Hours())
			minutes := int(elapsed.Minutes()) % 60
			seconds := int(elapsed.Seconds()) % 60
			status = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E95420")).  // Red/orange color for recording
				Bold(true).
				Render(fmt.Sprintf("● Recording %02d:%02d:%02d", hours, minutes, seconds))
		}
	} else if a.activeEntry != nil {
		elapsed := time.Since(a.activeEntry.StartTime)
		hours := int(elapsed.Hours())
		minutes := int(elapsed.Minutes()) % 60
		seconds := int(elapsed.Seconds()) % 60
		status = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E95420")).  // Red/orange color for recording
			Bold(true).
			Render(fmt.Sprintf("● Recording %02d:%02d:%02d", hours, minutes, seconds))
	}

	nav := lipgloss.JoinHorizontal(lipgloss.Top, tab1, " ", tab2, " ", tab3, " ", tab4, " ", tab5)
	return lipgloss.JoinHorizontal(lipgloss.Top, nav, "    ", status)
}

func (a *TimesheetApp) renderUserHeader() string {
	// Get username from stored field (populated during app creation)
	username := a.username
	if username == "" {
		username = "User"
	}

	// Get monthly hours from cache (recalculated only when history changes)
	monthlyHours := a.getMonthlyHours()

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DF9E2F")).
		Bold(true).
		Padding(0, 2).
		Width(a.width)

	// Use cached status (updated periodically in background)
	status := a.cachedStatus

	// Format: User Name | 06 January 2026 | Month total: 10.5h | Status: Idle
	now := time.Now()
	headerText := fmt.Sprintf("%s | %s | Month total: %.1fh | Status: %s",
		username,
		now.Format("02 January 2006"),
		monthlyHours,
		status)

	return headerStyle.Render(headerText)
}

func (a *TimesheetApp) renderProjectsView() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Padding(1, 0)

	title := titleStyle.Render("📁 Projects")

	// Search box
	searchBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DF9E2F")).
		Padding(0, 1).
		Margin(1, 0)

	var searchBoxBorder lipgloss.Color
	if a.searchMode {
		searchBoxBorder = lipgloss.Color("#DF9E2F") // Bright orange when focused
	} else {
		searchBoxBorder = lipgloss.Color("#8A8B8B") // Gray when not focused
	}
	searchBoxStyle = searchBoxStyle.BorderForeground(searchBoxBorder)

	searchBox := searchBoxStyle.Render(a.projectSearchInput.View())

	// Results section
	var resultsContent string

	if len(a.projectSearchInput.Value()) == 0 {
		hintStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
		resultsContent = hintStyle.Render("Start typing to search for projects...")
	} else if len(a.projectSearchInput.Value()) < 2 {
		hintStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
		resultsContent = hintStyle.Render("Type at least 2 characters to search...")
	} else if len(a.projects) == 0 {
		noResults := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("No projects found. Try a different search term.")
		resultsContent = noResults
	} else {
		// Calculate viewport for projects list
		reservedLines := 12
		availableHeight := a.height - reservedLines
		if availableHeight < 5 {
			availableHeight = 5
		}

		// Calculate scroll offset
		scrollOffset := 0
		if a.projectCursor >= availableHeight {
			scrollOffset = a.projectCursor - availableHeight + 1
		}

		// Determine visible range
		visibleStart := scrollOffset
		visibleEnd := scrollOffset + availableHeight
		if visibleEnd > len(a.projects) {
			visibleEnd = len(a.projects)
		}

		// Show results list
		var items []string

		selectedStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("#DF9E2F")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1).
			Bold(true)

		normalStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Padding(0, 1)

		for i := visibleStart; i < visibleEnd; i++ {
			project := a.projects[i]
			prefix := "  "
			if i == a.projectCursor && !a.searchMode {
				prefix = "▶ "
				items = append(items, selectedStyle.Render(prefix+project.Label))
			} else {
				items = append(items, normalStyle.Render(prefix+project.Label))
			}
		}

		listBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#DF9E2F")).
			Padding(1, 2).
			Margin(1, 0)

		listContent := lipgloss.JoinVertical(lipgloss.Left, items...)

		// Add scroll indicator
		if len(a.projects) > availableHeight {
			scrollInfo := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Italic(true).
				Render(fmt.Sprintf("  Showing %d-%d of %d", visibleStart+1, visibleEnd, len(a.projects)))
			resultsContent = scrollInfo + "\n" + listBox.Render(listContent)
		} else {
			resultsContent = listBox.Render(listContent)
		}
	}

	// Help text
	var helpText string
	if a.searchMode {
		helpText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("Tab/Esc: Navigate results | Enter: Select | Ctrl+R: Clear cache | Ctrl+C: Quit")
	} else {
		helpText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("↑/↓: Navigate | Enter: Select | /: Search | Esc: Back to search | Ctrl+R: Clear cache | Ctrl+C: Quit")
	}

	return lipgloss.JoinVertical(lipgloss.Top, title, searchBox, resultsContent, helpText)
}

func (a *TimesheetApp) renderProjectDetailsView() string {
	if a.selectedProject == nil {
		return "No project selected"
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Padding(1, 0)

	title := titleStyle.Render(fmt.Sprintf("📋 %s", a.selectedProject.Label))

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)

	subtitle := subtitleStyle.Render("Project Tasks")

	if len(a.tasks) == 0 {
		noTasks := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("No tasks found for this project")
		return lipgloss.JoinVertical(lipgloss.Top, title, "", subtitle, "", noTasks)
	}

	// Calculate viewport for tasks list
	reservedLines := 12
	availableHeight := a.height - reservedLines
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Calculate scroll offset
	scrollOffset := 0
	if a.taskCursor >= availableHeight {
		scrollOffset = a.taskCursor - availableHeight + 1
	}

	// Determine visible range
	visibleStart := scrollOffset
	visibleEnd := scrollOffset + availableHeight
	if visibleEnd > len(a.tasks) {
		visibleEnd = len(a.tasks)
	}

	var items []string

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#DF9E2F")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	for i := visibleStart; i < visibleEnd; i++ {
		task := a.tasks[i]
		prefix := "  "
		taskInfo := task.Label

		if i == a.taskCursor {
			prefix = "▶ "
			items = append(items, selectedStyle.Render(prefix+taskInfo))
		} else {
			items = append(items, normalStyle.Render(prefix+taskInfo))
		}
	}

	listBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DF9E2F")).
		Padding(1, 2).
		Margin(1, 0)

	list := listBox.Render(lipgloss.JoinVertical(lipgloss.Left, items...))

	// Add scroll indicator
	var scrollInfo string
	if len(a.tasks) > availableHeight {
		scrollInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			Render(fmt.Sprintf("  Showing %d-%d of %d", visibleStart+1, visibleEnd, len(a.tasks)))
	}

	helpText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("↑/↓: Navigate | Enter: Select | Esc: Back | Ctrl+C: Quit")

	if scrollInfo != "" {
		return lipgloss.JoinVertical(lipgloss.Top, title, subtitle, scrollInfo, list, helpText)
	}
	return lipgloss.JoinVertical(lipgloss.Top, title, subtitle, list, helpText)
}

func (a *TimesheetApp) renderActivitiesView() string {
	if a.selectedTask == nil {
		return "No task selected"
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Padding(1, 0)

	title := titleStyle.Render(fmt.Sprintf("⚡ Activities for: %s", a.selectedTask.Label))

	if len(a.activities) == 0 {
		noActivities := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("No activities found. Loading...")
		return lipgloss.JoinVertical(lipgloss.Top, title, "", noActivities)
	}

	// Calculate viewport - how many items can fit on screen
	// Reserve space for: title (3 lines), box borders (4 lines), help text (1 line), margins (4 lines)
	reservedLines := 12
	availableHeight := a.height - reservedLines
	if availableHeight < 5 {
		availableHeight = 5 // Minimum viewport size
	}

	// Calculate scroll offset to keep cursor visible
	scrollOffset := 0
	if a.activityCursor >= availableHeight {
		scrollOffset = a.activityCursor - availableHeight + 1
	}

	// Determine visible range
	visibleStart := scrollOffset
	visibleEnd := scrollOffset + availableHeight
	if visibleEnd > len(a.activities) {
		visibleEnd = len(a.activities)
	}

	var items []string

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#DF9E2F")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1).
		Bold(true)

	lastUsedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#51CF66")).
		Foreground(lipgloss.Color("0")).
		Padding(0, 1).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	// Only render visible items
	for i := visibleStart; i < visibleEnd; i++ {
		activity := a.activities[i]
		prefix := "  "
		label := activity.Label

		// Highlight last used activity
		if activity.ID == a.lastUsedActivityID && i != a.activityCursor {
			label += " (last used)"
		}

		if i == a.activityCursor {
			prefix = "▶ "
			if activity.ID == a.lastUsedActivityID {
				items = append(items, lastUsedStyle.Render(prefix+label+" (last used)"))
			} else {
				items = append(items, selectedStyle.Render(prefix+label))
			}
		} else if activity.ID == a.lastUsedActivityID {
			items = append(items, lastUsedStyle.Render(prefix+label))
		} else {
			items = append(items, normalStyle.Render(prefix+label))
		}
	}

	listBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DF9E2F")).
		Padding(1, 2).
		Margin(1, 0)

	list := listBox.Render(lipgloss.JoinVertical(lipgloss.Left, items...))

	// Add scroll indicator if there are more items
	var scrollInfo string
	if len(a.activities) > availableHeight {
		scrollInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			Render(fmt.Sprintf("  Showing %d-%d of %d", visibleStart+1, visibleEnd, len(a.activities)))
	}

	helpText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("↑/↓: Navigate | Enter: Select | Esc: Back | Ctrl+C: Quit")

	if scrollInfo != "" {
		return lipgloss.JoinVertical(lipgloss.Top, title, scrollInfo, list, helpText)
	}
	return lipgloss.JoinVertical(lipgloss.Top, title, list, helpText)
}

func (a *TimesheetApp) renderCurrentView() string {
	if a.selectedActivity == nil {
		return "No activity selected"
	}

	// Render the timer form
	formView := a.renderTimerForm()

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Padding(1, 0)

	title := titleStyle.Render(fmt.Sprintf("📊 History: %s, %s - %s", a.selectedProject.Label, a.selectedTask.Label, a.selectedActivity.Label))

	// Render the table
	tableView := a.renderEntriesTable()

	return lipgloss.JoinVertical(lipgloss.Top, formView, "", title, "", tableView)
}

func (a *TimesheetApp) renderTimerForm() string {
	formStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DF9E2F")).
		Padding(1, 2).
		Margin(0, 0)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(15)

	focusedLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DF9E2F")).
		Bold(true).
		Width(15)

	// Calculate elapsed time if start time is set
	var elapsedTime string
	if a.startTimeInput.Value() != "" {
		startStr := fmt.Sprintf("%s %s", a.dateInput.Value(), a.startTimeInput.Value())
		startTime, err := time.Parse("2006-01-02 15:04", startStr)
		if err == nil {
			var endTime time.Time
			if a.endTimeInput.Value() != "" {
				endStr := fmt.Sprintf("%s %s", a.dateInput.Value(), a.endTimeInput.Value())
				endTime, err = time.Parse("2006-01-02 15:04", endStr)
				if err != nil {
					endTime = time.Now()
				}
			} else {
				endTime = time.Now()
			}
			elapsed := endTime.Sub(startTime)
			hours := int(elapsed.Hours())
			minutes := int(elapsed.Minutes()) % 60
			elapsedTime = fmt.Sprintf("%dh %dm", hours, minutes)
		}
	}

	// Build form fields
	var formFields []string

	// Description field
	if a.focusedFormField == 0 && a.formMode {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			focusedLabelStyle.Render("Description:"),
			a.descriptionInput.View()))
	} else {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render("Description:"),
			a.descriptionInput.View()))
	}

	// Date field
	if a.focusedFormField == 1 && a.formMode {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			focusedLabelStyle.Render("Date:"),
			a.dateInput.View(),
			"  📅"))
	} else {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render("Date:"),
			a.dateInput.View()))
	}

	// Start time field
	if a.focusedFormField == 2 && a.formMode {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			focusedLabelStyle.Render("Start Time:"),
			a.startTimeInput.View()))
	} else {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render("Start Time:"),
			a.startTimeInput.View()))
	}

	// Elapsed time (read-only)
	formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render("Elapsed:"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render(elapsedTime)))

	// End time field
	if a.focusedFormField == 3 && a.formMode {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			focusedLabelStyle.Render("End Time:"),
			a.endTimeInput.View()))
	} else {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render("End Time:"),
			a.endTimeInput.View()))
	}

	// Submit button
	submitStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("82")).
		Padding(0, 2).
		Margin(0, 15)

	if a.focusedFormField == 4 && a.formMode {
		submitStyle = submitStyle.Bold(true).Background(lipgloss.Color("46"))
		formFields = append(formFields, submitStyle.Render("[ SUBMIT ]"))
	} else {
		formFields = append(formFields, submitStyle.Render("  SUBMIT  "))
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true).
		Margin(1, 0, 0, 0)

	if a.formMode {
		formFields = append(formFields, helpStyle.Render("Tab: Next Field | Shift+Tab: Previous | Enter: Submit | Esc: Exit Form"))
	} else {
		formFields = append(formFields, helpStyle.Render("n: New Entry | e: Edit Selected | ↑/↓: Navigate | Esc: Back"))
	}

	return formStyle.Render(lipgloss.JoinVertical(lipgloss.Left, formFields...))
}

func (a *TimesheetApp) renderEntriesTable() string {
	if len(a.history) == 0 {
		noCurrent := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(2, 0).
			Render("No time entries found for this task and activity")
		return noCurrent
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Width(20).
		Align(lipgloss.Left)

	cellStyle := lipgloss.NewStyle().
		Width(20).
		Align(lipgloss.Left)

	selectedStyle := lipgloss.NewStyle().
		Width(20).
		Align(lipgloss.Left).
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("#DF9E2F"))

	// Table header
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		headerStyle.Render("Date"),
		headerStyle.Render("Start"),
		headerStyle.Render("End"),
		headerStyle.Render("Elapsed"),
		headerStyle.Width(40).Render("Description"),
		headerStyle.Width(10).Render("Action"))

	// Calculate visible window for table rows
	maxTableHeight := 10
	visibleStart := 0
	visibleEnd := len(a.history)

	if len(a.history) > maxTableHeight {
		if a.historyCursor >= maxTableHeight/2 {
			visibleStart = a.historyCursor - maxTableHeight/2
		}
		visibleEnd = visibleStart + maxTableHeight
		if visibleEnd > len(a.history) {
			visibleEnd = len(a.history)
			visibleStart = visibleEnd - maxTableHeight
			if visibleStart < 0 {
				visibleStart = 0
			}
		}
	}

	// Table rows
	var rows []string
	for i := visibleStart; i < visibleEnd; i++ {
		entry := a.history[i]

		// Handle invalid/zero timestamps
		var dateStr, startStr string
		if !entry.StartTime().IsZero() {
			dateStr = entry.StartTime().Format("2006-01-02")
			startStr = entry.StartTime().Format("15:04")
		} else {
			dateStr = "----/--/--"
			startStr = "--:--"
		}

		// Check if entry is running (no end time)
		isRunning := entry.EndTime().IsZero() && !entry.StartTime().IsZero()

		var endStr string
		var elapsedStr string

		if isRunning {
			endStr = "--:--"
			// Calculate elapsed time from start to now (only if valid start time)
			if !entry.StartTime().IsZero() {
				elapsed := time.Since(entry.StartTime())
				hours := int(elapsed.Hours())
				minutes := int(elapsed.Minutes()) % 60
				elapsedStr = fmt.Sprintf("%dh %dm", hours, minutes)
			} else {
				elapsedStr = "--h --m"
			}
		} else {
			if !entry.EndTime().IsZero() {
				endStr = entry.EndTime().Format("15:04")
			} else {
				endStr = "--:--"
			}
			hours := int(entry.Duration())
			minutes := int((entry.Duration() - float64(hours)) * 60)
			elapsedStr = fmt.Sprintf("%dh %dm", hours, minutes)
		}

		descStr := entry.GetDescriptionString()
		if len(descStr) > 38 {
			descStr = descStr[:35] + "..."
		}

		var editAction string
		runningStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E95420")).  // Red/orange for running
			Bold(true)

		stoppedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).  // Gray for stopped
			Italic(true)

		if isRunning {
			editAction = runningStyle.Render("● Running")
		} else if entry.IsSubmitted() {
			editAction = "✓ Submitted"
		} else if !entry.EndTime().IsZero() {
			editAction = stoppedStyle.Render("⏹ Stopped")
		} else {
			editAction = "[Edit]"
		}

		var row string
		if i == a.historyCursor && !a.formMode {
			// For selected row, preserve status color (don't override with selection background)
			actionCell := selectedStyle.Width(10).Render(editAction)
			if isRunning || (!entry.EndTime().IsZero() && !entry.IsSubmitted()) {
				// Keep the status color by not re-styling the already-styled indicator
				actionCell = lipgloss.NewStyle().Width(10).Render(editAction)
			}
			row = lipgloss.JoinHorizontal(lipgloss.Top,
				selectedStyle.Render(dateStr),
				selectedStyle.Render(startStr),
				selectedStyle.Render(endStr),
				selectedStyle.Render(elapsedStr),
				selectedStyle.Width(40).Render(descStr),
				actionCell)
		} else {
			row = lipgloss.JoinHorizontal(lipgloss.Top,
				cellStyle.Render(dateStr),
				cellStyle.Render(startStr),
				cellStyle.Render(endStr),
				cellStyle.Render(elapsedStr),
				cellStyle.Width(40).Render(descStr),
				cellStyle.Width(10).Render(editAction))
		}
		rows = append(rows, row)
	}

	tableContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, rows...)...)

	tableStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DF9E2F")).
		Padding(1, 2).
		Margin(0, 0)

	var total float64
	for _, entry := range a.history {
		total += entry.Duration()
	}

	summaryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#51CF66")).
		Bold(true).
		Margin(1, 0, 0, 0)

	summary := summaryStyle.Render(fmt.Sprintf("Total: %.2fh (%d entries)", total, len(a.history)))

	// Scroll indicator
	var scrollInfo string
	if len(a.history) > maxTableHeight {
		scrollInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			Render(fmt.Sprintf("  Showing %d-%d of %d", visibleStart+1, visibleEnd, len(a.history)))
	}

	result := tableStyle.Render(tableContent)
	if scrollInfo != "" {
		return lipgloss.JoinVertical(lipgloss.Top, result, summary, scrollInfo)
	}
	return lipgloss.JoinVertical(lipgloss.Top, result, summary)
}

func (a *TimesheetApp) renderHistoryView() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Padding(1, 0)

	if len(a.allHistory) == 0 {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(2, 0)

		var msg string
		if a.err != nil {
			msg = fmt.Sprintf("Error loading history: %v", a.err)
		} else {
			msg = "Loading timesheet history from API...\n\nThis may take a moment on first load.\nPress ESC to go back."
		}

		return lipgloss.JoinVertical(lipgloss.Top, titleStyle.Render("📜 Timesheet History"), loadingStyle.Render(msg))
	}

	// Calculate pagination
	totalPages := (len(a.allHistory) + a.historyPageSize - 1) / a.historyPageSize
	if a.historyPage >= totalPages {
		a.historyPage = totalPages - 1
	}
	if a.historyPage < 0 {
		a.historyPage = 0
	}

	startIdx := a.historyPage * a.historyPageSize
	endIdx := startIdx + a.historyPageSize
	if endIdx > len(a.allHistory) {
		endIdx = len(a.allHistory)
	}

	pageEntries := a.allHistory[startIdx:endIdx]

	// Title with pagination info
	title := titleStyle.Render(fmt.Sprintf("📜 Timesheet History - Page %d of %d (%d total entries)", a.historyPage+1, totalPages, len(a.allHistory)))

	// Table styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Width(18).
		Align(lipgloss.Left)

	cellStyle := lipgloss.NewStyle().
		Width(18).
		Align(lipgloss.Left)

	selectedRowStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("#DF9E2F"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Width(110).
		Align(lipgloss.Left)

	selectedDescStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("#DF9E2F")).
		Width(110).
		Align(lipgloss.Left)

	// Table header
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		headerStyle.Render("Project"),
		headerStyle.Render("Activity"),
		headerStyle.Render("Task"),
		headerStyle.Render("Date"),
		headerStyle.Width(12).Render("Hours"),
		headerStyle.Width(14).Render("Status"))

	// Build table rows (2 rows per entry)
	var rows []string
	for i, entry := range pageEntries {
		isSelected := i == a.historyCursorPage

		// Prepare data
		project := entry.ProjectName
		if len(project) > 16 {
			project = project[:13] + "..."
		}

		activity := entry.Activity()
		if len(activity) > 16 {
			activity = activity[:13] + "..."
		}

		task := entry.TaskName
		if len(task) > 16 {
			task = task[:13] + "..."
		}

		dateStr := entry.StartTime().Format("2006-01-02")
		hoursStr := fmt.Sprintf("%.2fh", entry.Duration())

		status := "Pending"
		if entry.IsSubmitted() {
			status = "✓ Submitted"
		}

		// Row 1: Main data
		var row1 string
		if isSelected {
			row1 = lipgloss.JoinHorizontal(lipgloss.Top,
				selectedRowStyle.Width(18).Render(project),
				selectedRowStyle.Width(18).Render(activity),
				selectedRowStyle.Width(18).Render(task),
				selectedRowStyle.Width(18).Render(dateStr),
				selectedRowStyle.Width(12).Render(hoursStr),
				selectedRowStyle.Width(14).Render(status))
		} else {
			row1 = lipgloss.JoinHorizontal(lipgloss.Top,
				cellStyle.Render(project),
				cellStyle.Render(activity),
				cellStyle.Render(task),
				cellStyle.Render(dateStr),
				cellStyle.Width(12).Render(hoursStr),
				cellStyle.Width(14).Render(status))
		}

		// Row 2: Description (rendered from HTML/Markdown)
		description := entry.GetDescriptionString()
		if description == "" {
			description = "(no description)"
		}

		// Render HTML/Markdown to terminal
		renderedDesc := renderDescription(description, 106) // 110 - 4 for padding

		// Apply styling
		var row2 string
		if isSelected {
			row2 = selectedDescStyle.Render("  " + renderedDesc)
		} else {
			row2 = descStyle.Render("  " + renderedDesc)
		}

		rows = append(rows, row1)
		rows = append(rows, row2)

		// Add separator line between entries (except last)
		if i < len(pageEntries)-1 {
			separator := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render("─────────────────────────────────────────────────────────────────────────────────────────────────────────")
			rows = append(rows, separator)
		}
	}

	tableContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{header, ""}, rows...)...)

	tableStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DF9E2F")).
		Padding(1, 2)

	// Navigation help
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true).
		Margin(1, 0)

	help := helpStyle.Render("Navigation: ↑/↓ navigate • left/right or pgup/pgdn change page • esc back")

	return lipgloss.JoinVertical(lipgloss.Top, title, "", tableStyle.Render(tableContent), help)
}

// renderDescription renders HTML/Markdown description to plain terminal text
func renderDescription(html string, width int) string {
	if html == "" {
		return "(no description)"
	}

	// Create a glamour renderer with custom styles for compact display
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		// Fallback to plain text if glamour fails
		return strings.TrimSpace(html)
	}

	// Render the HTML/Markdown
	rendered, err := r.Render(html)
	if err != nil {
		// Fallback to plain text if rendering fails
		return strings.TrimSpace(html)
	}

	// Clean up the rendered output (remove excessive newlines)
	rendered = strings.TrimSpace(rendered)
	// Replace multiple newlines with single newline
	for strings.Contains(rendered, "\n\n") {
		rendered = strings.ReplaceAll(rendered, "\n\n", "\n")
	}

	return rendered
}

// Navigation helpers
func (a *TimesheetApp) moveCursorUp() {
	switch a.currentTab {
	case TabProjects:
		if a.projectCursor > 0 {
			a.projectCursor--
		}
	case TabProjectDetails:
		if a.taskCursor > 0 {
			a.taskCursor--
		}
	case TabActivities:
		if a.activityCursor > 0 {
			a.activityCursor--
		}
	case TabCurrent:
		if a.historyCursor > 0 {
			a.historyCursor--
		}
	case TabHistory:
		if a.historyCursorPage > 0 {
			a.historyCursorPage--
		}
	}
}

func (a *TimesheetApp) moveCursorDown() {
	switch a.currentTab {
	case TabProjects:
		if a.projectCursor < len(a.projects)-1 {
			a.projectCursor++
		}
	case TabProjectDetails:
		if a.taskCursor < len(a.tasks)-1 {
			a.taskCursor++
		}
	case TabActivities:
		if a.activityCursor < len(a.activities)-1 {
			a.activityCursor++
		}
	case TabCurrent:
		if a.historyCursor < len(a.history)-1 {
			a.historyCursor++
		}
	case TabHistory:
		// Calculate max cursor position for current page
		startIdx := a.historyPage * a.historyPageSize
		endIdx := startIdx + a.historyPageSize
		if endIdx > len(a.allHistory) {
			endIdx = len(a.allHistory)
		}
		pageSize := endIdx - startIdx
		if a.historyCursorPage < pageSize-1 {
			a.historyCursorPage++
		}
	}
}

func (a *TimesheetApp) handleSelection() tea.Cmd {
	switch a.currentTab {
	case TabProjects:
		if a.projectCursor < len(a.projects) {
			a.selectedProject = &a.projects[a.projectCursor]
			a.currentTab = TabProjectDetails
			a.taskCursor = 0
			a.selectedTask = nil
			a.selectedActivity = nil
			a.saveSessionState() // Save session state when project is selected
			return a.loadTasks(a.selectedProject.ID)
		}

	case TabProjectDetails:
		if a.taskCursor < len(a.tasks) {
			a.selectedTask = &a.tasks[a.taskCursor]
			a.currentTab = TabActivities
			a.activityCursor = 0
			a.selectedActivity = nil

			// Load activities (will use cache if available)
			cmd := a.loadActivities()

			// Load last used activity from config
			a.loadLastUsedActivity()

			a.saveSessionState() // Save session state when task is selected

			return cmd
		}

	case TabActivities:
		if a.activityCursor < len(a.activities) {
			a.selectedActivity = &a.activities[a.activityCursor]
			a.currentTab = TabCurrent
			a.historyCursor = 0

			// Save as last used activity
			a.saveLastUsedActivity(a.selectedActivity.ID)

			return a.loadCurrent()
		}
	}

	return nil
}

// Data loading commands
func (a *TimesheetApp) searchProjects(query string) tea.Cmd {
	return func() tea.Msg {
		if a.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		// Get cache key (first 2 letters, lowercase)
		cacheKey := strings.ToLower(query)
		if len(cacheKey) > 2 {
			cacheKey = cacheKey[:2]
		}

		// Fetch from API with the 2-letter prefix
		// NOTE: Cache checking is done in Update(), not here!
		// This Cmd runs in a goroutine and should NOT access shared state
		projects, err := a.apiClient.GetProjects(cacheKey)
		if err != nil {
			return projectSearchErrorMsg{
				Err:      err,
				CacheKey: cacheKey,
			}
		}

		// Filter the result for the actual query
		var filtered []api.ProjectListItem
		queryLower := strings.ToLower(query)
		for _, project := range projects {
			if strings.Contains(strings.ToLower(project.Label), queryLower) {
				filtered = append(filtered, project)
			}
		}

		// Return both filtered results AND full API response for caching
		return projectsLoadedMsg{
			Projects:    filtered,
			CacheKey:    cacheKey,
			AllProjects: projects,
			Query:       query,
		}
	}
}

// filterProjectsLocally filters projects locally by query string
func (a *TimesheetApp) filterProjectsLocally(projects []api.ProjectListItem, query string) []api.ProjectListItem {
	if query == "" {
		return projects
	}

	query = strings.ToLower(query)
	var filtered []api.ProjectListItem

	for _, project := range projects {
		if strings.Contains(strings.ToLower(project.Label), query) {
			filtered = append(filtered, project)
		}
	}

	return filtered
}

func (a *TimesheetApp) loadTasks(projectID int) tea.Cmd {
	// Check cache first (in Update function, not in Cmd)
	if cachedTasks, hasCached := a.taskCache[projectID]; hasCached {
		// Return cached tasks immediately
		return func() tea.Msg {
			return tasksLoadedMsg(cachedTasks)
		}
	}

	// Check if request is already in flight
	if a.taskCacheInFlight[projectID] {
		return nil // Don't make duplicate request
	}

	// Mark as in-flight
	a.taskCacheInFlight[projectID] = true

	return func() tea.Msg {
		if a.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		tasks, err := a.apiClient.GetTasks(fmt.Sprintf("%d", projectID))
		if err != nil {
			// Clear in-flight flag on error
			delete(a.taskCacheInFlight, projectID)
			return errorMsg(err)
		}

		// Cache the result (will be done in Update handler)
		return tasksLoadedMsg(tasks)
	}
}

func (a *TimesheetApp) loadActivities() tea.Cmd {
	// Check if activities are already cached
	if a.activityCacheValid {
		// Return cached activities immediately
		return func() tea.Msg {
			return activitiesLoadedMsg(a.activityCache)
		}
	}

	// Check if request is already in flight
	if a.activityCacheInFlight {
		return nil // Don't make duplicate request
	}

	// Mark as in-flight
	a.activityCacheInFlight = true

	return func() tea.Msg {
		if a.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		activities, err := a.apiClient.GetActivities()
		if err != nil {
			// Clear in-flight flag on error
			return errorMsg(err)
		}

		// Cache will be updated in Update handler
		return activitiesLoadedMsg(activities)
	}
}

func (a *TimesheetApp) loadCurrent() tea.Cmd {
	return func() tea.Msg {
		if a.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		// Get all timelogs
		timelogs, err := a.apiClient.GetTimelogs()
		if err != nil {
			return errorMsg(err)
		}

		// Filter by selected task and activity
		var filtered []api.TimelogEntry
		activityName := a.selectedActivity.Label

		for _, entry := range timelogs {
			if entry.TaskName == a.selectedTask.Label && entry.Activity() == activityName {
				filtered = append(filtered, entry)
			}
		}

		return historyLoadedMsg(filtered)
	}
}

func (a *TimesheetApp) loadAllHistory() tea.Cmd {
	return func() tea.Msg {
		if a.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		// Get all timelogs
		timelogs, err := a.apiClient.GetTimelogs()
		if err != nil {
			return errorMsg(err)
		}

		// Sort by start time descending (newest first)
		sort.Slice(timelogs, func(i, j int) bool {
			return timelogs[i].StartTime().After(timelogs[j].StartTime())
		})

		return historyLoadedMsg(timelogs)
	}
}

func (a *TimesheetApp) loadActiveEntry() tea.Cmd {
	return func() tea.Msg {
		// Fallback to local service
		entry, err := a.service.GetActiveTimeEntry()
		if err != nil {
			return errorMsg(err)
		}
		return activeEntryMsg(entry)
	}
}

// getStatusFromAPI checks the API for active timesheet status
// fetchStatusAsync fetches status from API in background and returns a message
func (a *TimesheetApp) fetchStatusAsync() tea.Cmd {
	return func() tea.Msg {
		if a.apiClient == nil {
			// Fallback to local check
			if a.activeEntry != nil {
				return statusUpdateMsg("Active")
			}
			return statusUpdateMsg("Idle")
		}

		// Fetch from API asynchronously
		activeEntry, err := a.apiClient.GetActiveTimesheet()
		if err != nil {
			// On error, fallback to local status
			if a.activeEntry != nil {
				return statusUpdateMsg("Active")
			}
			return statusUpdateMsg("Idle")
		}

		if activeEntry != nil {
			return statusUpdateMsg("Active")
		}
		return statusUpdateMsg("Idle")
	}
}

func (a *TimesheetApp) checkForRunningTimer() tea.Cmd {
	return func() tea.Msg {
		if a.apiClient == nil {
			return nil
		}

		// Get all timelogs
		timelogs, err := a.apiClient.GetTimelogs()
		if err != nil {
			return nil // Silently fail - don't block app startup
		}

		// Find first running timer (entry with zero end time)
		for _, entry := range timelogs {
			if entry.EndTime().IsZero() {
				// Found a running timer - need to load project, task, and activity details
				projects, err := a.apiClient.GetProjects(entry.ProjectName)
				if err != nil || len(projects) == 0 {
					continue
				}

				tasks, err := a.apiClient.GetTasks(fmt.Sprintf("%d", projects[0].ID))
				if err != nil {
					continue
				}

				// Find matching task
				var matchedTask *api.TaskListItem
				for _, task := range tasks {
					if task.Label == entry.TaskName {
						matchedTask = &task
						break
					}
				}
				if matchedTask == nil {
					continue
				}

				// Load activities
				activities, err := a.apiClient.GetActivities()
				if err != nil {
					continue
				}

				// Find matching activity
				var matchedActivity *api.ActivityListItem
				for _, activity := range activities {
					if activity.Label == entry.Activity() {
						matchedActivity = &activity
						break
					}
				}
				if matchedActivity == nil {
					continue
				}

				// Return the running timer with all details
				return runningTimerDetectedMsg{
					entry:    entry,
					project:  projects[0],
					task:     *matchedTask,
					activity: *matchedActivity,
				}
			}
		}

		return nil
	}
}

// SessionState holds the user's last selections for persistence
type SessionState struct {
	LastUsedProjectID  int `json:"last_used_project_id"`
	LastUsedTaskID     int `json:"last_used_task_id"`
	LastUsedActivityID int `json:"last_used_activity_id"`
}

// Session state management
func (a *TimesheetApp) loadSessionState() {
	// Try to load from a preferences file
	homeDir, _ := os.UserHomeDir()
	prefsPath := filepath.Join(homeDir, ".config", "kartoza-timesheets", "session.json")

	data, err := os.ReadFile(prefsPath)
	if err != nil {
		return
	}

	var session SessionState
	if err := json.Unmarshal(data, &session); err != nil {
		return
	}

	a.lastUsedActivityID = session.LastUsedActivityID

	// Note: We can't restore the full selections here because the data hasn't been loaded yet
	// The cursor positions will be set when the data is loaded and we can match IDs
}

func (a *TimesheetApp) saveSessionState() {
	homeDir, _ := os.UserHomeDir()
	prefsDir := filepath.Join(homeDir, ".config", "kartoza-timesheets")
	prefsPath := filepath.Join(prefsDir, "session.json")

	// Create directory if it doesn't exist
	os.MkdirAll(prefsDir, 0755)

	session := SessionState{
		LastUsedActivityID: a.lastUsedActivityID,
	}

	// Save selected project ID if available
	if a.selectedProject != nil {
		session.LastUsedProjectID = a.selectedProject.ID
	}

	// Save selected task ID if available
	if a.selectedTask != nil {
		session.LastUsedTaskID = a.selectedTask.ID
	}

	// Save selected activity ID if available
	if a.selectedActivity != nil {
		session.LastUsedActivityID = a.selectedActivity.ID
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(prefsPath, data, 0644)
}

// loadLastUsedActivity loads the last used activity from session state
func (a *TimesheetApp) loadLastUsedActivity() {
	a.loadSessionState()

	// Set cursor to last used activity
	for i, act := range a.activities {
		if act.ID == a.lastUsedActivityID {
			a.activityCursor = i
			break
		}
	}
}

// saveLastUsedActivity saves the last used activity to session state
func (a *TimesheetApp) saveLastUsedActivity(activityID int) {
	a.lastUsedActivityID = activityID
	a.saveSessionState()
}

// Form mode helpers
func (a *TimesheetApp) enterNewEntryMode() {
	a.formMode = true
	a.editingEntryID = 0
	a.focusedFormField = 0

	// Set form fields with current date/time
	now := time.Now()
	a.descriptionInput.SetValue("")
	a.dateInput.SetValue(now.Format("2006-01-02"))
	a.startTimeInput.SetValue(now.Format("15:04")) // Default start time to now
	a.endTimeInput.SetValue("")

	a.focusFormField()
}

func (a *TimesheetApp) enterEditMode() {
	if len(a.history) == 0 || a.historyCursor >= len(a.history) {
		return
	}

	entry := a.history[a.historyCursor]
	a.formMode = true
	a.editingEntryID = entry.ID
	a.focusedFormField = 0

	// Populate form fields with entry data
	a.descriptionInput.SetValue(entry.GetDescriptionString())
	a.dateInput.SetValue(entry.StartTime().Format("2006-01-02"))
	a.startTimeInput.SetValue(entry.StartTime().Format("15:04"))
	a.endTimeInput.SetValue(entry.EndTime().Format("15:04"))

	a.focusFormField()
}

func (a *TimesheetApp) focusFormField() {
	switch a.focusedFormField {
	case 0:
		a.descriptionInput.Focus()
	case 1:
		a.dateInput.Focus()
	case 2:
		a.startTimeInput.Focus()
	case 3:
		a.endTimeInput.Focus()
	}
}

func (a *TimesheetApp) blurAllFormFields() {
	a.descriptionInput.Blur()
	a.dateInput.Blur()
	a.startTimeInput.Blur()
	a.endTimeInput.Blur()
}

func (a *TimesheetApp) submitTimeEntry() tea.Cmd {
	return func() tea.Msg {
		if a.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		// Parse date and times
		dateStr := a.dateInput.Value()
		startTimeStr := a.startTimeInput.Value()
		endTimeStr := a.endTimeInput.Value()

		if dateStr == "" || startTimeStr == "" {
			return errorMsg(fmt.Errorf("date and start time are required"))
		}

		startStr := fmt.Sprintf("%s %s", dateStr, startTimeStr)
		startTime, err := time.Parse("2006-01-02 15:04", startStr)
		if err != nil {
			return errorMsg(fmt.Errorf("invalid start time format: %w", err))
		}

		var endTimePtr *time.Time
		var duration float64

		if endTimeStr != "" {
			endStr := fmt.Sprintf("%s %s", dateStr, endTimeStr)
			endTime, err := time.Parse("2006-01-02 15:04", endStr)
			if err != nil {
				return errorMsg(fmt.Errorf("invalid end time format: %w", err))
			}
			duration = endTime.Sub(startTime).Hours()

			if duration <= 0 {
				return errorMsg(fmt.Errorf("end time must be after start time"))
			}
			endTimePtr = &endTime
		} else {
			// If no end time, leave it nil - this creates a running timer
			endTimePtr = nil
			duration = 0
		}

		// Create time entry
		entry := models.TimeEntry{
			ProjectID:   fmt.Sprintf("%d", a.selectedProject.ID),
			ActivityID:  fmt.Sprintf("%d", a.selectedActivity.ID),
			Description: a.descriptionInput.Value(),
			StartTime:   startTime,
			EndTime:     endTimePtr,
			Duration:    duration,
		}

		if a.selectedTask != nil {
			taskID := fmt.Sprintf("%d", a.selectedTask.ID)
			entry.TaskID = &taskID
		}

		// Create or update entry
		if a.editingEntryID == 0 {
			err = a.apiClient.CreateTimesheet(entry)
		} else {
			err = a.apiClient.UpdateTimesheet(fmt.Sprintf("%d", a.editingEntryID), entry)
		}

		if err != nil {
			return errorMsg(err)
		}

		// Reload history and exit form mode
		timelogs, err := a.apiClient.GetTimelogs()
		if err != nil {
			return errorMsg(err)
		}

		// Filter by selected task and activity, but always include running entries
		var filtered []api.TimelogEntry
		activityName := a.selectedActivity.Label

		for _, entry := range timelogs {
			// Include if it matches the filter OR if it's a running entry
			if (entry.TaskName == a.selectedTask.Label && entry.Activity() == activityName) ||
				entry.EndTime().IsZero() {
				filtered = append(filtered, entry)
			}
		}

		// Return submission message with running status
		return timesheetSubmittedMsg{
			isRunning: endTimePtr == nil,
			history:   filtered,
		}
	}
}

func (a *TimesheetApp) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// clearMessages clears both error and success messages
func (a *TimesheetApp) clearMessages() {
	a.err = nil
	a.successMessage = ""
}

// clearProjectCache clears the project search cache and in-flight requests
func (a *TimesheetApp) clearProjectCache() {
	a.projectCache = make(map[string][]api.ProjectListItem)
	a.projectSearchInFlight = make(map[string]bool)
	a.successMessage = "✓ Project cache cleared"
}

// clearAllCaches clears all API caches
func (a *TimesheetApp) clearAllCaches() {
	a.projectCache = make(map[string][]api.ProjectListItem)
	a.projectSearchInFlight = make(map[string]bool)
	a.taskCache = make(map[int][]api.TaskListItem)
	a.taskCacheInFlight = make(map[int]bool)
	a.activityCache = nil
	a.activityCacheValid = false
	a.activityCacheInFlight = false
	a.invalidateMonthlyHoursCache()
	a.successMessage = "✓ All caches cleared"
}

// getMonthlyHours returns cached monthly hours, recalculating only if month changed
func (a *TimesheetApp) getMonthlyHours() float64 {
	now := time.Now()
	currentMonth := now.Month()
	currentYear := now.Year()

	// Check if cache is valid for current month
	if a.cachedMonth == currentMonth && a.cachedYear == currentYear {
		return a.cachedMonthlyHours
	}

	// Cache miss or new month - recalculate
	monthStart := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Second)

	var monthlyHours float64
	for _, entry := range a.history {
		// Skip entries with zero/invalid times
		if !entry.StartTime().IsZero() && entry.StartTime().After(monthStart) && entry.StartTime().Before(monthEnd) {
			if !entry.EndTime().IsZero() {
				// Completed entry
				monthlyHours += entry.Duration()
			} else {
				// Running entry - calculate current duration
				monthlyHours += time.Since(entry.StartTime()).Hours()
			}
		}
	}

	// Update cache
	a.cachedMonthlyHours = monthlyHours
	a.cachedMonth = currentMonth
	a.cachedYear = currentYear

	return monthlyHours
}

// invalidateMonthlyHoursCache marks the monthly hours cache as invalid
func (a *TimesheetApp) invalidateMonthlyHoursCache() {
	a.cachedMonth = 0
	a.cachedYear = 0
}
