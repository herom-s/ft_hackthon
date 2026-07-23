package client

import (
	"fmt"
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

// Login performs the login flow
// It prompts for username and password, then authenticates with the API
func (am *AuthManager) Login() (*LoginResponse, error) {
	username, err := PromptUsername()
	if err != nil {
		return nil, err
	}

	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	password, err := PromptPassword()
	if err != nil {
		return nil, err
	}

	if password == "" {
		return nil, fmt.Errorf("password cannot be empty")
	}

	// Call the API
	fmt.Println("\nAuthenticating...")
	resp, err := am.apiClient.Login(username, password)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Save the token and Gitea info to config
	cfg := &config.Config{
		Token:         resp.Token,
		User:          resp.User,
		GiteaCloneURL: resp.GiteaCloneURL,
		GiteaToken:    resp.GiteaToken,
	}

	if err := config.SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("+ Successfully logged in as %s\n", resp.User)
	fmt.Printf("+ Token saved to ~/.ft_hackthon/config.json\n")
	return resp, nil
}

// Register performs the registration flow
// Similar to Login but creates a new account
func (am *AuthManager) Register() (*LoginResponse, error) {
	username, err := PromptUsername()
	if err != nil {
		return nil, err
	}

	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	password, err := PromptPassword()
	if err != nil {
		return nil, err
	}

	if password == "" {
		return nil, fmt.Errorf("password cannot be empty")
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

	// Save the token and Gitea info to config
	cfg := &config.Config{
		Token:         resp.Token,
		User:          resp.User,
		GiteaCloneURL: resp.GiteaCloneURL,
		GiteaToken:    resp.GiteaToken,
	}

	if err := config.SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("+ Successfully registered as %s\n", resp.User)
	fmt.Printf("+ Token saved to ~/.ft_hackthon/config.json\n")
	return &resp, nil
}

// Logout clears the stored token
func (am *AuthManager) Logout() error {
	if err := config.ClearToken(); err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}
	fmt.Println("+ Successfully logged out")
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
