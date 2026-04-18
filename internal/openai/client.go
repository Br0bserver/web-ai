package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

type StreamDelta struct {
	ContentDelta string
	ThinkDelta   string
}

func NewClient(baseURL, apiKey string, timeoutSeconds int) *Client {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}
	return &Client{
		httpClient: &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
	}
}

func (c *Client) Chat(ctx context.Context, modelID string, messages []Message) (string, error) {
	requestBody := map[string]any{
		"model":    modelID,
		"messages": messages,
		"stream":   false,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(responseBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("upstream returned no choices")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("upstream returned empty content")
	}
	return content, nil
}

func (c *Client) ChatStream(ctx context.Context, modelID string, messages []Message, onDelta func(StreamDelta) error) (string, string, error) {
	requestBody := map[string]any{
		"model":    modelID,
		"messages": messages,
		"stream":   true,
		"stream_options": map[string]any{
			"include_usage": true,
		},
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", "", readErr
		}
		return "", "", fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(responseBody))
	}

	var contentBuilder strings.Builder
	var thinkBuilder strings.Builder
	reader := bufio.NewReader(resp.Body)
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return "", "", readErr
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				break
			}
			contentDelta, thinkDelta, parseErr := parseStreamPayload(payload)
			if parseErr != nil {
				return "", "", parseErr
			}
			if contentDelta != "" {
				contentBuilder.WriteString(contentDelta)
			}
			if thinkDelta != "" {
				thinkBuilder.WriteString(thinkDelta)
			}
			if onDelta != nil && (contentDelta != "" || thinkDelta != "") {
				if err := onDelta(StreamDelta{ContentDelta: contentDelta, ThinkDelta: thinkDelta}); err != nil {
					return "", "", err
				}
			}
		}
		if readErr == io.EOF {
			break
		}
	}

	content := strings.TrimSpace(contentBuilder.String())
	think := strings.TrimSpace(thinkBuilder.String())
	if content == "" && think == "" {
		return "", "", fmt.Errorf("upstream returned empty content")
	}
	return content, think, nil
}

func parseStreamPayload(payload string) (string, string, error) {
	var parsed struct {
		Choices []struct {
			Delta map[string]any `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return "", "", err
	}
	if len(parsed.Choices) == 0 {
		return "", "", nil
	}
	delta := parsed.Choices[0].Delta
	if delta == nil {
		return "", "", nil
	}
	content := firstString(delta, "content")
	think := firstString(delta, "reasoning_content", "reasoning", "think")
	return content, think, nil
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := payload[key]; ok {
			if value, ok := raw.(string); ok {
				return value
			}
		}
	}
	return ""
}
