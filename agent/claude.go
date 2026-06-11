package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"promptyly/config"
	"strings"
)

type ClaudeClient struct {
	Config config.ProviderConfig
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	System      string          `json:"system,omitempty"`
	Messages    []claudeMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewClaudeClient(cfg config.ProviderConfig) *ClaudeClient {
	if cfg.Model == "" {
		cfg.Model = "claude-3-5-sonnet-20240620"
	}
	return &ClaudeClient{Config: cfg}
}

func (c *ClaudeClient) Generate(systemPrompt, userPrompt string, onToken func(token string)) (*Response, error) {
	if c.Config.APIKey == "" {
		return nil, fmt.Errorf("Claude API key is not set. Configure it with 'promptyly config set claude_key <key>'")
	}

	url := "https://api.anthropic.com/v1/messages"

	reqBody := claudeRequest{
		Model:     c.Config.Model,
		MaxTokens: 4000,
		System:    systemPrompt,
		Messages: []claudeMessage{
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.2,
		Stream:      onToken != nil,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.Config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var claudeErr claudeResponse
		_ = json.Unmarshal(bodyBytes, &claudeErr)
		if claudeErr.Error != nil {
			return nil, fmt.Errorf("Claude API error (HTTP %d): %s", resp.StatusCode, claudeErr.Error.Message)
		}
		return nil, fmt.Errorf("Claude API HTTP status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if onToken != nil {
		reader := bufio.NewReader(resp.Body)
		var fullText strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "" {
				continue
			}

			var event struct {
				Type  string `json:"type"`
				Delta *struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta,omitempty"`
			}
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Text != "" {
					fullText.WriteString(event.Delta.Text)
					onToken(event.Delta.Text)
				}
			}
		}
		return ParseResponse(fullText.String()), nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(bodyBytes, &claudeResp); err != nil {
		return nil, err
	}

	if len(claudeResp.Content) == 0 || claudeResp.Content[0].Type != "text" {
		return nil, fmt.Errorf("received empty or non-text response from Claude API")
	}

	responseText := claudeResp.Content[0].Text
	return ParseResponse(responseText), nil
}
