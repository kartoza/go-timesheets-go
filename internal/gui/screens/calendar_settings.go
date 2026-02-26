package screens

import (
	"context"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/gui/util"
	"github.com/kartoza/go-timesheets-go/internal/service"
)

// CalendarSettingsScreen represents the Google Calendar settings screen
type CalendarSettingsScreen struct {
	Container fyne.CanvasObject

	window          fyne.Window
	calendarService *service.CalendarService

	// Widgets
	clientIDEntry     *widget.Entry
	clientSecretEntry *widget.Entry
	statusLabel       *canvas.Text
	connectBtn        *widget.Button
	disconnectBtn     *widget.Button
	loadingBar        *widget.ProgressBarInfinite

	// Callbacks
	OnBack func()
}

// NewCalendarSettingsScreen creates a new calendar settings screen
func NewCalendarSettingsScreen(window fyne.Window) *CalendarSettingsScreen {
	s := &CalendarSettingsScreen{
		window: window,
	}

	// Initialize calendar service
	s.calendarService, _ = service.NewCalendarService()

	s.build()
	return s
}

func (s *CalendarSettingsScreen) build() {
	goldColor := color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	grayColor := color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
	greenColor := color.NRGBA{R: 0x27, G: 0xAE, B: 0x60, A: 0xFF}
	redColor := color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF}

	// Title
	title := canvas.NewText("Google Calendar Settings", goldColor)
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	// Instructions
	instructions := widget.NewRichTextFromMarkdown(`### Setup Instructions

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing one
3. Enable the **Google Calendar API**
4. Go to **Credentials** → **Create Credentials** → **OAuth client ID**
5. Choose **Desktop application** as application type
6. Copy the **Client ID** and **Client Secret** below`)

	// Client ID entry
	clientIDLabel := canvas.NewText("Client ID", grayColor)
	clientIDLabel.TextSize = 12
	s.clientIDEntry = widget.NewEntry()
	s.clientIDEntry.SetPlaceHolder("Enter your Google OAuth Client ID")

	// Client Secret entry
	clientSecretLabel := canvas.NewText("Client Secret", grayColor)
	clientSecretLabel.TextSize = 12
	s.clientSecretEntry = widget.NewPasswordEntry()
	s.clientSecretEntry.SetPlaceHolder("Enter your Google OAuth Client Secret")

	// Load existing credentials if any
	cfg, _ := config.LoadGoogleCalendarConfig()
	if cfg != nil {
		s.clientIDEntry.SetText(cfg.ClientID)
		s.clientSecretEntry.SetText(cfg.ClientSecret)
	}

	// Status label
	s.statusLabel = canvas.NewText("Not connected", grayColor)
	s.statusLabel.TextSize = 14
	s.statusLabel.Alignment = fyne.TextAlignCenter

	// Update status based on connection
	if s.calendarService != nil && s.calendarService.IsConnected() {
		s.statusLabel.Text = "Connected to Google Calendar"
		s.statusLabel.Color = greenColor
	} else if cfg != nil && cfg.ClientID != "" {
		s.statusLabel.Text = "Credentials saved, not authenticated"
		s.statusLabel.Color = color.NRGBA{R: 0xF3, G: 0x9C, B: 0x12, A: 0xFF} // Orange
	}

	// Loading bar
	s.loadingBar = widget.NewProgressBarInfinite()
	s.loadingBar.Hide()

	// Connect button
	s.connectBtn = widget.NewButton("Connect to Google Calendar", func() {
		s.startOAuthFlow()
	})
	s.connectBtn.Importance = widget.HighImportance

	// Disconnect button
	s.disconnectBtn = widget.NewButton("Disconnect", func() {
		s.disconnect()
	})
	s.disconnectBtn.Importance = widget.DangerImportance
	if s.calendarService == nil || !s.calendarService.IsConnected() {
		s.disconnectBtn.Hide()
	}

	// Save credentials button
	saveBtn := widget.NewButton("Save Credentials", func() {
		s.saveCredentials()
	})

	// Back button
	backBtn := widget.NewButton("← Back to Settings", func() {
		if s.OnBack != nil {
			s.OnBack()
		}
	})

	// Form layout
	form := container.NewVBox(
		clientIDLabel,
		s.clientIDEntry,
		layout.NewSpacer(),
		clientSecretLabel,
		s.clientSecretEntry,
		layout.NewSpacer(),
		container.NewCenter(saveBtn),
	)

	// Status section
	statusSection := container.NewVBox(
		widget.NewSeparator(),
		container.NewCenter(s.statusLabel),
		s.loadingBar,
		container.NewCenter(s.connectBtn),
		container.NewCenter(s.disconnectBtn),
	)

	// Main content
	content := container.NewVBox(
		container.NewCenter(title),
		widget.NewSeparator(),
		instructions,
		widget.NewSeparator(),
		form,
		statusSection,
		layout.NewSpacer(),
		widget.NewSeparator(),
		container.NewCenter(backBtn),
	)

	// Container with border
	border := canvas.NewRectangle(goldColor)
	border.StrokeColor = goldColor
	border.StrokeWidth = 2
	border.FillColor = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}
	border.CornerRadius = 10

	contentContainer := container.NewStack(
		border,
		container.NewPadded(container.NewPadded(content)),
	)

	// Scroll wrapper for smaller screens
	scroll := container.NewVScroll(contentContainer)
	scroll.SetMinSize(fyne.NewSize(400, 500))

	s.Container = container.NewCenter(scroll)

	// Update status colors
	_ = greenColor
	_ = redColor
}

