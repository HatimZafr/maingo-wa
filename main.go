package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"maingo/internal/agent"
	"maingo/internal/config"
	"maingo/internal/llm"
	"maingo/internal/session"
	"maingo/internal/tool"
	"maingo/internal/whatsapp"
)

func main() {
	cfgPath := "config.toml"
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		cfgPath = p
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Gagal load config: %v", err)
	}

	systemPrompt := ""
	if sp, err := config.LoadSystemPrompt("system-prompt.txt"); err == nil {
		systemPrompt = sp
	}
	// Load semua tools/*/skill.txt sebagai tambahan system prompt
	skillFiles, _ := filepath.Glob("tools/*/skill.txt")
	for _, sf := range skillFiles {
		if skill, err := config.LoadSystemPrompt(sf); err == nil {
			systemPrompt += "\n\n" + skill
			fmt.Printf("[SKILL] Loaded: %s\n", sf)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Session store
	sessionStore, err := session.NewStore(cfg.Session.DBPath, cfg.Session.MaxHistory)
	if err != nil {
		log.Fatalf("Gagal init session store: %v", err)
	}
	defer sessionStore.Close()

	// LLM client
	llmClient := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.Temperature)

	// Tool registry
	toolCfg := tool.Config{
		ShellTimeoutSec: cfg.Tools.ShellTimeoutSec,
		HTTPTimeoutSec:  cfg.Tools.HTTPTimeoutSec,
	}
	toolRegistry := tool.NewRegistry(toolCfg)
	tool.RegisterBuiltins(toolRegistry)
	tool.RegisterMayarTools(toolRegistry)
	if err := toolRegistry.Scan(); err != nil {
		log.Fatalf("Gagal scan tools: %v", err)
	}
	fmt.Printf("Loaded %d tools\n", toolRegistry.Count())

	// WhatsApp client
	waClient, err := whatsapp.NewClient(cfg.WhatsApp.Allowlist)
	if err != nil {
		log.Fatalf("Gagal init WhatsApp client: %v", err)
	}

	// Agent
	ag := agent.New(agent.Config{
		SystemPrompt: systemPrompt,
		MaxRounds:    cfg.LLM.MaxToolRounds,
		WA:           waClient,
		LLM:          llmClient,
		Sessions:     sessionStore,
		Tools:        toolRegistry,
	})

	waClient.SetMessageHandler(ag.HandleMessage)

	// Connect WhatsApp
	if err := waClient.Connect(ctx); err != nil {
		log.Fatalf("Gagal connect WhatsApp: %v", err)
	}

	fmt.Println("Warming up npx mayar...")
	go func() {
		exec.Command("npx", "-y", "mayar@latest", "whoami", "--json").Run()
		fmt.Println("[WARMUP] npx mayar ready")
	}()
	time.Sleep(3 * time.Second)

	fmt.Println("Bot berjalan. Tekan Ctrl+C untuk berhenti.")

	<-ctx.Done()
	fmt.Println("\nShutting down...")
	waClient.Disconnect()
	fmt.Println("Bot berhenti.")
}
