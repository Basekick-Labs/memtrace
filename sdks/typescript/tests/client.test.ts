import { describe, it, expect, vi, beforeEach } from "vitest";
import { Memtrace } from "../src/client";
import {
  AuthenticationError,
  ConflictError,
  MemtraceError,
  NotFoundError,
} from "../src/errors";
import {
  BASE_URL,
  API_KEY,
  MEMORY_JSON,
  AGENT_JSON,
  AGENT_STATS_JSON,
  SESSION_JSON,
  SESSION_CONTEXT_JSON,
  mockResponse,
  mockTextResponse,
} from "./fixtures";

let client: Memtrace;
let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  client = new Memtrace(BASE_URL, API_KEY);
});

// --- Convenience methods ---

describe("remember", () => {
  it("stores an episodic memory", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, MEMORY_JSON));
    const mem = await client.remember("agent_1", "User prefers dark mode");
    expect(mem.content).toBe("User prefers dark mode");
    expect(mem.memory_type).toBe("episodic");
  });

  it("sends correct request body", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, MEMORY_JSON));
    await client.remember("agent_1", "hello");
    const [, init] = fetchMock.mock.calls[0];
    const body = JSON.parse(init.body);
    expect(body.agent_id).toBe("agent_1");
    expect(body.memory_type).toBe("episodic");
    expect(body.event_type).toBe("general");
  });
});

describe("recall", () => {
  it("retrieves recent memories", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse(200, { memories: [MEMORY_JSON], count: 1, has_more: false }),
    );
    const result = await client.recall("agent_1");
    expect(result.count).toBe(1);
    expect(result.memories[0].content).toBe("User prefers dark mode");
  });
});

describe("decide", () => {
  it("logs a decision", async () => {
    const decisionJSON = {
      ...MEMORY_JSON,
      memory_type: "decision",
      event_type: "decision",
    };
    fetchMock.mockResolvedValueOnce(mockResponse(200, decisionJSON));
    const mem = await client.decide(
      "agent_1",
      "Use PostgreSQL",
      "Better JSON support",
    );
    expect(mem.memory_type).toBe("decision");
  });

  it("sends reasoning in metadata", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, MEMORY_JSON));
    await client.decide("agent_1", "Use PostgreSQL", "Better JSON support");
    const [, init] = fetchMock.mock.calls[0];
    const body = JSON.parse(init.body);
    expect(body.metadata.reasoning).toBe("Better JSON support");
  });
});

// --- Memory CRUD ---

describe("addMemory", () => {
  it("stores a memory", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, MEMORY_JSON));
    const mem = await client.addMemory({
      agent_id: "agent_1",
      memory_type: "episodic",
      event_type: "general",
      content: "hello",
    });
    expect(mem.agent_id).toBe("agent_1");
  });
});

describe("addMemories", () => {
  it("stores a batch", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse(200, { memories: [MEMORY_JSON, MEMORY_JSON], count: 2 }),
    );
    const result = await client.addMemories([
      {
        agent_id: "agent_1",
        memory_type: "episodic",
        event_type: "general",
        content: "first",
      },
      {
        agent_id: "agent_1",
        memory_type: "episodic",
        event_type: "general",
        content: "second",
      },
    ]);
    expect(result).toHaveLength(2);
  });
});

describe("listMemories", () => {
  it("lists memories", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse(200, { memories: [MEMORY_JSON], count: 1, has_more: false }),
    );
    const result = await client.listMemories({
      agent_id: "agent_1",
      limit: 10,
    });
    expect(result.count).toBe(1);
  });

  it("sends correct query params", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse(200, { memories: [], count: 0, has_more: false }),
    );
    await client.listMemories({
      agent_id: "agent_1",
      since: "2h",
      limit: 50,
    });
    const [url] = fetchMock.mock.calls[0];
    expect(url).toContain("agent_id=agent_1");
    expect(url).toContain("since=2h");
    expect(url).toContain("limit=50");
  });
});

describe("searchMemories", () => {
  it("searches memories", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse(200, {
        results: [MEMORY_JSON],
        count: 1,
        query_time_ms: 5,
      }),
    );
    const result = await client.searchMemories({
      agent_id: "agent_1",
      content_contains: "dark",
    });
    expect(result.count).toBe(1);
    expect(result.query_time_ms).toBe(5);
  });
});

// --- Agents ---

describe("registerAgent", () => {
  it("registers an agent", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, AGENT_JSON));
    const agent = await client.registerAgent({ name: "test-agent" });
    expect(agent.name).toBe("test-agent");
  });
});

describe("getAgent", () => {
  it("gets an agent", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, AGENT_JSON));
    const agent = await client.getAgent("agent_1");
    expect(agent.id).toBe("agent_1");
  });
});

describe("getAgentStats", () => {
  it("gets stats", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, AGENT_STATS_JSON));
    const stats = await client.getAgentStats("agent_1");
    expect(stats.memory_count).toBe(42);
    expect(stats.memory_types.episodic).toBe(30);
  });
});

