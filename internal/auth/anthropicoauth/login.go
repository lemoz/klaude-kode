package anthropicoauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

const (
	DefaultAccountScope = "claude"
	DefaultClientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	DefaultAPIBase      = "https://api.anthropic.com"
	DefaultOAuthHost    = "https://claude.ai"
	DefaultModel        = "claude-sonnet-4-6"
	OAuthBetaHeader     = "oauth-2025-04-20"
)

var defaultScopes = []string{
	"user:profile",
	"user:inference",
	"user:sessions:claude_code",
	"user:mcp_servers",
	"user:file_upload",
	"org:create_api_key",
}

var defaultHTTPClient = &http.Client{Timeout: 15 * time.Second}

type LoginOptions struct {
	ProfileID    string
	DisplayName  string
	DefaultModel string
	AccountScope string
	OAuthHost    string
	AuthorizeURL string
	TokenURL     string
	APIBase      string
	ClientID     string
	Scopes       []string
	OpenBrowser  bool
	BrowserOpener func(string) error
	HTTPClient   *http.Client
	Output       io.Writer
	Timeout      time.Duration
}

type LoginResult struct {
	Profile contracts.AuthProfile
	AuthURL string
}

type tokenExchangeResponse struct {
	AccessToken string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

type tokenExchangeRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	ClientID     string `json:"client_id"`
	CodeVerifier string `json:"code_verifier"`
	State        string `json:"state"`
}

type callbackResult struct {
	Code string
	Err  error
}

func PerformLogin(ctx context.Context, opts LoginOptions) (LoginResult, error) {
	resolved := resolveOptions(opts)

	codeVerifier, err := randomToken(48)
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate pkce verifier: %w", err)
	}
	state, err := randomToken(24)
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate oauth state: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return LoginResult{}, fmt.Errorf("listen for oauth callback: %w", err)
	}
	defer listener.Close()

	callbackPort := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", callbackPort)
	authURL := buildAuthorizeURL(resolved, redirectURI, codeVerifier, state)

	callbacks := make(chan callbackResult, 1)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			if errMessage := strings.TrimSpace(query.Get("error")); errMessage != "" {
				http.Error(w, "Anthropic login failed. Return to Klaude Kode.", http.StatusBadRequest)
				sendCallbackResult(callbacks, callbackResult{Err: fmt.Errorf("oauth callback error: %s", errMessage)})
				return
			}
			if query.Get("state") != state {
				http.Error(w, "OAuth state mismatch. Return to Klaude Kode.", http.StatusBadRequest)
				sendCallbackResult(callbacks, callbackResult{Err: fmt.Errorf("oauth callback state mismatch")})
				return
			}
			code := strings.TrimSpace(query.Get("code"))
			if code == "" {
				http.Error(w, "OAuth callback did not include a code.", http.StatusBadRequest)
				sendCallbackResult(callbacks, callbackResult{Err: fmt.Errorf("oauth callback missing code")})
				return
			}
			w.Header().Set("content-type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, "<html><body><h1>Anthropic login complete</h1><p>Return to Klaude Kode.</p></body></html>")
			sendCallbackResult(callbacks, callbackResult{Code: code})
		}),
	}

	serverErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	if resolved.Output != nil {
		fmt.Fprintf(resolved.Output, "anthropic oauth: open this URL if your browser does not launch:\n%s\n", authURL)
	}

	if resolved.OpenBrowser {
		if err := resolved.BrowserOpener(authURL); err != nil && resolved.Output != nil {
			fmt.Fprintf(resolved.Output, "anthropic oauth: browser launch failed: %v\n", err)
		}
	}

	waitCtx := ctx
	cancel := func() {}
	if resolved.Timeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, resolved.Timeout)
	}
	defer cancel()

	var callback callbackResult
	select {
	case callback = <-callbacks:
	case err := <-serverErr:
		if err != nil {
			return LoginResult{}, fmt.Errorf("oauth callback server failed: %w", err)
		}
		return LoginResult{}, fmt.Errorf("oauth callback server stopped before receiving a code")
	case <-waitCtx.Done():
		_ = server.Shutdown(context.Background())
		return LoginResult{}, fmt.Errorf("anthropic oauth timed out waiting for browser login")
	}
	_ = server.Shutdown(context.Background())
	<-serverErr
	if callback.Err != nil {
		return LoginResult{}, callback.Err
	}

	tokens, err := exchangeTokens(waitCtx, resolved, callback.Code, redirectURI, codeVerifier, state)
	if err != nil {
		return LoginResult{}, err
	}

	scopeString := strings.TrimSpace(tokens.Scope)
	if scopeString == "" {
		scopeString = strings.Join(resolved.Scopes, " ")
	}
	expiresAt := ""
	if tokens.ExpiresIn > 0 {
		expiresAt = strconv.FormatInt(time.Now().UTC().Add(time.Duration(tokens.ExpiresIn)*time.Second).Unix(), 10)
	}

	profileID := strings.TrimSpace(resolved.ProfileID)
	if profileID == "" {
		profileID = "anthropic-main"
	}
	displayName := strings.TrimSpace(resolved.DisplayName)
	if displayName == "" {
		displayName = "Anthropic Main"
	}
	defaultModel := strings.TrimSpace(resolved.DefaultModel)
	if defaultModel == "" {
		defaultModel = DefaultModel
	}

	return LoginResult{
		AuthURL: authURL,
		Profile: contracts.AuthProfile{
			ID:           profileID,
			Kind:         contracts.AuthProfileAnthropicOAuth,
			Provider:     contracts.ProviderAnthropic,
			DisplayName:  displayName,
			DefaultModel: defaultModel,
			Settings: map[string]string{
				"oauth_host":          resolved.OAuthHost,
				"account_scope":       resolved.AccountScope,
				"oauth_authorize_url": resolved.AuthorizeURL,
				"oauth_token_url":     resolved.TokenURL,
				"oauth_client_id":     resolved.ClientID,
				"oauth_access_token":  tokens.AccessToken,
				"oauth_refresh_token": tokens.RefreshToken,
				"oauth_scopes":        scopeString,
				"oauth_expires_at":    expiresAt,
				"api_base":            resolved.APIBase,
			},
		},
	}, nil
}

