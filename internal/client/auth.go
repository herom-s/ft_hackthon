package client

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/ft_hackthon/internal/config"
	"golang.org/x/term"
)

// AuthManager handles authentication flows for the CLI
type AuthManager struct {
	apiClient *APIClient
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(apiClient *APIClient) *AuthManager {
	return &AuthManager{
		apiClient: apiClient,
	}
}

// PromptUsername prompts the user for a username
func PromptUsername() (string, error) {
	fmt.Print("Username: ")
	var username string
	_, err := fmt.Scanln(&username)
	if err != nil {
		return "", fmt.Errorf("failed to read username: %w", err)
	}
	return strings.TrimSpace(username), nil
}

// PromptPassword prompts the user for a password with masking
func PromptPassword() (string, error) {
	fmt.Print("Password: ")
	bytepwd, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Println() // New line after password input
	return string(bytepwd), nil
}

// credentials prompts in the interactive shell; in CI there is no one to
// answer, so they come from FT_HACKTHON_USERNAME / FT_HACKTHON_PASSWORD.
func credentials() (string, string, error) {
	if !Interactive {
		username, password := os.Getenv("FT_HACKTHON_USERNAME"), os.Getenv("FT_HACKTHON_PASSWORD")
		if username == "" || password == "" {
			return "", "", fmt.Errorf("CI mode reads credentials from FT_HACKTHON_USERNAME and FT_HACKTHON_PASSWORD; set both")
		}
		return username, password, nil
	}

	username, err := PromptUsername()
	if err != nil {
		return "", "", err
	}
	if username == "" {
		return "", "", fmt.Errorf("username cannot be empty")
	}

	password, err := PromptPassword()
	if err != nil {
		return "", "", err
	}
	if password == "" {
		return "", "", fmt.Errorf("password cannot be empty")
	}
	return username, password, nil
}

// Login performs the login flow
// It prompts for username and password, then authenticates with the API
func (am *AuthManager) Login() (*LoginResponse, error) {
	username, password, err := credentials()
	if err != nil {
		return nil, err
	}

	// Call the API
	fmt.Println("\nAuthenticating...")
	resp, err := am.apiClient.Login(username, password)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	if err := am.saveSession(resp); err != nil {
		return nil, err
	}

	OKf("Successfully logged in as %s", resp.User)
	OKf("Token saved to ~/.ft_hackthon/config.json")
	return resp, nil
}

// Register performs the registration flow
// Similar to Login but creates a new account
func (am *AuthManager) Register() (*LoginResponse, error) {
	username, password, err := credentials()
	if err != nil {
		return nil, err
	}

	// For now, we'll assume the API has a /auth/register endpoint
	fmt.Println("\nRegistering...")
	payload := map[string]string{
		"username": username,
		"password": password,
	}

	var resp LoginResponse
	httpResp, err := am.apiClient.client.R().
		SetBody(payload).
		SetResult(&resp).
		Post("/auth/register")

	if err != nil {
		return nil, fmt.Errorf("registration request failed: %w", err)
	}

	if httpResp.StatusCode() != 201 && httpResp.StatusCode() != 200 {
		return nil, fmt.Errorf("registration failed: status %d - %s", httpResp.StatusCode(), string(httpResp.Body()))
	}

	am.apiClient.SetToken(resp.Token)

	if err := am.saveSession(&resp); err != nil {
		return nil, err
	}

	OKf("Successfully registered as %s", resp.User)
	OKf("Token saved to ~/.ft_hackthon/config.json")
	return &resp, nil
}

// saveSession stores the token and Gitea info together with the server
// settings that just worked, so later sessions reuse them without flags.
func (am *AuthManager) saveSession(resp *LoginResponse) error {
	cfg := &config.Config{
		Token:         resp.Token,
		User:          resp.User,
		GiteaCloneURL: resp.GiteaCloneURL,
		GiteaToken:    resp.GiteaToken,
		APIURL:        am.apiClient.baseURL,
		Insecure:      am.apiClient.insecure,
	}
	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// Logout clears the stored token
func (am *AuthManager) Logout() error {
	if err := config.ClearToken(); err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}
	OKf("Successfully logged out")
	return nil
}

// IsAuthenticated checks if the user is currently authenticated
func (am *AuthManager) IsAuthenticated() (bool, error) {
	return config.IsAuthenticated()
}

// GetCurrentUser returns the current authenticated user
func (am *AuthManager) GetCurrentUser() (string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", err
	}
	return cfg.User, nil
}
