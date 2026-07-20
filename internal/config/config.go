package config

import (
	"flag"
	"os"
)

type Config struct {
	DataDir   string
	HTTPAddr  string
	HTTPToken string
	EnableStdio bool
	EnableHTTP  bool
}

func Load() Config {
	cfg := Config{
		DataDir:   envOrDefault("WAITING_DATA_DIR", "./data"),
		HTTPToken: os.Getenv("WAITING_MCP_TOKEN"),
	}

	var stdio bool
	var httpAddr string
	flag.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "directory for markdown data")
	flag.BoolVar(&stdio, "stdio", false, "enable stdio MCP transport")
	flag.StringVar(&httpAddr, "http", "", "enable HTTP MCP transport on given address (e.g. :8080)")
	flag.Parse()

	cfg.EnableStdio = stdio
	cfg.EnableHTTP = httpAddr != ""
	cfg.HTTPAddr = httpAddr

	if !cfg.EnableStdio && !cfg.EnableHTTP {
		cfg.EnableStdio = true
	}

	return cfg
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