func resolveOptions(opts LoginOptions) LoginOptions {
	accountScope := strings.ToLower(strings.TrimSpace(opts.AccountScope))
	if accountScope == "" {
		accountScope = DefaultAccountScope
	}

	authorizeURL := strings.TrimSpace(opts.AuthorizeURL)
	if authorizeURL == "" {
		if env := strings.TrimSpace(os.Getenv("KLAUDE_ANTHROPIC_OAUTH_AUTHORIZE_URL")); env != "" {
			authorizeURL = env
		} else if accountScope == "console" {
			authorizeURL = "https://platform.claude.com/oauth/authorize"
		} else {
			authorizeURL = "https://claude.com/cai/oauth/authorize"
		}
	}

	tokenURL := strings.TrimSpace(opts.TokenURL)
	if tokenURL == "" {
		tokenURL = strings.TrimSpace(os.Getenv("KLAUDE_ANTHROPIC_OAUTH_TOKEN_URL"))
		if tokenURL == "" {
			tokenURL = "https://platform.claude.com/v1/oauth/token"
		}
	}

	apiBase := strings.TrimSpace(opts.APIBase)
	if apiBase == "" {
		apiBase = strings.TrimSpace(os.Getenv("KLAUDE_ANTHROPIC_API_BASE"))
		if apiBase == "" {
			apiBase = DefaultAPIBase
		}
	}

	clientID := strings.TrimSpace(opts.ClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(os.Getenv("KLAUDE_ANTHROPIC_OAUTH_CLIENT_ID"))
		if clientID == "" {
			clientID = DefaultClientID
		}
	}

	oauthHost := strings.TrimSpace(opts.OAuthHost)
	if oauthHost == "" {
		oauthHost = strings.TrimSpace(os.Getenv("KLAUDE_ANTHROPIC_OAUTH_HOST"))
		if oauthHost == "" {
			if accountScope == "console" {
				oauthHost = "https://platform.claude.com"
			} else {
				oauthHost = DefaultOAuthHost
			}
		}
	}

	scopes := append([]string(nil), opts.Scopes...)
	if len(scopes) == 0 {
		if env := strings.TrimSpace(os.Getenv("KLAUDE_ANTHROPIC_OAUTH_SCOPES")); env != "" {
			scopes = strings.Fields(env)
		} else {
			scopes = append([]string(nil), defaultScopes...)
		}
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	openBrowser := opts.OpenBrowser
	if !opts.OpenBrowser {
		openBrowser = false
	}

	browserOpener := opts.BrowserOpener
	if browserOpener == nil {
		browserOpener = OpenBrowser
	}

	return LoginOptions{
		ProfileID:     opts.ProfileID,
		DisplayName:   opts.DisplayName,
		DefaultModel:  opts.DefaultModel,
		AccountScope:  accountScope,
		OAuthHost:     oauthHost,
		AuthorizeURL:  authorizeURL,
		TokenURL:      tokenURL,
		APIBase:       apiBase,
		ClientID:      clientID,
		Scopes:        scopes,
		OpenBrowser:   openBrowser,
		BrowserOpener: browserOpener,
		HTTPClient:    httpClient,
		Output:        opts.Output,
		Timeout:       timeout,
	}
}

func buildAuthorizeURL(opts LoginOptions, redirectURI string, codeVerifier string, state string) string {
	codeChallenge := sha256.Sum256([]byte(codeVerifier))
	challenge := base64.RawURLEncoding.EncodeToString(codeChallenge[:])
	authURL, _ := url.Parse(opts.AuthorizeURL)
	query := authURL.Query()
	query.Set("code", "true")
	query.Set("client_id", opts.ClientID)
	query.Set("response_type", "code")
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(opts.Scopes, " "))
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("state", state)
	authURL.RawQuery = query.Encode()
	return authURL.String()
}

