import { describe, it, expect } from "vitest";
import type {
  Memory,
  AddMemoryRequest,
  ListOptions,
  SearchQuery,
  Agent,
  AgentStats,
  Session,
  SessionContext,
} from "../src/types";
import { MEMORY_JSON, AGENT_JSON, AGENT_STATS_JSON, SESSION_JSON, SESSION_CONTEXT_JSON } from "./fixtures";

describe("Memory", () => {
  it("full memory has all fields", () => {
    const m = MEMORY_JSON as Memory;
    expect(m.memory_type).toBe("episodic");
    expect(m.importance).toBe(0.8);
    expect(m.tags).toEqual(["preference"]);
  });

  it("minimal memory works without optional fields", () => {
    const m: Memory = {
      time: "2026-02-08T12:00:00Z",
      org_id: "org_1",
      agent_id: "agent_1",
      memory_type: "decision",
      event_type: "decision",
      content: "chose option A",
    };
    expect(m.session_id).toBeUndefined();
    expect(m.tags).toBeUndefined();
  });
});

describe("AddMemoryRequest", () => {
  it("optional fields are excluded by JSON.stringify", () => {
    const req: AddMemoryRequest = {
      agent_id: "agent_1",
      memory_type: "episodic",
      event_type: "general",
      content: "hello",
    };
    const json = JSON.parse(JSON.stringify(req));
    expect(json).not.toHaveProperty("session_id");
    expect(json).not.toHaveProperty("tags");
    expect(json).not.toHaveProperty("dedup_key");
  });
});

describe("ListOptions", () => {
  it("accepts order literal", () => {
    const opts: ListOptions = { agent_id: "agent_1", order: "desc", limit: 50 };
    expect(opts.order).toBe("desc");
  });
});

describe("SearchQuery", () => {
  it("tags is a list", () => {
    const q: SearchQuery = { tags: ["a", "b"] };
    expect(q.tags).toEqual(["a", "b"]);
  });
});

describe("Agent", () => {
  it("parses from JSON fixture", () => {
    const a = AGENT_JSON as Agent;
    expect(a.name).toBe("test-agent");
    expect(a.created_at).toBe("2026-02-08T10:00:00Z");
  });
});

describe("AgentStats", () => {
  it("parses from JSON fixture", () => {
    const s = AGENT_STATS_JSON as AgentStats;
    expect(s.memory_count).toBe(42);
    expect(s.memory_types.episodic).toBe(30);
  });
});

describe("Session", () => {
  it("parses from JSON fixture", () => {
    const s = SESSION_JSON as Session;
    expect(s.status).toBe("active");
    expect(s.closed_at).toBeNull();
  });
});

describe("SessionContext", () => {
  it("parses from JSON fixture", () => {
    const sc = SESSION_CONTEXT_JSON as SessionContext;
    expect(sc.memory_count).toBe(1);
    expect(sc.context).toContain("Session Context");
  });
});
