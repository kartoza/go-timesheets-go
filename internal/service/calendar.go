package service

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/models"
)

// CalendarService provides calendar operations shared between TUI and GUI
type CalendarService struct {
	client *api.GoogleCalendarClient
}

// NewCalendarService creates a new calendar service
// Loads stored credentials if available
func NewCalendarService() (*CalendarService, error) {
	// Load stored configuration
	cfg, err := config.LoadGoogleCalendarConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load calendar config: %w", err)
	}

	// No config exists yet
	if cfg == nil {
		return &CalendarService{}, nil
	}

	// Config exists but no client ID/secret
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return &CalendarService{}, nil
	}

	// Create client with stored credentials
	client := api.NewGoogleCalendarClient(cfg.ClientID, cfg.ClientSecret)

	// If we have tokens, set them
	if cfg.RefreshToken != "" {
		var expiry time.Time
		if cfg.TokenExpiry != "" {
			expiry, _ = time.Parse(time.RFC3339, cfg.TokenExpiry)
		}
		token := &api.GoogleCalendarToken{
			AccessToken:  cfg.AccessToken,
			RefreshToken: cfg.RefreshToken,
			TokenType:    "Bearer",
			Expiry:       expiry,
		}
		client.SetToken(token)
	}

	return &CalendarService{client: client}, nil
}

// NewCalendarServiceWithCredentials creates a calendar service with specific credentials
func NewCalendarServiceWithCredentials(clientID, clientSecret string) *CalendarService {
	client := api.NewGoogleCalendarClient(clientID, clientSecret)
	return &CalendarService{client: client}
}

// IsConnected checks if Google Calendar is authenticated
func (s *CalendarService) IsConnected() bool {
	if s.client == nil {
		return false
	}
	return s.client.IsAuthenticated()
}

// HasCredentials checks if OAuth credentials are configured
func (s *CalendarService) HasCredentials() bool {
	cfg, err := config.LoadGoogleCalendarConfig()
	if err != nil || cfg == nil {
		return false
	}
	return cfg.ClientID != "" && cfg.ClientSecret != ""
}

// GetEventsForDay returns calendar events for a specific day
func (s *CalendarService) GetEventsForDay(ctx context.Context, date time.Time) ([]models.CalendarEvent, error) {
	if s.client == nil {
		return nil, fmt.Errorf("calendar not configured")
	}
	if !s.client.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated with Google Calendar")
	}

	events, err := s.client.GetEventsForDay(ctx, date)
	if err != nil {
		return nil, err
	}

	// Filter out all-day events for timesheet purposes
	filtered := make([]models.CalendarEvent, 0, len(events))
	for _, event := range events {
		if !event.AllDay {
			filtered = append(filtered, event)
		}
	}

	return filtered, nil
}

// GetEventsForDayIncludingAllDay returns all calendar events including all-day events
func (s *CalendarService) GetEventsForDayIncludingAllDay(ctx context.Context, date time.Time) ([]models.CalendarEvent, error) {
	if s.client == nil {
		return nil, fmt.Errorf("calendar not configured")
	}
	if !s.client.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated with Google Calendar")
	}

	return s.client.GetEventsForDay(ctx, date)
}

// StartAuthFlow initiates the OAuth2 authentication flow
// Returns the auth URL to open in browser and starts the callback server
func (s *CalendarService) StartAuthFlow(ctx context.Context, clientID, clientSecret string) (authURL string, codeChan <-chan string, errChan <-chan error, err error) {
	// Create client with provided credentials
	s.client = api.NewGoogleCalendarClient(clientID, clientSecret)

	// Start local callback server
	codeChan, errChan, err = s.client.StartLocalAuthServer(ctx)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to start auth server: %w", err)
	}

	// Generate auth URL
	authURL = s.client.GetAuthURL("kartoza-timesheet")

	return authURL, codeChan, errChan, nil
}

// CompleteAuthFlow exchanges the authorization code for tokens and saves them
func (s *CalendarService) CompleteAuthFlow(ctx context.Context, code, clientID, clientSecret string) error {
	if s.client == nil {
		s.client = api.NewGoogleCalendarClient(clientID, clientSecret)
	}

	// Exchange code for tokens
	token, err := s.client.ExchangeCode(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	// Save configuration with tokens
	cfg := &config.GoogleCalendarConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenExpiry:  token.Expiry.Format(time.RFC3339),
	}

	if err := config.SaveGoogleCalendarConfig(cfg); err != nil {
		return fmt.Errorf("failed to save calendar config: %w", err)
	}

	return nil
}

// SaveCredentials saves OAuth client credentials without tokens
func (s *CalendarService) SaveCredentials(clientID, clientSecret string) error {
	// Load existing config to preserve tokens if any
	cfg, _ := config.LoadGoogleCalendarConfig()
	if cfg == nil {
		cfg = &config.GoogleCalendarConfig{}
	}

	cfg.ClientID = clientID
	cfg.ClientSecret = clientSecret

	if err := config.SaveGoogleCalendarConfig(cfg); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	// Update client
	s.client = api.NewGoogleCalendarClient(clientID, clientSecret)

	return nil
}

// Disconnect removes all stored calendar credentials and tokens
func (s *CalendarService) Disconnect() error {
	s.client = nil
	return config.DeleteGoogleCalendarConfig()
}

// RefreshTokenIfNeeded refreshes the access token if it's expired
func (s *CalendarService) RefreshTokenIfNeeded(ctx context.Context) error {
	if s.client == nil {
		return fmt.Errorf("calendar not configured")
	}

	token := s.client.GetToken()
	if token == nil {
		return fmt.Errorf("no token available")
	}

	if !token.IsExpired() {
		return nil
	}

	// Refresh token
	if err := s.client.RefreshToken(ctx); err != nil {
		return err
	}

	// Save updated token
	cfg, err := config.LoadGoogleCalendarConfig()
	if err != nil || cfg == nil {
		return fmt.Errorf("failed to load config for token update")
	}

	newToken := s.client.GetToken()
	cfg.AccessToken = newToken.AccessToken
	cfg.TokenExpiry = newToken.Expiry.Format(time.RFC3339)

	return config.SaveGoogleCalendarConfig(cfg)
}

// OpenBrowser opens the given URL in the system's default browser
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