func exchangeTokens(ctx context.Context, opts LoginOptions, code string, redirectURI string, codeVerifier string, state string) (tokenExchangeResponse, error) {
	body, err := json.Marshal(tokenExchangeRequest{
		GrantType:    "authorization_code",
		Code:         code,
		RedirectURI:  redirectURI,
		ClientID:     opts.ClientID,
		CodeVerifier: codeVerifier,
		State:        state,
	})
	if err != nil {
		return tokenExchangeResponse{}, fmt.Errorf("encode oauth token exchange request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, opts.TokenURL, bytes.NewReader(body))
	if err != nil {
		return tokenExchangeResponse{}, fmt.Errorf("build oauth token exchange request: %w", err)
	}
	req.Header.Set("content-type", "application/json")

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return tokenExchangeResponse{}, fmt.Errorf("exchange oauth code for tokens: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return tokenExchangeResponse{}, fmt.Errorf("read oauth token exchange response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tokenExchangeResponse{}, fmt.Errorf("oauth token exchange failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var tokens tokenExchangeResponse
	if err := json.Unmarshal(responseBody, &tokens); err != nil {
		return tokenExchangeResponse{}, fmt.Errorf("decode oauth token exchange response: %w", err)
	}
	if strings.TrimSpace(tokens.AccessToken) == "" {
		return tokenExchangeResponse{}, fmt.Errorf("oauth token exchange did not return an access token")
	}
	return tokens, nil
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func sendCallbackResult(ch chan<- callbackResult, result callbackResult) {
	select {
	case ch <- result:
	default:
	}
}

func OpenBrowser(target string) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("browser target is empty")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
