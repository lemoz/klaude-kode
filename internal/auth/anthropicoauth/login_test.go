package anthropicoauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPerformLoginBuildsAnthropicOAuthProfile(t *testing.T) {
	var tokenRequest map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/authorize":
			redirectURI := r.URL.Query().Get("redirect_uri")
			if redirectURI == "" {
				t.Fatalf("expected redirect_uri query param")
			}
			state := r.URL.Query().Get("state")
			http.Redirect(w, r, redirectURI+"?code=test-code&state="+state, http.StatusFound)
		case "/v1/oauth/token":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST for token exchange, got %s", r.Method)
			}
			if got := r.Header.Get("content-type"); got != "application/json" {
				t.Fatalf("expected application/json content type, got %q", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&tokenRequest); err != nil {
				t.Fatalf("decode token exchange request: %v", err)
			}
			w.Header().Set("content-type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"oauth-access","refresh_token":"oauth-refresh","expires_in":3600,"scope":"user:profile user:inference"}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	var output strings.Builder
	result, err := PerformLogin(context.Background(), LoginOptions{
		ProfileID:     "anthropic-main",
		DisplayName:   "Anthropic Main",
		DefaultModel:  "claude-sonnet-4-6",
		AccountScope:  "claude",
		OAuthHost:     "https://claude.ai",
		AuthorizeURL:  server.URL + "/oauth/authorize",
		TokenURL:      server.URL + "/v1/oauth/token",
		APIBase:       server.URL,
		ClientID:      "client-test",
		OpenBrowser:   true,
		BrowserOpener: func(target string) error { _, err := http.Get(target); return err },
		Output:        &output,
	})
	if err != nil {
		t.Fatalf("PerformLogin returned error: %v", err)
	}

	if tokenRequest["grant_type"] != "authorization_code" {
		t.Fatalf("expected authorization_code grant, got %#v", tokenRequest["grant_type"])
	}
	if tokenRequest["code"] != "test-code" {
		t.Fatalf("expected authorization code test-code, got %#v", tokenRequest["code"])
	}
	if tokenRequest["client_id"] != "client-test" {
		t.Fatalf("expected client-test client_id, got %#v", tokenRequest["client_id"])
	}
	if redirectURI, _ := tokenRequest["redirect_uri"].(string); !strings.HasPrefix(redirectURI, "http://127.0.0.1:") {
		t.Fatalf("expected localhost redirect uri, got %#v", tokenRequest["redirect_uri"])
	}
	if codeVerifier, _ := tokenRequest["code_verifier"].(string); strings.TrimSpace(codeVerifier) == "" {
		t.Fatalf("expected non-empty code verifier")
	}

	if result.Profile.Kind != "anthropic_oauth" {
		t.Fatalf("expected anthropic_oauth profile kind, got %s", result.Profile.Kind)
	}
	if result.Profile.Settings["oauth_access_token"] != "oauth-access" {
		t.Fatalf("expected oauth access token to be saved, got %q", result.Profile.Settings["oauth_access_token"])
	}
	if result.Profile.Settings["oauth_refresh_token"] != "oauth-refresh" {
		t.Fatalf("expected oauth refresh token to be saved, got %q", result.Profile.Settings["oauth_refresh_token"])
	}
	if result.Profile.Settings["api_base"] != server.URL {
		t.Fatalf("expected api_base to match oauth test server, got %q", result.Profile.Settings["api_base"])
	}
	if !strings.Contains(output.String(), "open this URL") {
		t.Fatalf("expected login output to include auth URL guidance, got %q", output.String())
	}
}
