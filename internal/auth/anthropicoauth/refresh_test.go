package anthropicoauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestShouldRefreshUsesExpiryBuffer(t *testing.T) {
	profile := contracts.AuthProfile{
		Kind: contracts.AuthProfileAnthropicOAuth,
		Settings: map[string]string{
			"oauth_expires_at": strconv.FormatInt(time.Now().UTC().Add(4*time.Minute).Unix(), 10),
		},
	}
	if !ShouldRefresh(profile) {
		t.Fatalf("expected profile inside refresh buffer to require refresh")
	}

	profile.Settings["oauth_expires_at"] = strconv.FormatInt(time.Now().UTC().Add(10*time.Minute).Unix(), 10)
	if ShouldRefresh(profile) {
		t.Fatalf("expected profile outside refresh buffer not to require refresh")
	}
}

func TestMaybeRefreshProfileRefreshesAnthropicOAuthProfile(t *testing.T) {
	var requestBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/oauth/token" {
			t.Fatalf("expected token path, got %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode refresh request: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"refreshed-access","refresh_token":"refreshed-refresh","expires_in":7200,"scope":"user:profile user:inference"}`))
	}))
	defer server.Close()

	profile := contracts.AuthProfile{
		ID:           "anthropic-main",
		Kind:         contracts.AuthProfileAnthropicOAuth,
		Provider:     contracts.ProviderAnthropic,
		DefaultModel: "claude-sonnet-4-6",
		Settings: map[string]string{
			"oauth_access_token":  "old-access",
			"oauth_refresh_token": "old-refresh",
			"oauth_expires_at":    strconv.FormatInt(time.Now().UTC().Add(-time.Minute).Unix(), 10),
			"oauth_scopes":        "user:profile user:inference",
			"oauth_token_url":     server.URL + "/v1/oauth/token",
			"oauth_client_id":     "client-123",
			"oauth_host":          "https://claude.ai",
			"account_scope":       "claude",
			"api_base":            "https://api.anthropic.com",
		},
	}

	refreshed, changed, err := MaybeRefreshProfile(context.Background(), profile, false)
	if err != nil {
		t.Fatalf("MaybeRefreshProfile returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected profile to be refreshed")
	}
	if requestBody["grant_type"] != "refresh_token" {
		t.Fatalf("expected refresh_token grant, got %#v", requestBody["grant_type"])
	}
	if requestBody["refresh_token"] != "old-refresh" {
		t.Fatalf("expected refresh token old-refresh, got %#v", requestBody["refresh_token"])
	}
	if requestBody["client_id"] != "client-123" {
		t.Fatalf("expected client id client-123, got %#v", requestBody["client_id"])
	}
	if scopes, _ := requestBody["scope"].(string); !strings.Contains(scopes, "user:inference") {
		t.Fatalf("expected scopes in refresh request, got %#v", requestBody["scope"])
	}
	if refreshed.Settings["oauth_access_token"] != "refreshed-access" {
		t.Fatalf("expected refreshed access token, got %q", refreshed.Settings["oauth_access_token"])
	}
	if refreshed.Settings["oauth_refresh_token"] != "refreshed-refresh" {
		t.Fatalf("expected refreshed refresh token, got %q", refreshed.Settings["oauth_refresh_token"])
	}
}

func TestMaybeRefreshProfileSkipsWhenRefreshNotNeeded(t *testing.T) {
	profile := contracts.AuthProfile{
		Kind: contracts.AuthProfileAnthropicOAuth,
		Settings: map[string]string{
			"oauth_access_token":  "current-access",
			"oauth_refresh_token": "current-refresh",
			"oauth_expires_at":    strconv.FormatInt(time.Now().UTC().Add(30*time.Minute).Unix(), 10),
		},
	}
	refreshed, changed, err := MaybeRefreshProfile(context.Background(), profile, false)
	if err != nil {
		t.Fatalf("MaybeRefreshProfile returned error: %v", err)
	}
	if changed {
		t.Fatalf("expected no refresh when profile is still fresh")
	}
	if refreshed.Settings["oauth_access_token"] != "current-access" {
		t.Fatalf("expected access token to remain unchanged, got %q", refreshed.Settings["oauth_access_token"])
	}
}
