package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"web-ai/internal/chat"
	"web-ai/internal/config"
	"web-ai/internal/openai"
	"web-ai/internal/render"
	"web-ai/internal/server"
	"web-ai/internal/session"
	"web-ai/internal/store"
	"web-ai/static"
)

func envInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	configPath := flag.String("config", envString("CONFIG", "config.json"), "config file path (env: CONFIG)")
	port := flag.Int("p", envInt("PORT", 0), "listen port (env: PORT)")
	dataDir := flag.String("d", envString("DATA_DIR", ""), "data directory (env: DATA_DIR)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if *port > 0 {
		cfg.Server.Port = *port
	}
	if *dataDir != "" {
		cfg.Server.DataDir = *dataDir
	}

	if err := os.MkdirAll(cfg.Server.DataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	dbPath := filepath.Join(cfg.Server.DataDir, "web-ai.db")
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	renderer := render.New()
	client := openai.NewClient(cfg.Provider.BaseURL, cfg.Provider.APIKey, cfg.Provider.TimeoutSeconds)
	sessions := session.NewManager(st, session.DefaultTTL)
	chatService := chat.NewService(cfg, st, renderer, client)

	staticFS, err := fs.Sub(static.StaticFS, "dist")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}

	handler := server.New(cfg, st, sessions, chatService, staticFS).Handler()
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("web-ai listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
