package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cdossman/klaude-kode/internal/auth/anthropicoauth"
	"github.com/cdossman/klaude-kode/internal/contracts"
)

var defaultHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

type anthropicMessageRequest struct {
	Model     string                     `json:"model"`
	MaxTokens int                        `json:"max_tokens"`
	System    string                     `json:"system,omitempty"`
	Messages  []anthropicMessageEnvelope `json:"messages"`
}

type anthropicMessageEnvelope struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicMessageResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type openRouterCompletionRequest struct {
	Model    string                  `json:"model"`
	Messages []openRouterChatMessage `json:"messages"`
}

type openRouterChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content any `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *staticAdapter) maybeCompleteLive(ctx context.Context, profile contracts.AuthProfile, req contracts.CompletionRequest, model string) (contracts.CompletionResult, bool, error) {
	credential, err := resolveLiveCredential(profile)
	if err != nil {
		if errors.Is(err, ErrCredentialRefUnsupported) {
			return contracts.CompletionResult{}, false, nil
		}
		return contracts.CompletionResult{}, false, err
	}
	if strings.TrimSpace(credential.Value) == "" {
		return contracts.CompletionResult{}, false, nil
	}

	apiBase := strings.TrimSpace(profile.Settings["api_base"])
	if apiBase == "" {
		switch profile.Kind {
		case contracts.AuthProfileAnthropicOAuth:
			apiBase = anthropicoauth.DefaultAPIBase
		default:
			return contracts.CompletionResult{}, false, nil
		}
	}

	switch profile.Kind {
	case contracts.AuthProfileAnthropicOAuth:
		result, err := completeAnthropicLive(ctx, apiBase, credential, req, model)
		return result, true, err
	case contracts.AuthProfileAnthropicAPIKey:
		result, err := completeAnthropicLive(ctx, apiBase, credential, req, model)
		return result, true, err
	case contracts.AuthProfileOpenRouterAPIKey:
		result, err := completeOpenRouterLive(ctx, apiBase, credential.Value, profile, req, model)
		return result, true, err
	default:
		return contracts.CompletionResult{}, false, nil
	}
}

func completeAnthropicLive(ctx context.Context, apiBase string, credential resolvedCredential, req contracts.CompletionRequest, model string) (contracts.CompletionResult, error) {
	systemPrompt := strings.Join(req.SystemPrompt, "\n\n")
	payload := anthropicMessageRequest{
		Model:     model,
		MaxTokens: 1024,
		System:    strings.TrimSpace(systemPrompt),
		Messages:  make([]anthropicMessageEnvelope, 0, len(req.Messages)),
	}
	for _, message := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		payload.Messages = append(payload.Messages, anthropicMessageEnvelope{
			Role:    role,
			Content: message.Content,
		})
	}
	if len(payload.Messages) == 0 {
		payload.Messages = append(payload.Messages, anthropicMessageEnvelope{
			Role:    "user",
			Content: "no user message provided",
		})
	}

	endpoint := strings.TrimRight(apiBase, "/") + "/v1/messages"
	body, err := json.Marshal(payload)
	if err != nil {
		return contracts.CompletionResult{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return contracts.CompletionResult{}, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	switch credential.Mode {
	case credentialModeBearer:
		httpReq.Header.Set("authorization", "Bearer "+credential.Value)
		httpReq.Header.Set("anthropic-beta", anthropicoauth.OAuthBetaHeader)
	default:
		httpReq.Header.Set("x-api-key", credential.Value)
	}

	resp, err := defaultHTTPClient.Do(httpReq)
	if err != nil {
		return contracts.CompletionResult{}, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return contracts.CompletionResult{}, err
	}

	var parsed anthropicMessageResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return contracts.CompletionResult{}, fmt.Errorf("decode anthropic response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return contracts.CompletionResult{}, classifyHTTPError(
			resp.StatusCode,
			anthropicErrorMessage(parsed.Error, string(responseBody), resp.StatusCode),
		)
	}

	text := ""
	for _, item := range parsed.Content {
		if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
			text = strings.TrimSpace(item.Text)
			break
		}
	}
	if text == "" {
		return contracts.CompletionResult{}, fmt.Errorf("anthropic response did not include text content")
	}

	return contracts.CompletionResult{
		Message: contracts.CanonicalMessage{
			Role:    "assistant",
			Content: text,
		},
	}, nil
}

func completeOpenRouterLive(ctx context.Context, apiBase string, apiKey string, profile contracts.AuthProfile, req contracts.CompletionRequest, model string) (contracts.CompletionResult, error) {
	payload := openRouterCompletionRequest{
		Model:    model,
		Messages: make([]openRouterChatMessage, 0, len(req.Messages)),
	}
	for _, message := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role == "" {
			continue
		}
		payload.Messages = append(payload.Messages, openRouterChatMessage{
			Role:    role,
			Content: message.Content,
		})
	}
	if len(payload.Messages) == 0 {
		payload.Messages = append(payload.Messages, openRouterChatMessage{
			Role:    "user",
			Content: "no user message provided",
		})
	}

	endpoint := strings.TrimRight(apiBase, "/") + "/chat/completions"
	body, err := json.Marshal(payload)
	if err != nil {
		return contracts.CompletionResult{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return contracts.CompletionResult{}, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("authorization", "Bearer "+apiKey)
	if referer := strings.TrimSpace(profile.Settings["http_referer"]); referer != "" {
		httpReq.Header.Set("http-referer", referer)
	}
	if title := strings.TrimSpace(profile.Settings["app_name"]); title != "" {
		httpReq.Header.Set("x-title", title)
	}

	resp, err := defaultHTTPClient.Do(httpReq)
	if err != nil {
		return contracts.CompletionResult{}, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return contracts.CompletionResult{}, err
	}

	var parsed openRouterCompletionResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return contracts.CompletionResult{}, fmt.Errorf("decode openrouter response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := string(responseBody)
		if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
			message = parsed.Error.Message
		}
		return contracts.CompletionResult{}, classifyHTTPError(resp.StatusCode, message)
	}
	if len(parsed.Choices) == 0 {
		return contracts.CompletionResult{}, fmt.Errorf("openrouter response did not include choices")
	}

	text := normalizeOpenRouterContent(parsed.Choices[0].Message.Content)
	if text == "" {
		return contracts.CompletionResult{}, fmt.Errorf("openrouter response did not include assistant content")
	}

	return contracts.CompletionResult{
		Message: contracts.CanonicalMessage{
			Role:    "assistant",
			Content: text,
		},
	}, nil
}

func anthropicErrorMessage(errorPayload *struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}, fallback string, statusCode int) string {
	if errorPayload != nil && strings.TrimSpace(errorPayload.Message) != "" {
		return errorPayload.Message
	}
	trimmed := strings.TrimSpace(fallback)
	if trimmed == "" {
		return fmt.Sprintf("status %d", statusCode)
	}
	return trimmed
}

func normalizeOpenRouterContent(content any) string {
	switch value := content.(type) {
	case string:
		return strings.TrimSpace(value)
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if entry, ok := item.(map[string]any); ok {
				text, _ := entry["text"].(string)
				if strings.TrimSpace(text) != "" {
					parts = append(parts, strings.TrimSpace(text))
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func classifyHTTPError(statusCode int, message string) error {
	normalizedMessage := strings.TrimSpace(message)
	if normalizedMessage == "" {
		normalizedMessage = fmt.Sprintf("provider request failed with status %d", statusCode)
	}

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &Error{
			Code:      ErrorCodeAuthFailed,
			Message:   normalizedMessage,
			Retryable: false,
		}
	case http.StatusTooManyRequests:
		return &Error{
			Code:      ErrorCodeProviderRequestFailed,
			Message:   normalizedMessage,
			Retryable: true,
		}
	case http.StatusBadRequest, http.StatusNotFound:
		if looksLikeInvalidModel(normalizedMessage) {
			return &Error{
				Code:      ErrorCodeInvalidModel,
				Message:   normalizedMessage,
				Retryable: false,
			}
		}
	}

	return &Error{
		Code:      ErrorCodeProviderRequestFailed,
		Message:   normalizedMessage,
		Retryable: statusCode >= 500,
	}
}

func looksLikeInvalidModel(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(lower, "model") && (strings.Contains(lower, "invalid") || strings.Contains(lower, "not found") || strings.Contains(lower, "unsupported"))
}
