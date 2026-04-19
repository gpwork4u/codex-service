package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	clientID  = "app_EMoamEEZ73f0CkXaXp7hrann"
	issuer    = "https://auth.openai.com"
	tokenURL  = issuer + "/oauth/token"
	refreshMargin = 5 * time.Minute
)

type TokenManager struct {
	mu    sync.RWMutex
	creds *Credentials
	store *Store
}

func NewTokenManager(creds *Credentials, store *Store) *TokenManager {
	return &TokenManager{creds: creds, store: store}
}

func (tm *TokenManager) GetToken() (accessToken, accountID string, err error) {
	tm.mu.RLock()
	if time.Now().Before(tm.creds.ExpiresAt.Add(-refreshMargin)) {
		accessToken = tm.creds.AccessToken
		accountID = tm.creds.AccountID
		tm.mu.RUnlock()
		return
	}
	tm.mu.RUnlock()

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock
	if time.Now().Before(tm.creds.ExpiresAt.Add(-refreshMargin)) {
		return tm.creds.AccessToken, tm.creds.AccountID, nil
	}

	if err := tm.refresh(); err != nil {
		return "", "", fmt.Errorf("token refresh failed: %w", err)
	}
	return tm.creds.AccessToken, tm.creds.AccountID, nil
}

func (tm *TokenManager) refresh() error {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tm.creds.RefreshToken},
		"client_id":     {clientID},
	}

	resp, err := http.PostForm(tokenURL, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("refresh returned %d: %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return err
	}

	expiresIn := tok.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}

	tm.creds.AccessToken = tok.AccessToken
	if tok.RefreshToken != "" {
		tm.creds.RefreshToken = tok.RefreshToken
	}
	if tok.IDToken != "" {
		tm.creds.IDToken = tok.IDToken
		if id := extractAccountID(tok.IDToken); id != "" {
			tm.creds.AccountID = id
		}
	}
	tm.creds.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)

	return tm.store.Save(tm.creds)
}

func (tm *TokenManager) UpdateCredentials(creds *Credentials) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.creds = creds
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// JWT parsing (no signature verification — personal use, trusted issuer)

type jwtClaims struct {
	ChatGPTAccountID string         `json:"chatgpt_account_id"`
	Auth             *authClaim     `json:"https://api.openai.com/auth"`
	Organizations    []organization `json:"organizations"`
}

type authClaim struct {
	ChatGPTAccountID string `json:"chatgpt_account_id"`
}

type organization struct {
	ID string `json:"id"`
}

func extractAccountID(token string) string {
	claims, err := parseJWTClaims(token)
	if err != nil {
		return ""
	}
	if claims.ChatGPTAccountID != "" {
		return claims.ChatGPTAccountID
	}
	if claims.Auth != nil && claims.Auth.ChatGPTAccountID != "" {
		return claims.Auth.ChatGPTAccountID
	}
	if len(claims.Organizations) > 0 && claims.Organizations[0].ID != "" {
		return claims.Organizations[0].ID
	}
	return ""
}

func parseJWTClaims(token string) (*jwtClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT: expected 3 parts, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}
	return &claims, nil
}
