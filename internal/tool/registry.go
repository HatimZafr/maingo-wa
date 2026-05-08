package tool

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  ParameterSchema `json:"parameters"`
}

type ParameterSchema struct {
	Type       string                   `json:"type"`
	Properties map[string]ParameterProp `json:"properties"`
	Required   []string                 `json:"required,omitempty"`
}

type ParameterProp struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

type Tool interface {
	Metadata() ToolDef
	Execute(ctx context.Context, argsJSON string) (string, error)
}

type Registry struct {
	tools map[string]Tool
	cfg   Config
}

type Config struct {
	DefinitionsDir  string
	CustomDir       string
	ShellTimeoutSec int
	HTTPTimeoutSec  int
}

func NewRegistry(cfg Config) *Registry {
	return &Registry{
		tools: make(map[string]Tool),
		cfg:   cfg,
	}
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Metadata().Name] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) All() []ToolDef {
	defs := make([]ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Metadata())
	}
	return defs
}

func (r *Registry) Count() int {
	return len(r.tools)
}

func (r *Registry) Scan() error {
	entries, err := os.ReadDir(r.cfg.DefinitionsDir)
	if err != nil {
		return fmt.Errorf("read definitions dir %s: %w", r.cfg.DefinitionsDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") && !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := r.cfg.DefinitionsDir + "/" + entry.Name()
		t, err := ParseYAMLTool(path, r.cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[TOOL] Parse error %s: %v\n", entry.Name(), err)
			continue
		}
		r.Register(t)
		fmt.Printf("[TOOL] Loaded: %s\n", t.Metadata().Name)
	}
	return nil
}

func (r *Registry) Execute(ctx context.Context, name string, argsJSON string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("tool tidak ditemukan: %s", name)
	}
	return t.Execute(ctx, argsJSON)
}

func ParseYAMLTool(path string, cfg Config) (Tool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var def yamlToolDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	if def.Name == "" || def.Description == "" {
		return nil, fmt.Errorf("name dan description wajib diisi")
	}

	toolDef := ToolDef{
		Name:        def.Name,
		Description: def.Description,
		Parameters: ParameterSchema{
			Type:       "object",
			Properties: make(map[string]ParameterProp),
		},
	}

	for _, p := range def.Parameters {
		prop := ParameterProp{
			Type:        p.Type,
			Description: p.Description,
		}
		if len(p.Enum) > 0 {
			prop.Enum = p.Enum
		}
		toolDef.Parameters.Properties[p.Name] = prop
		if p.Required {
			toolDef.Parameters.Required = append(toolDef.Parameters.Required, p.Name)
		}
	}

	return &yamlTool{
		def:      toolDef,
		params:   def.Parameters,
		executor: def.Executor,
		cfg:      cfg,
	}, nil
}

type yamlToolDef struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Parameters  []yamlParam  `yaml:"parameters"`
	Executor    yamlExecutor `yaml:"executor"`
}

type yamlParam struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	Required    bool     `yaml:"required"`
	Description string   `yaml:"description"`
	Enum        []string `yaml:"enum,omitempty"`
	Regex       string   `yaml:"validation_regex,omitempty"`
}

type yamlExecutor struct {
	Type          string            `yaml:"type"`
	Command       string            `yaml:"command,omitempty"`
	URL           string            `yaml:"url,omitempty"`
	Method        string            `yaml:"method,omitempty"`
	Headers       map[string]string `yaml:"headers,omitempty"`
	Body          string            `yaml:"body,omitempty"`
	TLSSkipVerify bool              `yaml:"tls_skip_verify,omitempty"`
	Routes        []yamlRoute       `yaml:"routes,omitempty"`
}

type yamlRoute struct {
	When    map[string]string `yaml:"when"`
	Method  string            `yaml:"method,omitempty"`
	URL     string            `yaml:"url,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`
}

type yamlTool struct {
	def      ToolDef
	params   []yamlParam
	executor yamlExecutor
	cfg      Config
}

func (t *yamlTool) Metadata() ToolDef {
	return t.def
}

