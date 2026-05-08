package tool

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func RegisterBuiltins(r *Registry) {
	r.Register(&webSearchTool{})
	r.Register(&httpFetchTool{})
}

// -- web_search --

type webSearchTool struct{}

func (t *webSearchTool) Metadata() ToolDef {
	return ToolDef{
		Name:        "web_search",
		Description: "Cari informasi di web. Return teks hasil pencarian dari DuckDuckGo.",
		Parameters: ParameterSchema{
			Type: "object",
			Properties: map[string]ParameterProp{
				"query": {Type: "string", Description: "Query pencarian"},
			},
			Required: []string{"query"},
		},
	}
}

func (t *webSearchTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	args, err := parseArgs(argsJSON)
	if err != nil {
		return "", err
	}
	query := url.QueryEscape(args["query"])
	reqURL := "https://lite.duckduckgo.com/lite/?q=" + query

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MaingoBot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request gagal: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, 512<<10)
	raw, _ := io.ReadAll(limited)

	text := stripHTML(string(raw))
	text = cleanWhitespace(text)
	lines := strings.Split(text, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || len(line) < 30 {
			continue
		}
		result = append(result, line)
		if len(result) >= 20 {
			break
		}
	}
	if len(result) == 0 {
		return "Tidak ada hasil pencarian yang relevan.", nil
	}
	return strings.Join(result, "\n"), nil
}

// -- http_fetch --

type httpFetchTool struct{}

func (t *httpFetchTool) Metadata() ToolDef {
	return ToolDef{
		Name:        "http_fetch",
		Description: "Ambil konten dari URL dan kembalikan teks yang sudah dibersihkan dari HTML. Gunakan untuk membaca halaman web atau API.",
		Parameters: ParameterSchema{
			Type: "object",
			Properties: map[string]ParameterProp{
				"url":    {Type: "string", Description: "URL yang akan di-fetch"},
				"method": {Type: "string", Description: "HTTP method", Enum: []string{"GET", "POST"}},
			},
			Required: []string{"url"},
		},
	}
}

func (t *httpFetchTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	args, err := parseArgs(argsJSON)
	if err != nil {
		return "", err
	}

	targetURL := args["url"]
	method := args["method"]
	if method == "" {
		method = "GET"
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return "", fmt.Errorf("URL invalid: %w", err)
	}
	if err := validateURL(parsed); err != nil {
		return "", fmt.Errorf("URL diblokir: %w", err)
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL != nil {
				return validateURL(req.URL)
			}
			return nil
		},
	}

	req, _ := http.NewRequestWithContext(ctx, method, targetURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MaingoBot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch gagal: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, 256<<10)
	raw, _ := io.ReadAll(limited)

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") || strings.HasPrefix(string(raw), "<!") || strings.HasPrefix(string(raw), "<html") {
		text := stripHTML(string(raw))
		text = cleanWhitespace(text)
		return truncateStr(text, 16<<10), nil
	}

	return truncateStr(string(raw), 16<<10), nil
}

// -- helpers --

var (
	htmlTag  = regexp.MustCompile(`<[^>]*>`)
	htmlEnt  = regexp.MustCompile(`&[a-z]+;|&#\d+;`)
	multiSpc = regexp.MustCompile(`\n\s*\n\s*\n+`)
)

func stripHTML(s string) string {
	s = htmlTag.ReplaceAllString(s, " ")
	s = htmlEnt.ReplaceAllString(s, " ")
	s = regexp.MustCompile(`<style[^>]*>.*?</style>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`<script[^>]*>.*?</script>`).ReplaceAllString(s, "")
	return s
}

func cleanWhitespace(s string) string {
	s = multiSpc.ReplaceAllString(s, "\n\n")
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return strings.Join(out, "\n")
}

