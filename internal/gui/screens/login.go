package screens

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/gui/util"
)

// LoginScreen represents the login screen
type LoginScreen struct {
	Container fyne.CanvasObject

	window fyne.Window

	// Form widgets
	urlEntry      *widget.Entry
	usernameEntry *widget.Entry
	passwordEntry *widget.Entry
	loginButton   *widget.Button
	errorLabel    *canvas.Text
	loadingBar    *widget.ProgressBarInfinite

	// Callbacks
	OnLogin func(*api.Client, string)
}

// NewLoginScreen creates a new login screen
func NewLoginScreen(window fyne.Window) *LoginScreen {
	s := &LoginScreen{
		window: window,
	}
	s.build()
	return s
}

func (s *LoginScreen) build() {
	// Title
	title := canvas.NewText("Kartoza Timesheets", color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	title.TextSize = 28
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	subtitle := canvas.NewText("Sign in to continue", color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA0, A: 0xFF})
	subtitle.TextSize = 14
	subtitle.Alignment = fyne.TextAlignCenter

	// Form fields
	s.urlEntry = widget.NewEntry()
	s.urlEntry.SetPlaceHolder("https://timesheets.kartoza.com")
	s.urlEntry.SetText("https://timesheets.kartoza.com")

	s.usernameEntry = widget.NewEntry()
	s.usernameEntry.SetPlaceHolder("Username")

	s.passwordEntry = widget.NewPasswordEntry()
	s.passwordEntry.SetPlaceHolder("Password")
	s.passwordEntry.OnSubmitted = func(text string) {
		s.attemptLogin()
	}

	// Error label
	s.errorLabel = canvas.NewText("", color.NRGBA{R: 0xE7, G: 0x4C, B: 0x3C, A: 0xFF})
	s.errorLabel.TextSize = 12
	s.errorLabel.Alignment = fyne.TextAlignCenter

	// Loading indicator
	s.loadingBar = widget.NewProgressBarInfinite()
	s.loadingBar.Hide()

	// Login button
	s.loginButton = widget.NewButton("Sign In", s.attemptLogin)
	s.loginButton.Importance = widget.HighImportance

	// Form layout
	form := container.NewVBox(
		container.NewCenter(title),
		container.NewCenter(subtitle),
		widget.NewSeparator(),
		layout.NewSpacer(),
		widget.NewLabel("Server URL"),
		s.urlEntry,
		widget.NewLabel("Username"),
		s.usernameEntry,
		widget.NewLabel("Password"),
		s.passwordEntry,
		layout.NewSpacer(),
		s.errorLabel,
		s.loadingBar,
		container.NewCenter(s.loginButton),
		layout.NewSpacer(),
	)

	// Container with border
	formBorder := canvas.NewRectangle(color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF})
	formBorder.StrokeColor = color.NRGBA{R: 0xDD, G: 0xA0, B: 0x36, A: 0xFF}
	formBorder.StrokeWidth = 2
	formBorder.FillColor = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}
	formBorder.CornerRadius = 10

	formContainer := container.NewStack(
		formBorder,
		container.NewPadded(container.NewPadded(form)),
	)

	s.Container = container.NewCenter(formContainer)
}

func (s *LoginScreen) attemptLogin() {
	url := s.urlEntry.Text
	username := s.usernameEntry.Text
	password := s.passwordEntry.Text

	if url == "" || username == "" || password == "" {
		s.showError("Please fill in all fields")
		return
	}

	s.setLoading(true)
	s.errorLabel.Text = ""
	s.errorLabel.Refresh()

	util.RunAsync(
		func() (*api.Client, error) {
			// Create API client
			apiConfig := api.Config{
				BaseURL:  url,
				Username: username,
				Password: password,
				Timeout:  30,
			}
			client, err := api.NewClient(apiConfig)
			if err != nil {
				return nil, err
			}

			// Attempt login
			loginResp, err := client.Login(username, password)
			if err != nil {
				return nil, err
			}

			// Save token
			if err := config.SaveToken(loginResp.Token, username, url); err != nil {
				// Non-fatal, just log
			}

			// Set auth token on client
			client.SetAuthToken(loginResp.Token)
			client.SetUsername(username)

			return client, nil
		},
		func(client *api.Client, err error) {
			s.setLoading(false)
			if err != nil {
				s.showError("Login failed: " + err.Error())
				return
			}
			if s.OnLogin != nil {
				s.OnLogin(client, s.usernameEntry.Text)
			}
		},
	)
}

func (s *LoginScreen) showError(msg string) {
	s.errorLabel.Text = msg
	s.errorLabel.Refresh()
}

func (s *LoginScreen) setLoading(loading bool) {
	if loading {
		s.loadingBar.Show()
		s.loginButton.Disable()
	} else {
		s.loadingBar.Hide()
		s.loginButton.Enable()
	}
}

// TryAutoLogin attempts to login with saved credentials
func (s *LoginScreen) TryAutoLogin() {
	savedToken, err := config.LoadToken()
	if err != nil || savedToken == nil || savedToken.Token == "" {
		return
	}

	s.setLoading(true)

	util.RunAsync(
		func() (*api.Client, error) {
			apiConfig := api.Config{
				BaseURL:   savedToken.BaseURL,
				Username:  savedToken.Username,
				AuthToken: savedToken.Token,
				Timeout:   30,
			}
			client, err := api.NewClient(apiConfig)
			if err != nil {
				return nil, err
			}
			client.SetAuthToken(savedToken.Token)
			client.SetUsername(savedToken.Username)

			// Verify token is still valid
			if err := client.HealthCheck(); err != nil {
				return nil, err
			}

			return client, nil
		},
		func(client *api.Client, err error) {
			s.setLoading(false)
			if err != nil {
				// Token invalid, show login form
				return
			}
			if s.OnLogin != nil {
				s.OnLogin(client, savedToken.Username)
			}
		},
	)
}
