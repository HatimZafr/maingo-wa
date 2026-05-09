package tool

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func RegisterMayarTools(r *Registry) {
	r.Register(&mayarAPITool{})
}

type mayarAPITool struct{}

func (t *mayarAPITool) Metadata() ToolDef {
	return ToolDef{
		Name:        "mayar_api",
		Description: "Panggil Mayar REST API langsung (sandbox). Auto-inject auth dari env MAYAR_API_KEY. Gunakan jika CLI mayar gagal. Base URL: https://api.mayar.club/hl/v1",
		Parameters: ParameterSchema{
			Type: "object",
			Properties: map[string]ParameterProp{
				"method":     {Type: "string", Description: "HTTP method", Enum: []string{"GET", "POST", "PUT", "DELETE"}},
				"path":       {Type: "string", Description: "API path (contoh: /invoices, /products, /transactions/balance)"},
				"body":       {Type: "string", Description: "JSON body untuk POST/PUT"},
			},
			Required: []string{"method", "path"},
		},
	}
}

func (t *mayarAPITool) Execute(ctx context.Context, argsJSON string) (string, error) {
	args, err := parseArgs(argsJSON)
	if err != nil {
		return "", err
	}

	apiKey := os.Getenv("MAYAR_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("MAYAR_API_KEY env tidak disetel. Minta user setel API key dari https://web.mayar.club → Integration")
	}

	baseURL := os.Getenv("MAYAR_API_URL")
	if baseURL == "" {
		baseURL = "https://api.mayar.club/hl/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	method := strings.ToUpper(args["method"])
	path := args["path"]
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	var body io.Reader
	if b := args["body"]; b != "" {
		body = bytes.NewReader([]byte(b))
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API call gagal: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 512<<10))

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(data))
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return string(data), nil
	}
	return truncateStr(pretty.String(), 16<<10), nil
}
