package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	deviceCodeURL = issuer + "/api/accounts/deviceauth/usercode"
	deviceTokenURL = issuer + "/api/accounts/deviceauth/token"
	deviceVerifyURL = "https://auth.openai.com/codex/device"
	pollingMarginMS = 3000
)

type deviceCodeResponse struct {
	DeviceAuthID string          `json:"device_auth_id"`
	UserCode     string          `json:"user_code"`
	Interval     json.RawMessage `json:"interval"`
}

type deviceTokenResponse struct {
	AuthorizationCode string `json:"authorization_code"`
	CodeVerifier      string `json:"code_verifier"`
}

func DeviceCodeLogin() (*Credentials, error) {
	// Step 1: Request device code
	reqBody, _ := json.Marshal(map[string]string{
		"client_id": clientID,
	})

	resp, err := http.Post(deviceCodeURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request returned %d: %s", resp.StatusCode, string(body))
	}

	var dcResp deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcResp); err != nil {
		return nil, fmt.Errorf("failed to decode device code response: %w", err)
	}

	// Step 2: Display instructions
	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("  請到以下網址登入: %s\n", deviceVerifyURL)
	fmt.Printf("  輸入驗證碼: %s\n", dcResp.UserCode)
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("等待授權中...")

	// Step 3: Poll for token
	interval := 5
	if len(dcResp.Interval) > 0 {
		raw := strings.Trim(string(dcResp.Interval), `"`)
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			interval = v
		}
	}
	pollInterval := time.Duration(interval)*time.Second + time.Duration(pollingMarginMS)*time.Millisecond

	pollBody, _ := json.Marshal(map[string]string{
		"device_auth_id": dcResp.DeviceAuthID,
		"user_code":      dcResp.UserCode,
	})

	for {
		time.Sleep(pollInterval)

		pollResp, err := http.Post(deviceTokenURL, "application/json", bytes.NewReader(pollBody))
		if err != nil {
			return nil, fmt.Errorf("polling failed: %w", err)
		}

		if pollResp.StatusCode == http.StatusForbidden || pollResp.StatusCode == http.StatusNotFound {
			pollResp.Body.Close()
			fmt.Print(".")
			continue
		}

		if pollResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(pollResp.Body)
			pollResp.Body.Close()
			return nil, fmt.Errorf("polling returned %d: %s", pollResp.StatusCode, string(body))
		}

		var dtResp deviceTokenResponse
		if err := json.NewDecoder(pollResp.Body).Decode(&dtResp); err != nil {
			pollResp.Body.Close()
			return nil, fmt.Errorf("failed to decode token response: %w", err)
		}
		pollResp.Body.Close()

		fmt.Println("\n授權成功！正在交換 token...")

		// Step 4: Exchange authorization code for tokens
		return exchangeCode(dtResp.AuthorizationCode, dtResp.CodeVerifier)
	}
}

func exchangeCode(code, codeVerifier string) (*Credentials, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"https://auth.openai.com/deviceauth/callback"},
		"client_id":     {clientID},
		"code_verifier": {codeVerifier},
	}

	resp, err := http.PostForm(tokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange returned %d: %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	expiresIn := tok.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}

	accountID := extractAccountID(tok.IDToken)
	if accountID == "" {
		accountID = extractAccountID(tok.AccessToken)
	}

	return &Credentials{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		IDToken:      tok.IDToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		AccountID:    accountID,
	}, nil
}
