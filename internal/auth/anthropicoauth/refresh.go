package anthropicoauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

const refreshBuffer = 5 * time.Minute

type refreshTokenRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
	Scope        string `json:"scope,omitempty"`
}

func ShouldRefresh(profile contracts.AuthProfile) bool {
	if profile.Kind != contracts.AuthProfileAnthropicOAuth {
		return false
	}
	expiresAt := strings.TrimSpace(profile.Settings["oauth_expires_at"])
	if expiresAt == "" {
		return false
	}
	unixSeconds, err := strconv.ParseInt(expiresAt, 10, 64)
	if err != nil {
		return false
	}
	return time.Now().UTC().Add(refreshBuffer).Unix() >= unixSeconds
}

func MaybeRefreshProfile(ctx context.Context, profile contracts.AuthProfile, force bool) (contracts.AuthProfile, bool, error) {
	if profile.Kind != contracts.AuthProfileAnthropicOAuth {
		return profile, false, nil
	}
	if !force && !ShouldRefresh(profile) {
		return profile, false, nil
	}

	refreshToken := strings.TrimSpace(profile.Settings["oauth_refresh_token"])
	if refreshToken == "" {
		return profile, false, nil
	}

	opts := resolveOptions(LoginOptions{
		AccountScope: profile.Settings["account_scope"],
		OAuthHost:    profile.Settings["oauth_host"],
		TokenURL:     profile.Settings["oauth_token_url"],
		ClientID:     profile.Settings["oauth_client_id"],
		APIBase:      profile.Settings["api_base"],
		Scopes:       strings.Fields(profile.Settings["oauth_scopes"]),
		HTTPClient:   defaultHTTPClient,
	})

	requestBody := refreshTokenRequest{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
		ClientID:     opts.ClientID,
		Scope:        strings.Join(opts.Scopes, " "),
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return profile, false, fmt.Errorf("encode oauth refresh request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, opts.TokenURL, bytes.NewReader(body))
	if err != nil {
		return profile, false, fmt.Errorf("build oauth refresh request: %w", err)
	}
	req.Header.Set("content-type", "application/json")

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return profile, false, fmt.Errorf("refresh oauth token: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return profile, false, fmt.Errorf("read oauth refresh response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return profile, false, fmt.Errorf("oauth refresh failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var tokens tokenExchangeResponse
	if err := json.Unmarshal(responseBody, &tokens); err != nil {
		return profile, false, fmt.Errorf("decode oauth refresh response: %w", err)
	}
	if strings.TrimSpace(tokens.AccessToken) == "" {
		return profile, false, fmt.Errorf("oauth refresh response did not include an access token")
	}

	next := cloneProfile(profile)
	if next.Settings == nil {
		next.Settings = map[string]string{}
	}
	next.Settings["oauth_access_token"] = tokens.AccessToken
	if strings.TrimSpace(tokens.RefreshToken) != "" {
		next.Settings["oauth_refresh_token"] = tokens.RefreshToken
	}
	if strings.TrimSpace(tokens.Scope) != "" {
		next.Settings["oauth_scopes"] = strings.TrimSpace(tokens.Scope)
	}
	if tokens.ExpiresIn > 0 {
		next.Settings["oauth_expires_at"] = strconv.FormatInt(time.Now().UTC().Add(time.Duration(tokens.ExpiresIn)*time.Second).Unix(), 10)
	}
	if strings.TrimSpace(next.Settings["oauth_token_url"]) == "" {
		next.Settings["oauth_token_url"] = opts.TokenURL
	}
	if strings.TrimSpace(next.Settings["oauth_client_id"]) == "" {
		next.Settings["oauth_client_id"] = opts.ClientID
	}
	if strings.TrimSpace(next.Settings["api_base"]) == "" {
		next.Settings["api_base"] = opts.APIBase
	}
	if strings.TrimSpace(next.Settings["oauth_host"]) == "" {
		next.Settings["oauth_host"] = opts.OAuthHost
	}
	if strings.TrimSpace(next.Settings["account_scope"]) == "" {
		next.Settings["account_scope"] = opts.AccountScope
	}
	return next, true, nil
}

func cloneProfile(profile contracts.AuthProfile) contracts.AuthProfile {
	cloned := profile
	if profile.Settings != nil {
		cloned.Settings = make(map[string]string, len(profile.Settings))
		for key, value := range profile.Settings {
			cloned.Settings[key] = value
		}
	}
	return cloned
}
