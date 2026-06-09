package agent

import (
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
}

// Agent defines the interface for communicating with LLM providers.
type Agent interface {
	Generate(systemPrompt, userPrompt string) (*Response, error)
}

var (
	fileRegex    = regexp.MustCompile(`(?s)<file\s+name="([^"]+)">([\s\S]*?)<\/file>`)
	summaryRegex = regexp.MustCompile(`(?s)<summary>([\s\S]*?)<\/summary>`)
)

// ParseResponse extracts files and a summary from the LLM output.
func ParseResponse(text string) *Response {
	res := &Response{
		Files: make(map[string]string),
	}

	// Extract files
	fileMatches := fileRegex.FindAllStringSubmatch(text, -1)
	for _, m := range fileMatches {
		if len(m) >= 3 {
			filename := strings.TrimSpace(m[1])
			content := m[2]
			// Trim leading/trailing newlines but preserve indentation
			content = strings.TrimPrefix(content, "\n")
			content = strings.TrimSuffix(content, "\n")
			res.Files[filename] = content
		}
	}

	// Extract summary
	summaryMatch := summaryRegex.FindStringSubmatch(text)
	if len(summaryMatch) >= 2 {
		res.Summary = strings.TrimSpace(summaryMatch[1])
	} else {
		// Fallback: search for any text outside the file tags
		// or generate a generic summary
		res.Summary = "Updated files: " + strings.Join(getFileKeys(res.Files), ", ")
	}

	return res
}

func getFileKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
