package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kartoza/go-timesheets-go/internal/models"
)

const (
	// Google OAuth2 endpoints
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"

	// Google Calendar API endpoint
	calendarAPIBase = "https://www.googleapis.com/calendar/v3"

	// OAuth2 scopes - read-only access to calendar
	calendarScope = "https://www.googleapis.com/auth/calendar.readonly"

	// Local callback port for OAuth
	callbackPort = 9876
)

// GoogleCalendarToken represents stored OAuth2 tokens
type GoogleCalendarToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
}

// IsExpired checks if the access token is expired or about to expire
func (t *GoogleCalendarToken) IsExpired() bool {
	// Consider expired if less than 5 minutes remaining
	return time.Now().Add(5 * time.Minute).After(t.Expiry)
}

// GoogleCalendarClient provides methods to interact with Google Calendar API
type GoogleCalendarClient struct {
	httpClient   *http.Client
	clientID     string
	clientSecret string
	token        *GoogleCalendarToken
	redirectURL  string
}

// NewGoogleCalendarClient creates a new Google Calendar API client
func NewGoogleCalendarClient(clientID, clientSecret string) *GoogleCalendarClient {
	return &GoogleCalendarClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  fmt.Sprintf("http://localhost:%d/callback", callbackPort),
	}
}

// SetToken sets the OAuth2 token for API calls
func (c *GoogleCalendarClient) SetToken(token *GoogleCalendarToken) {
	c.token = token
}

// GetToken returns the current token
func (c *GoogleCalendarClient) GetToken() *GoogleCalendarToken {
	return c.token
}

// IsAuthenticated checks if we have valid credentials
func (c *GoogleCalendarClient) IsAuthenticated() bool {
	return c.token != nil && c.token.RefreshToken != ""
}

// GetAuthURL generates the OAuth2 authorization URL
func (c *GoogleCalendarClient) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", c.clientID)
	params.Set("redirect_uri", c.redirectURL)
	params.Set("response_type", "code")
	params.Set("scope", calendarScope)
	params.Set("access_type", "offline") // Request refresh token
	params.Set("prompt", "consent")      // Force consent to get refresh token
	if state != "" {
		params.Set("state", state)
	}

	return googleAuthURL + "?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for access and refresh tokens
func (c *GoogleCalendarClient) ExchangeCode(ctx context.Context, code string) (*GoogleCalendarToken, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)
	data.Set("redirect_uri", c.redirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST", googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	token := &GoogleCalendarToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		Expiry:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}

	c.token = token
	return token, nil
}

// RefreshToken refreshes the access token using the stored refresh token
func (c *GoogleCalendarClient) RefreshToken(ctx context.Context) error {
	if c.token == nil || c.token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	data := url.Values{}
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)
	data.Set("refresh_token", c.token.RefreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token refresh failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode refresh response: %w", err)
	}

	c.token.AccessToken = tokenResp.AccessToken
	c.token.TokenType = tokenResp.TokenType
	c.token.Expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return nil
}

// ensureValidToken checks if token is valid and refreshes if needed
func (c *GoogleCalendarClient) ensureValidToken(ctx context.Context) error {
	if c.token == nil {
		return fmt.Errorf("not authenticated")
	}

	if c.token.IsExpired() {
		if err := c.RefreshToken(ctx); err != nil {
			return fmt.Errorf("failed to refresh expired token: %w", err)
		}
	}

	return nil
}

// GetEventsForDay fetches calendar events for a specific day
func (c *GoogleCalendarClient) GetEventsForDay(ctx context.Context, date time.Time) ([]models.CalendarEvent, error) {
	// Set time range for the entire day in local time
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	endOfDay := startOfDay.Add(24 * time.Hour)

	return c.GetEventsForRange(ctx, startOfDay, endOfDay)
}

