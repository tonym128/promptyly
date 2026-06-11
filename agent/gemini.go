package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"promptyly/config"
	"strings"
)

type GeminiClient struct {
	Config config.ProviderConfig
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiGenerationConfig struct {
	Temperature float64 `json:"temperature,omitempty"`
}

type geminiRequest struct {
	Contents          []geminiContent          `json:"contents"`
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig  `json:"generationConfig,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewGeminiClient(cfg config.ProviderConfig) *GeminiClient {
	if cfg.Model == "" {
		cfg.Model = "gemini-1.5-flash"
	}
	return &GeminiClient{Config: cfg}
}

func (c *GeminiClient) Generate(ctx context.Context, systemPrompt, userPrompt string, onToken func(token string)) (*Response, error) {
	if c.Config.APIKey == "" {
		return nil, fmt.Errorf("Gemini API key is not set. Configure it with 'promptyly config set gemini_key <key>'")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", c.Config.Model, c.Config.APIKey)
	if onToken != nil {
		url = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?key=%s", c.Config.Model, c.Config.APIKey)
	}

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: userPrompt}},
			},
		},
		GenerationConfig: &geminiGenerationConfig{
			Temperature: 0.2,
		},
	}

	if systemPrompt != "" {
		reqBody.SystemInstruction = &geminiSystemInstruction{
			Parts: []geminiPart{{Text: systemPrompt}},
		}
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var geminiErr geminiResponse
		_ = json.Unmarshal(bodyBytes, &geminiErr)
		if geminiErr.Error != nil {
			return nil, fmt.Errorf("Gemini API error (HTTP %d): %s", resp.StatusCode, geminiErr.Error.Message)
		}
		return nil, fmt.Errorf("Gemini API HTTP status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if onToken != nil {
		dec := json.NewDecoder(resp.Body)
		t, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("failed to read stream start token: %v", err)
		}
		delim, ok := t.(json.Delim)
		if !ok || delim != '[' {
			return nil, fmt.Errorf("expected JSON array start in stream response, got: %v", t)
		}

		var fullText strings.Builder
		for dec.More() {
			var chunk geminiResponse
			if err := dec.Decode(&chunk); err != nil {
				return nil, fmt.Errorf("failed to decode chunk: %v", err)
			}
			if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
				txt := chunk.Candidates[0].Content.Parts[0].Text
				if txt != "" {
					fullText.WriteString(txt)
					onToken(txt)
				}
			}
		}
		_, _ = dec.Token() // Read closing bracket
		return ParseResponse(fullText.String()), nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return nil, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("received empty response from Gemini API")
	}

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text
	return ParseResponse(responseText), nil
}
