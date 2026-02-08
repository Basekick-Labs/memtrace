export const BASE_URL = "http://localhost:9100";
export const API_KEY = "mtk_test_key_12345";

export const MEMORY_JSON = {
  time: "2026-02-08T12:00:00Z",
  org_id: "org_123",
  agent_id: "agent_1",
  session_id: "sess_1",
  memory_type: "episodic",
  event_type: "general",
  content: "User prefers dark mode",
  metadata: { source: "settings" },
  tags: ["preference"],
  dedup_key: "abc123",
  importance: 0.8,
  parent_id: null,
};

export const AGENT_JSON = {
  id: "agent_1",
  org_id: "org_123",
  name: "test-agent",
  description: "A test agent",
  config: { model: "gpt-4" },
  created_at: "2026-02-08T10:00:00Z",
  last_active_at: "2026-02-08T12:00:00Z",
};

export const AGENT_STATS_JSON = {
  agent_id: "agent_1",
  memory_count: 42,
  memories_24h: 10,
  errors_24h: 1,
  session_count: 5,
  active_sessions: 2,
  last_active_at: "2026-02-08T12:00:00Z",
  memory_types: { episodic: 30, decision: 12 },
};

export const SESSION_JSON = {
  id: "sess_1",
  org_id: "org_123",
  agent_id: "agent_1",
  status: "active",
  metadata: { task: "onboarding" },
  created_at: "2026-02-08T10:00:00Z",
  updated_at: "2026-02-08T12:00:00Z",
  closed_at: null,
};

export const SESSION_CONTEXT_JSON = {
  session_id: "sess_1",
  context:
    "## Session Context (sess_1)\n\n### Recent Actions (1)\n- [12:00] general: User prefers dark mode\n",
  memory_count: 1,
};

/** Create a mock Response with JSON body. */
export function mockResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/** Create a mock Response with plain text body. */
export function mockTextResponse(status: number, text: string): Response {
  return new Response(text, { status });
}