func (s *CalendarSettingsScreen) saveCredentials() {
	clientID := s.clientIDEntry.Text
	clientSecret := s.clientSecretEntry.Text

	if clientID == "" || clientSecret == "" {
		s.statusLabel.Text = "Please enter both Client ID and Client Secret"
		s.statusLabel.Color = color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF}
		s.statusLabel.Refresh()
		return
	}

	if s.calendarService == nil {
		s.calendarService = service.NewCalendarServiceWithCredentials(clientID, clientSecret)
	}

	err := s.calendarService.SaveCredentials(clientID, clientSecret)
	if err != nil {
		s.statusLabel.Text = "Failed to save credentials"
		s.statusLabel.Color = color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF}
		s.statusLabel.Refresh()
		return
	}

	s.statusLabel.Text = "Credentials saved successfully"
	s.statusLabel.Color = color.NRGBA{R: 0x27, G: 0xAE, B: 0x60, A: 0xFF}
	s.statusLabel.Refresh()
}

func (s *CalendarSettingsScreen) startOAuthFlow() {
	clientID := s.clientIDEntry.Text
	clientSecret := s.clientSecretEntry.Text

	if clientID == "" || clientSecret == "" {
		s.statusLabel.Text = "Please enter credentials first"
		s.statusLabel.Color = color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF}
		s.statusLabel.Refresh()
		return
	}

	// Show loading state
	s.loadingBar.Show()
	s.connectBtn.Disable()
	s.statusLabel.Text = "Starting authentication..."
	s.statusLabel.Color = color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
	s.statusLabel.Refresh()

	// Initialize service if needed
	if s.calendarService == nil {
		s.calendarService = service.NewCalendarServiceWithCredentials(clientID, clientSecret)
	}

	// Start OAuth flow
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	util.RunAsync(
		func() (string, error) {
			authURL, codeChan, errChan, err := s.calendarService.StartAuthFlow(ctx, clientID, clientSecret)
			if err != nil {
				cancel()
				return "", err
			}

			// Open browser
			if err := service.OpenBrowser(authURL); err != nil {
				// If browser fails, just show the URL
				fyne.Do(func() {
					s.statusLabel.Text = "Open this URL in browser"
					s.statusLabel.Refresh()
				})
			}

			fyne.Do(func() {
				s.statusLabel.Text = "Waiting for authorization..."
				s.statusLabel.Refresh()
			})

			// Wait for code or error
			select {
			case code := <-codeChan:
				return code, nil
			case err := <-errChan:
				return "", err
			case <-ctx.Done():
				return "", ctx.Err()
			}
		},
		func(code string, err error) {
			defer cancel()

			if err != nil {
				s.loadingBar.Hide()
				s.connectBtn.Enable()
				s.statusLabel.Text = "Authentication failed: " + err.Error()
				s.statusLabel.Color = color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF}
				s.statusLabel.Refresh()
				return
			}

			// Exchange code for tokens
			s.statusLabel.Text = "Completing authentication..."
			s.statusLabel.Refresh()

			util.RunAsync(
				func() (bool, error) {
					return true, s.calendarService.CompleteAuthFlow(context.Background(), code, clientID, clientSecret)
				},
				func(_ bool, err error) {
					s.loadingBar.Hide()
					s.connectBtn.Enable()

					if err != nil {
						s.statusLabel.Text = "Failed to complete auth: " + err.Error()
						s.statusLabel.Color = color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF}
						s.statusLabel.Refresh()
						return
					}

					s.statusLabel.Text = "Connected to Google Calendar"
					s.statusLabel.Color = color.NRGBA{R: 0x27, G: 0xAE, B: 0x60, A: 0xFF}
					s.statusLabel.Refresh()
					s.disconnectBtn.Show()
				},
			)
		},
	)
}

func (s *CalendarSettingsScreen) disconnect() {
	if s.calendarService != nil {
		_ = s.calendarService.Disconnect()
	}

	s.statusLabel.Text = "Disconnected from Google Calendar"
	s.statusLabel.Color = color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
	s.statusLabel.Refresh()
	s.disconnectBtn.Hide()
}

// Refresh updates the screen state
func (s *CalendarSettingsScreen) Refresh() {
	// Reload service state
	s.calendarService, _ = service.NewCalendarService()

	// Update status
	if s.calendarService != nil && s.calendarService.IsConnected() {
		s.statusLabel.Text = "Connected to Google Calendar"
		s.statusLabel.Color = color.NRGBA{R: 0x27, G: 0xAE, B: 0x60, A: 0xFF}
		s.disconnectBtn.Show()
	} else {
		cfg, _ := config.LoadGoogleCalendarConfig()
		if cfg != nil && cfg.ClientID != "" {
			s.statusLabel.Text = "Credentials saved, not authenticated"
			s.statusLabel.Color = color.NRGBA{R: 0xF3, G: 0x9C, B: 0x12, A: 0xFF}
		} else {
			s.statusLabel.Text = "Not connected"
			s.statusLabel.Color = color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF}
		}
		s.disconnectBtn.Hide()
	}
	s.statusLabel.Refresh()
}

// SetCalendarService sets the calendar service
func (s *CalendarSettingsScreen) SetCalendarService(svc *service.CalendarService) {
	s.calendarService = svc
	s.Refresh()
}
