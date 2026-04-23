package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tyrm/mcp-wikipedia-local/internal/logic"
)

type Config struct {
	Host string
	Port int
}

type Server struct {
	logic      *logic.Logic
	cfg        *Config
	sseServer  *server.SSEServer
	httpServer *http.Server
}

func New(l *logic.Logic, cfg *Config) (*Server, error) {
	mcpSrv := server.NewMCPServer(
		"mcp-wikipedia-local",
		"1.0.0",
	)

	searchTool := mcp.NewTool("search",
		mcp.WithDescription("Search Wikipedia articles by query"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default 10)"),
		),
	)

	getPageTool := mcp.NewTool("get_page",
		mcp.WithDescription("Retrieve a Wikipedia article by title"),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("Article title"),
		),
		mcp.WithString("mode",
			mcp.Description("Retrieval mode: full, lead, outline, or section (default lead)"),
		),
		mcp.WithString("section",
			mcp.Description("Section title when mode is section"),
		),
	)

	mcpSrv.AddTool(searchTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}
		limit := req.GetInt("limit", 10)

		results, err := l.Search(ctx, query, limit)
		if err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}

		var sb strings.Builder
		for i, r := range results {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.Title))
			if r.LeadSnippet != "" {
				sb.WriteString(fmt.Sprintf("   %s\n", r.LeadSnippet))
			}
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(sb.String()),
			},
		}, nil
	})

	mcpSrv.AddTool(getPageTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, err := req.RequireString("title")
		if err != nil {
			return nil, fmt.Errorf("get_page: %w", err)
		}
		mode := req.GetString("mode", "lead")
		section := req.GetString("section", "")

		resp, err := l.GetPage(ctx, title, mode, section)
		if err != nil {
			return nil, fmt.Errorf("get_page: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(resp.Content),
			},
		}, nil
	})

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	baseURL := fmt.Sprintf("http://%s", addr)

	sseSrv := server.NewSSEServer(mcpSrv,
		server.WithBaseURL(baseURL),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("/sse", sseSrv.SSEHandler())
	mux.Handle("/message", sseSrv.MessageHandler())

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return &Server{
		logic:      l,
		cfg:        cfg,
		sseServer:  sseSrv,
		httpServer: httpSrv,
	}, nil
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
