package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// FileResult represents a generated file.
type FileResult struct {
	Name    string
	Content string
}

// Response represents the parsed LLM response.
type Response struct {
	Files   map[string]string
	Summary string
	RawText string
}

// Agent defines the interface for communicating with LLM providers.
type Agent interface {
	Generate(ctx context.Context, systemPrompt, userPrompt string, onToken func(token string)) (*Response, error)
}

var (
	fileRegex    = regexp.MustCompile(`(?si)<file\s+name=["']([^"']+)["']\s*>([\s\S]*?)<\/file>`)
	summaryRegex = regexp.MustCompile(`(?si)<summary>([\s\S]*?)<\/summary>`)
	codeBlockRegex = regexp.MustCompile(`(?s)^[\s]*` + "```" + `[a-zA-Z]*\n([\s\S]*?)\n` + "```" + `[\s]*$`)
)

// ParseResponse extracts files and a summary from the LLM output.
func ParseResponse(text string) *Response {
	fmt.Printf("\n--- [DEBUG] Parsing LLM Response (%d bytes) ---\n", len(text))
	if len(text) > 500 {
		fmt.Printf("[DEBUG] Response start: %s...\n", text[:500])
	} else {
		fmt.Printf("[DEBUG] Response text: %s\n", text)
	}
	
	res := &Response{
		Files:   make(map[string]string),
		RawText: text,
	}

	// Extract files
	fileMatches := fileRegex.FindAllStringSubmatch(text, -1)
	fmt.Printf("[DEBUG] Found %d file matches in response.\n", len(fileMatches))
	
	for _, m := range fileMatches {
		if len(m) >= 3 {
			filename := strings.TrimSpace(m[1])
			content := m[2]
			
			// Strip leading/trailing whitespace
			content = strings.TrimSpace(content)
			
			rawLen := len(content)

			// If the content is wrapped in markdown code blocks, strip them
			if subMatch := codeBlockRegex.FindStringSubmatch(content); len(subMatch) >= 2 {
				content = subMatch[1]
			} else {
				// Also check for code blocks that might not have a newline after the opening backticks
				// or are just simple triple backticks
				content = strings.TrimPrefix(content, "```" + "javascript")
				content = strings.TrimPrefix(content, "```" + "js")
				content = strings.TrimPrefix(content, "```" + "html")
				content = strings.TrimPrefix(content, "```" + "css")
				content = strings.TrimPrefix(content, "```")
				content = strings.TrimSuffix(content, "```")
				content = strings.TrimSpace(content)
			}
			
			res.Files[filename] = content
			fmt.Printf("[DEBUG] Extracted file: '%s' (%d bytes, raw was %d)\n", filename, len(content), rawLen)
		}
	}

	// Fallback for truncated responses: check if there's an unclosed <file> tag at the end
	if len(fileMatches) == 0 || !strings.Contains(text, "</file>") || strings.LastIndex(text, "<file") > strings.LastIndex(text, "</file>") {
		unclosedRegex := regexp.MustCompile(`(?si)<file\s+name=["']([^"']+)["']\s*>([\s\S]*)$`)
		if m := unclosedRegex.FindStringSubmatch(text); len(m) >= 3 {
			filename := strings.TrimSpace(m[1])
			if _, exists := res.Files[filename]; !exists {
				content := m[2]
				content = strings.TrimSpace(content)
				// Basic code fence stripping for partial content
				content = strings.TrimPrefix(content, "```" + "javascript")
				content = strings.TrimPrefix(content, "```" + "js")
				content = strings.TrimPrefix(content, "```" + "html")
				content = strings.TrimPrefix(content, "```" + "css")
				content = strings.TrimPrefix(content, "```")
				
				res.Files[filename] = content
				fmt.Printf("[DEBUG] ⚠️ Extracted PARTIAL/TRUNCATED file: '%s' (%d bytes)\n", filename, len(content))
			}
		}
	}

	// Extract summary
	summaryMatch := summaryRegex.FindStringSubmatch(text)
	if len(summaryMatch) >= 2 {
		res.Summary = strings.TrimSpace(summaryMatch[1])
		fmt.Printf("[DEBUG] Extracted summary: %s\n", res.Summary)
	} else {
		// Fallback: search for any text outside the file tags
		// or generate a generic summary
		res.Summary = "Updated files: " + strings.Join(getFileKeys(res.Files), ", ")
		fmt.Printf("[DEBUG] No summary tag found, used fallback: %s\n", res.Summary)
	}
	fmt.Printf("--- [DEBUG] End Parsing ---\n\n")

	return res
}

func getFileKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