// GetEventsForRange fetches calendar events for a time range
func (c *GoogleCalendarClient) GetEventsForRange(ctx context.Context, start, end time.Time) ([]models.CalendarEvent, error) {
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	// Build API URL
	apiURL := fmt.Sprintf("%s/calendars/primary/events", calendarAPIBase)

	params := url.Values{}
	params.Set("timeMin", start.UTC().Format(time.RFC3339))
	params.Set("timeMax", end.UTC().Format(time.RFC3339))
	params.Set("singleEvents", "true") // Expand recurring events
	params.Set("orderBy", "startTime") // Sort by start time
	params.Set("maxResults", "50")     // Limit results

	fullURL := apiURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create events request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token.AccessToken))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("events request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var eventsResp struct {
		Items []struct {
			ID       string `json:"id"`
			Summary  string `json:"summary"`
			Location string `json:"location"`
			Start    struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"` // For all-day events
			} `json:"start"`
			End struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"` // For all-day events
			} `json:"end"`
			ColorID string `json:"colorId"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
		return nil, fmt.Errorf("failed to decode events response: %w", err)
	}

	events := make([]models.CalendarEvent, 0, len(eventsResp.Items))
	for _, item := range eventsResp.Items {
		event := models.CalendarEvent{
			ID:       item.ID,
			Title:    item.Summary,
			Location: item.Location,
			ColorHex: getColorFromID(item.ColorID),
		}

		// Parse start time
		if item.Start.DateTime != "" {
			if t, err := time.Parse(time.RFC3339, item.Start.DateTime); err == nil {
				event.Start = t
			}
		} else if item.Start.Date != "" {
			if t, err := time.Parse("2006-01-02", item.Start.Date); err == nil {
				event.Start = t
				event.AllDay = true
			}
		}

		// Parse end time
		if item.End.DateTime != "" {
			if t, err := time.Parse(time.RFC3339, item.End.DateTime); err == nil {
				event.End = t
			}
		} else if item.End.Date != "" {
			if t, err := time.Parse("2006-01-02", item.End.Date); err == nil {
				event.End = t
			}
		}

		events = append(events, event)
	}

	return events, nil
}

// StartLocalAuthServer starts a local HTTP server for OAuth callback
// Returns a channel that will receive the authorization code
func (c *GoogleCalendarClient) StartLocalAuthServer(ctx context.Context) (<-chan string, <-chan error, error) {
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", callbackPort))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start callback server: %w", err)
	}

	server := &http.Server{}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		errorParam := r.URL.Query().Get("error")

		if errorParam != "" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Authentication Failed</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1 style="color: #E74C3C;">Authentication Failed</h1>
<p>Error: %s</p>
<p>You can close this window.</p>
</body></html>`, errorParam)
			errChan <- fmt.Errorf("OAuth error: %s", errorParam)
		} else if code != "" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Authentication Successful</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1 style="color: #2ECC71;">✓ Authentication Successful</h1>
<p>Go Timesheets Go is now connected to your Google Calendar.</p>
<p>You can close this window and return to the application.</p>
</body></html>`)
			codeChan <- code
		} else {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Error</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
<h1 style="color: #E74C3C;">Error</h1>
<p>No authorization code received.</p>
</body></html>`)
			errChan <- fmt.Errorf("no authorization code received")
		}

		// Shutdown server after handling callback
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = server.Shutdown(context.Background())
		}()
	})

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	// Shutdown on context cancellation
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()

	return codeChan, errChan, nil
}

// getColorFromID maps Google Calendar color IDs to hex colors
func getColorFromID(colorID string) string {
	colors := map[string]string{
		"1":  "#7986CB", // Lavender
		"2":  "#33B679", // Sage
		"3":  "#8E24AA", // Grape
		"4":  "#E67C73", // Flamingo
		"5":  "#F6BF26", // Banana
		"6":  "#F4511E", // Tangerine
		"7":  "#039BE5", // Peacock
		"8":  "#616161", // Graphite
		"9":  "#3F51B5", // Blueberry
		"10": "#0B8043", // Basil
		"11": "#D50000", // Tomato
	}

	if hex, ok := colors[colorID]; ok {
		return hex
	}
	return "#4285F4" // Default Google blue
}
