package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Basekick-Labs/memtrace/pkg/sdk"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"

func main() {
	url := os.Getenv("MEMTRACE_URL")
	if url == "" {
		url = "http://localhost:9100"
	}
	apiKey := os.Getenv("MEMTRACE_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "MEMTRACE_API_KEY is required")
		os.Exit(1)
	}

	client := sdk.New(url, apiKey)
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "memtrace",
		Title:   "Memtrace — Memory for AI Agents",
		Version: version,
	}, &mcp.ServerOptions{
		Instructions: "Memtrace provides persistent memory for AI agents. Use these tools to store, recall, and search memories. Memories are temporal — always specify time windows when recalling.",
	})

	registerTools(server, client)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

// --- Arg types ---

type RememberArgs struct {
	AgentID    string                 `json:"agent_id" jsonschema:"required,description=Agent ID"`
	Content    string                 `json:"content" jsonschema:"required,description=Memory content text"`
	MemoryType string                 `json:"memory_type,omitempty" jsonschema:"description=Memory type: episodic (default)\\, decision\\, entity\\, session"`
	EventType  string                 `json:"event_type,omitempty" jsonschema:"description=Event type (e.g. page_crawled\\, error\\, api_call). Default: general"`
	SessionID  string                 `json:"session_id,omitempty" jsonschema:"description=Session ID to scope this memory to"`
	Tags       []string               `json:"tags,omitempty" jsonschema:"description=Tags for categorization"`
	Importance float64                `json:"importance,omitempty" jsonschema:"description=Importance score 0.0 to 1.0"`
	Metadata   map[string]interface{} `json:"metadata,omitempty" jsonschema:"description=Arbitrary key-value metadata"`
}

type RecallArgs struct {
	AgentID    string `json:"agent_id" jsonschema:"required,description=Agent ID"`
	Since      string `json:"since,omitempty" jsonschema:"description=Time window (e.g. 2h\\, 24h\\, 7d). Default: 24h"`
	SessionID  string `json:"session_id,omitempty" jsonschema:"description=Filter by session ID"`
	MemoryType string `json:"memory_type,omitempty" jsonschema:"description=Filter by memory type"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Max results. Default: 50"`
}

type SearchArgs struct {
	AgentID         string   `json:"agent_id,omitempty" jsonschema:"description=Filter by agent ID"`
	ContentContains string   `json:"content_contains,omitempty" jsonschema:"description=Search text within memory content"`
	MemoryTypes     []string `json:"memory_types,omitempty" jsonschema:"description=Filter by memory types (episodic\\, decision\\, entity\\, session)"`
	Tags            []string `json:"tags,omitempty" jsonschema:"description=Filter by tags"`
	Since           string   `json:"since,omitempty" jsonschema:"description=Time window (e.g. 2h\\, 24h)"`
	MinImportance   float64  `json:"min_importance,omitempty" jsonschema:"description=Minimum importance score 0.0 to 1.0"`
	Limit           int      `json:"limit,omitempty" jsonschema:"description=Max results. Default: 50"`
}

type DecideArgs struct {
	AgentID   string `json:"agent_id" jsonschema:"required,description=Agent ID"`
	Decision  string `json:"decision" jsonschema:"required,description=The decision that was made"`
	Reasoning string `json:"reasoning" jsonschema:"required,description=Why this decision was made"`
}

type SessionCreateArgs struct {
	AgentID  string                 `json:"agent_id" jsonschema:"required,description=Agent ID that owns this session"`
	Metadata map[string]interface{} `json:"metadata,omitempty" jsonschema:"description=Session metadata (e.g. goal\\, context)"`
}

type SessionContextArgs struct {
	SessionID    string   `json:"session_id" jsonschema:"required,description=Session ID"`
	Since        string   `json:"since,omitempty" jsonschema:"description=Time window (e.g. 4h). Default: all session memories"`
	IncludeTypes []string `json:"include_types,omitempty" jsonschema:"description=Memory types to include (episodic\\, decision\\, entity\\, session)"`
	MaxTokens    int      `json:"max_tokens,omitempty" jsonschema:"description=Approximate max token budget for context"`
}

type AgentRegisterArgs struct {
	Name        string `json:"name" jsonschema:"required,description=Agent name (used as the agent ID)"`
	Description string `json:"description,omitempty" jsonschema:"description=What this agent does"`
}

// --- Tool registration ---