func (t *yamlTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	args, err := parseArgs(argsJSON)
	if err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if err := t.validate(args); err != nil {
		return "", fmt.Errorf("validasi parameter: %w", err)
	}

	e := t.executor

	// Route matching: cari route yang "when" conditions-nya cocok dengan args
	if len(e.Routes) > 0 {
		route := t.matchRoute(args)
		if route == nil {
			return "", fmt.Errorf("tidak ada route yang cocok untuk parameter: %v", args)
		}
		e.Method = route.Method
		e.URL = route.URL
		e.Headers = route.Headers
		e.Body = route.Body
	}

	switch e.Type {
	case "shell":
		return t.executeShell(ctx, args, e)
	case "http":
		return t.executeHTTP(ctx, args, e)
	case "raw_shell":
		return t.executeRawShell(ctx, args, e)
	default:
		return "", fmt.Errorf("executor type tidak dikenal: %s", e.Type)
	}
}

func (t *yamlTool) matchRoute(args map[string]string) *yamlRoute {
	for i := range t.executor.Routes {
		r := &t.executor.Routes[i]
		match := true
		for k, v := range r.When {
			if args[k] != v {
				match = false
				break
			}
		}
		if match {
			return r
		}
	}
	return nil
}

func (t *yamlTool) validate(args map[string]string) error {
	for _, p := range t.params {
		val, ok := args[p.Name]
		if !ok {
			if p.Required {
				return fmt.Errorf("parameter %s wajib diisi", p.Name)
			}
			continue
		}
		switch p.Type {
		case "number":
			if _, err := strconv.ParseFloat(val, 64); err != nil {
				return fmt.Errorf("parameter %s harus number, got %q", p.Name, val)
			}
		case "boolean":
			if val != "true" && val != "false" {
				return fmt.Errorf("parameter %s harus boolean, got %q", p.Name, val)
			}
		}
		if p.Regex != "" {
			matched, err := regexp.MatchString(p.Regex, val)
			if err != nil {
				return fmt.Errorf("regex %s untuk %s invalid: %w", p.Regex, p.Name, err)
			}
			if !matched {
				return fmt.Errorf("parameter %s tidak match pattern %s", p.Name, p.Regex)
			}
		}
	}
	return nil
}

func (t *yamlTool) executeShell(ctx context.Context, args map[string]string, e yamlExecutor) (string, error) {
	parts := strings.Fields(substitute(e.Command, args))
	if len(parts) == 0 {
		return "", fmt.Errorf("command kosong")
	}
	timeout := time.Duration(t.cfg.ShellTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command error: %w", err)
	}
	return string(output), nil
}

func (t *yamlTool) executeRawShell(ctx context.Context, args map[string]string, e yamlExecutor) (string, error) {
	timeout := time.Duration(t.cfg.ShellTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	script := substitute(e.Command, args)
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("raw_shell error: %w", err)
	}
	return string(output), nil
}

func (t *yamlTool) executeHTTP(ctx context.Context, args map[string]string, e yamlExecutor) (string, error) {
	method := e.Method
	if method == "" {
		method = "GET"
	}
	targetURL := substitute(e.URL, args)
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if err := validateURL(parsed); err != nil {
		return "", fmt.Errorf("URL tidak diizinkan: %w", err)
	}
	var body io.Reader
	if e.Body != "" {
		body = strings.NewReader(substitute(e.Body, args))
	}
	timeout := time.Duration(t.cfg.HTTPTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	for k, v := range e.Headers {
		req.Header.Set(k, substitute(v, args))
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: e.TLSSkipVerify,
		},
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
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
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, 256<<10)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("response body kosong (status: %d)", resp.StatusCode)
	}
	result := fmt.Sprintf("Status: %d\n%s", resp.StatusCode, string(data))
	return truncateStr(result, 32<<10), nil
}

func validateURL(u *url.URL) error {
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("hostname kosong")
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("IP private/loopback/link-local tidak diizinkan: %s", host)
		}
	}
	if host == "localhost" || strings.HasSuffix(host, ".local") {
		return fmt.Errorf("localhost tidak diizinkan")
	}
	return nil
}

func parseArgs(argsJSON string) (map[string]string, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
		return nil, fmt.Errorf("parse JSON args: %w", err)
	}
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result, nil
}

func substitute(template string, args map[string]string) string {
	result := template
	for k, v := range args {
		result = strings.ReplaceAll(result, "{{."+k+"}}", v)
	}
	return result
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
