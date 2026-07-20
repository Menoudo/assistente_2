package mcpserver

import (
	"context"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/priahin-i/assistente_2/internal/mcp/tools"
	"github.com/priahin-i/assistente_2/internal/store"
)

func NewServer(repo store.Repository) *sdkmcp.Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "waiting-mcp",
		Title:   "Waiting For MCP",
		Version: "0.1.0",
	}, nil)

	waitingTools := tools.NewWaitingTools(repo)
	personTools := tools.NewPersonTools(repo)

	registerWaitingTools(server, waitingTools)
	registerPersonTools(server, personTools)
	registerResources(server, repo)

	return server
}

func registerWaitingTools(server *sdkmcp.Server, waitingTools *tools.WaitingTools) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "waiting_create",
		Description: "Create a new waiting-for obligation.",
	}, wrap(waitingTools.Create))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "waiting_get",
		Description: "Get a waiting item by id.",
	}, wrap(waitingTools.Get))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "waiting_list",
		Description: "List waiting items with optional filters.",
	}, wrap(waitingTools.List))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "waiting_update",
		Description: "Update waiting item fields except history.",
	}, wrap(waitingTools.Update))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "waiting_record_check",
		Description: "Record a review/check and schedule the next check date.",
	}, wrap(waitingTools.RecordCheck))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "waiting_complete",
		Description: "Mark a waiting item as done.",
	}, wrap(waitingTools.Complete))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "waiting_cancel",
		Description: "Mark a waiting item as cancelled.",
	}, wrap(waitingTools.Cancel))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "waiting_review_due",
		Description: "List active waiting items due for review on or before a date.",
	}, wrap(waitingTools.ReviewDue))
}

func registerPersonTools(server *sdkmcp.Server, personTools *tools.PersonTools) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "person_upsert",
		Description: "Create or update a responsible person.",
	}, wrap(personTools.Upsert))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "person_list",
		Description: "List all people.",
	}, wrap(personTools.List))
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "person_get",
		Description: "Get a person and their active waiting items.",
	}, wrap(personTools.Get))
}

func registerResources(server *sdkmcp.Server, repo store.Repository) {
	server.AddResource(&sdkmcp.Resource{
		URI:         "waiting://due/today",
		Name:        "Due waiting review",
		Description: "Aggregated markdown review of waiting items due today (UTC).",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		content, err := repo.RenderDueReviewMarkdown(ctx, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		return &sdkmcp.ReadResourceResult{
			Contents: []*sdkmcp.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "text/markdown",
				Text:     content,
			}},
		}, nil
	})

	waitingHandler := func(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		id := strings.TrimPrefix(req.Params.URI, "waiting://")
		if id == "" || id == "due/today" {
			return nil, sdkmcp.ResourceNotFoundError(req.Params.URI)
		}
		content, err := repo.ReadWaitingMarkdown(ctx, id)
		if err != nil {
			return nil, sdkmcp.ResourceNotFoundError(req.Params.URI)
		}
		return &sdkmcp.ReadResourceResult{
			Contents: []*sdkmcp.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "text/markdown",
				Text:     content,
			}},
		}, nil
	}
	server.AddResourceTemplate(&sdkmcp.ResourceTemplate{
		URITemplate: "waiting://{id}",
		Name:        "Waiting markdown",
		Description: "Raw markdown for a waiting item.",
		MIMEType:    "text/markdown",
	}, waitingHandler)

	peopleHandler := func(ctx context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		id := strings.TrimPrefix(req.Params.URI, "people://")
		if id == "" {
			return nil, sdkmcp.ResourceNotFoundError(req.Params.URI)
		}
		content, err := repo.ReadPersonMarkdown(ctx, id)
		if err != nil {
			return nil, sdkmcp.ResourceNotFoundError(req.Params.URI)
		}
		return &sdkmcp.ReadResourceResult{
			Contents: []*sdkmcp.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: "text/markdown",
				Text:     content,
			}},
		}, nil
	}
	server.AddResourceTemplate(&sdkmcp.ResourceTemplate{
		URITemplate: "people://{id}",
		Name:        "Person markdown",
		Description: "Raw markdown for a person.",
		MIMEType:    "text/markdown",
	}, peopleHandler)
}

type toolHandler[In, Out any] func(context.Context, *sdkmcp.CallToolRequest, In) (*sdkmcp.CallToolResult, Out, error)

func wrap[In, Out any](handler toolHandler[In, Out]) sdkmcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input In) (*sdkmcp.CallToolResult, Out, error) {
		result, out, err := handler(ctx, req, input)
		if err == nil {
			return result, out, nil
		}
		toolResult, _, toolErr := tools.ToolError(err)
		var zero Out
		if toolErr != nil {
			return nil, zero, toolErr
		}
		return toolResult, zero, nil
	}
}