// --- Sessions ---

describe("createSession", () => {
  it("creates a session", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, SESSION_JSON));
    const session = await client.createSession({ agent_id: "agent_1" });
    expect(session.status).toBe("active");
  });
});

describe("getSession", () => {
  it("gets a session", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, SESSION_JSON));
    const session = await client.getSession("sess_1");
    expect(session.id).toBe("sess_1");
  });
});

describe("getSessionContext", () => {
  it("gets context with opts", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, SESSION_CONTEXT_JSON));
    const ctx = await client.getSessionContext("sess_1", { since: "24h" });
    expect(ctx.memory_count).toBe(1);
  });

  it("gets context without opts", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, SESSION_CONTEXT_JSON));
    const ctx = await client.getSessionContext("sess_1");
    expect(ctx.session_id).toBe("sess_1");
  });
});

describe("listSessions", () => {
  it("lists all sessions without filter", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse(200, { sessions: [SESSION_JSON], count: 1 }),
    );
    const result = await client.listSessions();
    expect(result.count).toBe(1);
    expect(result.sessions[0].id).toBe("sess_1");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe(`${BASE_URL}/api/v1/sessions`);
  });

  it("encodes agent_id into query string when filtered", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse(200, { sessions: [], count: 0 }),
    );
    await client.listSessions("agent with spaces");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe(
      `${BASE_URL}/api/v1/sessions?agent_id=agent%20with%20spaces`,
    );
  });
});

describe("closeSession", () => {
  it("closes a session", async () => {
    const closed = {
      ...SESSION_JSON,
      status: "closed",
      closed_at: "2026-02-08T13:00:00Z",
    };
    fetchMock.mockResolvedValueOnce(mockResponse(200, closed));
    const session = await client.closeSession("sess_1");
    expect(session.status).toBe("closed");
    expect(session.closed_at).toBe("2026-02-08T13:00:00Z");
  });
});

// --- Error handling ---

describe("error handling", () => {
  it("throws AuthenticationError on 401", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse(401, { error: "Invalid API key" }),
    );
    await expect(client.getAgent("x")).rejects.toThrow(AuthenticationError);
  });

  it("throws NotFoundError on 404", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse(404, { error: "Agent not found" }),
    );
    await expect(client.getAgent("missing")).rejects.toThrow(NotFoundError);
  });

  it("throws ConflictError on 409", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse(409, { error: "Duplicate memory" }),
    );
    await expect(
      client.addMemory({
        agent_id: "agent_1",
        memory_type: "episodic",
        event_type: "general",
        content: "dup",
      }),
    ).rejects.toThrow(ConflictError);
  });

  it("throws MemtraceError on 500 with statusCode", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse(500, { error: "Internal server error" }),
    );
    await expect(client.getAgent("x")).rejects.toSatisfy((e: MemtraceError) => {
      expect(e).toBeInstanceOf(MemtraceError);
      expect(e.statusCode).toBe(500);
      return true;
    });
  });

  it("handles non-JSON error body", async () => {
    fetchMock.mockResolvedValueOnce(mockTextResponse(502, "Bad Gateway"));
    await expect(client.getAgent("x")).rejects.toSatisfy((e: MemtraceError) => {
      expect(e).toBeInstanceOf(MemtraceError);
      expect(e.message).toContain("Bad Gateway");
      return true;
    });
  });

  it("preserves error message from server", async () => {
    fetchMock.mockResolvedValueOnce(
      mockResponse(401, { error: "Invalid API key" }),
    );
    await expect(client.getAgent("x")).rejects.toSatisfy((e: AuthenticationError) => {
      expect(e.message).toBe("Invalid API key");
      expect(e.statusCode).toBe(401);
      return true;
    });
  });
});

// --- Headers ---

describe("headers", () => {
  it("sends x-api-key header", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, AGENT_JSON));
    await client.getAgent("agent_1");
    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers["x-api-key"]).toBe(API_KEY);
  });

  it("sends Content-Type on POST", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, MEMORY_JSON));
    await client.remember("agent_1", "hello");
    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers["Content-Type"]).toBe("application/json");
  });

  it("does not send Content-Type on GET", async () => {
    fetchMock.mockResolvedValueOnce(mockResponse(200, AGENT_JSON));
    await client.getAgent("agent_1");
    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers["Content-Type"]).toBeUndefined();
  });
});

// --- URL construction ---

describe("URL construction", () => {
  it("strips trailing slash from baseURL", async () => {
    const c = new Memtrace(BASE_URL + "/", API_KEY);
    fetchMock.mockResolvedValueOnce(mockResponse(200, AGENT_JSON));
    await c.getAgent("agent_1");
    const [url] = fetchMock.mock.calls[0];
    expect(url).toBe(`${BASE_URL}/api/v1/agents/agent_1`);
  });
});
