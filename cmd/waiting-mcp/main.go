package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/priahin-i/assistente_2/internal/config"
	"github.com/priahin-i/assistente_2/internal/mcpserver"
	mdstore "github.com/priahin-i/assistente_2/internal/store/markdown"
)

func main() {
	cfg := config.Load()
	repo := mdstore.NewStore(cfg.DataDir)
	server := mcpserver.NewServer(repo)

	ctx := context.Background()
	errCh := make(chan error, 2)
	started := 0

	if cfg.EnableStdio {
		started++
		go func() {
			errCh <- server.Run(ctx, &sdkmcp.StdioTransport{})
		}()
	}

	if cfg.EnableHTTP {
		started++
		handler := sdkmcp.NewStreamableHTTPHandler(func(req *http.Request) *sdkmcp.Server {
			return server
		}, &sdkmcp.StreamableHTTPOptions{Stateless: true})
		mux := http.NewServeMux()
		mux.Handle("/mcp", withBearerAuth(cfg.HTTPToken, handler))
		go func() {
			log.Printf("waiting-mcp HTTP listening on %s", cfg.HTTPAddr)
			errCh <- http.ListenAndServe(cfg.HTTPAddr, mux)
		}()
	}

	if started == 0 {
		log.Fatal("no transport enabled")
	}

	if err := <-errCh; err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func withBearerAuth(token string, next http.Handler) http.Handler {
	if token == "" {
		log.Println("warning: WAITING_MCP_TOKEN is empty, HTTP endpoint is unauthenticated")
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
