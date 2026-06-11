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

type OpenAIClient struct {
	Config      config.ProviderConfig
	ProviderKey string // "ollama", "lmstudio", "openai", "custom"
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewOpenAIClient(cfg config.ProviderConfig, providerKey string) *OpenAIClient {
	// Apply provider-specific defaults if missing
	if cfg.URL == "" {
		if providerKey == "ollama" {
			cfg.URL = "http://localhost:11434"
		} else if providerKey == "lmstudio" {
			cfg.URL = "http://localhost:1234/v1"
		}
	}
	if cfg.Model == "" {
		if providerKey == "ollama" {
			cfg.Model = "llama3"
		} else if providerKey == "lmstudio" {
			cfg.Model = "meta-llama-3-8b-instruct"
		}
	}
	return &OpenAIClient{Config: cfg, ProviderKey: providerKey}
}

func cleanURL(rawURL string) string {
	idx := strings.Index(rawURL, "http://")
	if idx == -1 {
		idx = strings.Index(rawURL, "https://")
	}
	if idx == -1 {
		return rawURL
	}
	cleaned := rawURL[idx:]
	endIdx := len(cleaned)
	for i, r := range cleaned {
		if r == ' ' || r == '\\' || r == '"' || r == '\'' || r == '`' || r == '\n' || r == '\r' || r == '\t' {
			endIdx = i
			break
		}
	}
	return cleaned[:endIdx]
}

func resolveURL(baseURL string) string {
	baseURL = cleanURL(baseURL)
	if strings.HasSuffix(baseURL, "/chat/completions") {
		return baseURL
	}
	if strings.HasSuffix(baseURL, "/chat") || strings.HasSuffix(baseURL, "/chat/") {
		return strings.TrimSuffix(baseURL, "/") + "/completions"
	}
	if strings.HasSuffix(baseURL, "/v1") || strings.HasSuffix(baseURL, "/v1/") {
		return strings.TrimSuffix(baseURL, "/") + "/chat/completions"
	}
	return strings.TrimSuffix(baseURL, "/") + "/v1/chat/completions"
}

func (c *OpenAIClient) Generate(systemPrompt, userPrompt string, onToken func(token string)) (*Response, error) {
	if c.Config.URL == "" {
		return nil, fmt.Errorf("URL endpoint is not configured for %s", c.ProviderKey)
	}

	url := resolveURL(c.Config.URL)

	messages := []openAIMessage{}
	if systemPrompt != "" {
		messages = append(messages, openAIMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, openAIMessage{Role: "user", Content: userPrompt})

	reqBody := openAIRequest{
		Model:       c.Config.Model,
		Messages:    messages,
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

	if c.Config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.Config.APIKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var openAIErr openAIResponse
		_ = json.Unmarshal(bodyBytes, &openAIErr)
		if openAIErr.Error != nil {
			return nil, fmt.Errorf("%s error (HTTP %d): %s", c.ProviderKey, resp.StatusCode, openAIErr.Error.Message)
		}
		return nil, fmt.Errorf("%s HTTP status %d: %s", c.ProviderKey, resp.StatusCode, string(bodyBytes))
	}

	if reqBody.Stream {
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
			if data == "[DONE]" {
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				if len(chunk.Choices) > 0 {
					content := chunk.Choices[0].Delta.Content
					if content != "" {
						fullText.WriteString(content)
						onToken(content)
					}
				}
			}
		}
		return ParseResponse(fullText.String()), nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(bodyBytes, &openAIResp); err != nil {
		return nil, err
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("received empty choices from %s API", c.ProviderKey)
	}

	responseText := openAIResp.Choices[0].Message.Content
	return ParseResponse(responseText), nil
}