func registerTools(server *mcp.Server, client *sdk.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "memtrace_remember",
		Description: "Store a memory. Use this to record actions, observations, events, or any information the agent should remember later.",
	}, rememberHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memtrace_recall",
		Description: "Retrieve recent memories for an agent. Returns memories in reverse chronological order.",
	}, recallHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memtrace_search",
		Description: "Search memories with structured filters: by content text, memory types, tags, importance, and time range.",
	}, searchHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memtrace_decide",
		Description: "Log a decision with reasoning. Creates an auditable record of what was decided and why.",
	}, decideHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memtrace_session_create",
		Description: "Start a new session — a bounded context for a unit of work. Memories can be scoped to sessions.",
	}, sessionCreateHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memtrace_session_context",
		Description: "Get LLM-ready session context as formatted markdown. Returns all memories for a session grouped by type, ready to inject into a prompt.",
	}, sessionContextHandler(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "memtrace_agent_register",
		Description: "Register a new agent. Required before storing memories for that agent.",
	}, agentRegisterHandler(client))
}

// --- Handlers ---

func rememberHandler(c *sdk.Client) mcp.ToolHandlerFor[RememberArgs, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args RememberArgs) (*mcp.CallToolResult, any, error) {
		memType := args.MemoryType
		if memType == "" {
			memType = "episodic"
		}
		eventType := args.EventType
		if eventType == "" {
			eventType = "general"
		}

		mem, err := c.AddMemory(ctx, &sdk.AddMemoryRequest{
			AgentID:    args.AgentID,
			Content:    args.Content,
			MemoryType: memType,
			EventType:  eventType,
			SessionID:  args.SessionID,
			Tags:       args.Tags,
			Importance: args.Importance,
			Metadata:   args.Metadata,
		})
		if err != nil {
			return nil, nil, err
		}
		return textResult(mem)
	}
}

func recallHandler(c *sdk.Client) mcp.ToolHandlerFor[RecallArgs, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args RecallArgs) (*mcp.CallToolResult, any, error) {
		since := args.Since
		if since == "" {
			since = "24h"
		}
		limit := args.Limit
		if limit == 0 {
			limit = 50
		}

		list, err := c.ListMemories(ctx, &sdk.ListOptions{
			AgentID:    args.AgentID,
			Since:      since,
			SessionID:  args.SessionID,
			MemoryType: args.MemoryType,
			Limit:      limit,
			Order:      "desc",
		})
		if err != nil {
			return nil, nil, err
		}
		return textResult(list)
	}
}

func searchHandler(c *sdk.Client) mcp.ToolHandlerFor[SearchArgs, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args SearchArgs) (*mcp.CallToolResult, any, error) {
		limit := args.Limit
		if limit == 0 {
			limit = 50
		}

		result, err := c.SearchMemories(ctx, &sdk.SearchQuery{
			AgentID:         args.AgentID,
			ContentContains: args.ContentContains,
			MemoryTypes:     args.MemoryTypes,
			Tags:            args.Tags,
			Since:           args.Since,
			MinImportance:   args.MinImportance,
			Limit:           limit,
			Order:           "desc",
		})
		if err != nil {
			return nil, nil, err
		}
		return textResult(result)
	}
}

func decideHandler(c *sdk.Client) mcp.ToolHandlerFor[DecideArgs, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args DecideArgs) (*mcp.CallToolResult, any, error) {
		mem, err := c.Decide(ctx, args.AgentID, args.Decision, args.Reasoning)
		if err != nil {
			return nil, nil, err
		}
		return textResult(mem)
	}
}

func sessionCreateHandler(c *sdk.Client) mcp.ToolHandlerFor[SessionCreateArgs, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args SessionCreateArgs) (*mcp.CallToolResult, any, error) {
		session, err := c.CreateSession(ctx, &sdk.CreateSessionRequest{
			AgentID:  args.AgentID,
			Metadata: args.Metadata,
		})
		if err != nil {
			return nil, nil, err
		}
		return textResult(session)
	}
}

func sessionContextHandler(c *sdk.Client) mcp.ToolHandlerFor[SessionContextArgs, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args SessionContextArgs) (*mcp.CallToolResult, any, error) {
		result, err := c.GetSessionContext(ctx, args.SessionID, &sdk.ContextOptions{
			Since:        args.Since,
			IncludeTypes: args.IncludeTypes,
			MaxTokens:    args.MaxTokens,
		})
		if err != nil {
			return nil, nil, err
		}
		// Return the markdown context directly as text — it's already LLM-ready.
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: result.Context},
			},
		}, nil, nil
	}
}

func agentRegisterHandler(c *sdk.Client) mcp.ToolHandlerFor[AgentRegisterArgs, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args AgentRegisterArgs) (*mcp.CallToolResult, any, error) {
		agent, err := c.RegisterAgent(ctx, &sdk.RegisterAgentRequest{
			Name:        args.Name,
			Description: args.Description,
		})
		if err != nil {
			return nil, nil, err
		}
		return textResult(agent)
	}
}

// textResult marshals v to JSON and returns it as a text content result.
func textResult(v interface{}) (*mcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(data)},
		},
	}, nil, nil
}
