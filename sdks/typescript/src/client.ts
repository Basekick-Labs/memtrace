import {
  MemtraceError,
  AuthenticationError,
  NotFoundError,
  ConflictError,
  NoArcInstanceError,
} from "./errors";
import type {
  Memory,
  AddMemoryRequest,
  ListOptions,
  MemoryList,
  SearchQuery,
  SearchResult,
  Agent,
  RegisterAgentRequest,
  AgentStats,
  Session,
  SessionList,
  CreateSessionRequest,
  ContextOptions,
  SessionContext,
} from "./types";

const DEFAULT_TIMEOUT = 30_000;

export interface MemtraceOptions {
  /** Request timeout in milliseconds. Defaults to 30000. */
  timeout?: number;
}

/**
 * Client for the Memtrace API.
 *
 * @example
 * ```typescript
 * import { Memtrace } from '@basekick-labs/memtrace-sdk'
 *
 * const client = new Memtrace('http://localhost:9100', 'mtk_...')
 * await client.remember('agent_1', 'User prefers dark mode')
 * ```
 */
export class Memtrace {
  private readonly baseURL: string;
  private readonly apiKey: string;
  private readonly timeout: number;

  constructor(baseURL: string, apiKey: string, options?: MemtraceOptions) {
    this.baseURL = baseURL.replace(/\/+$/, "");
    this.apiKey = apiKey;
    this.timeout = options?.timeout ?? DEFAULT_TIMEOUT;
  }

  // --- Convenience methods ---

  /** Store an episodic memory (quick add). */
  async remember(agentId: string, content: string): Promise<Memory> {
    return this.addMemory({
      agent_id: agentId,
      memory_type: "episodic",
      event_type: "general",
      content,
    });
  }

  /** Retrieve recent memories for an agent. */
  async recall(agentId: string, since: string = "24h"): Promise<MemoryList> {
    return this.listMemories({
      agent_id: agentId,
      since,
      order: "desc",
      limit: 100,
    });
  }

  /** Log a decision with reasoning. */
  async decide(
    agentId: string,
    decision: string,
    reasoning: string,
  ): Promise<Memory> {
    return this.addMemory({
      agent_id: agentId,
      memory_type: "decision",
      event_type: "decision",
      content: decision,
      metadata: { reasoning },
    });
  }

  // --- Memory CRUD ---

  /** Store a new memory. */
  async addMemory(req: AddMemoryRequest): Promise<Memory> {
    return this.post<Memory>("/api/v1/memories", req);
  }

  /** Store multiple memories in a batch. */
  async addMemories(memories: AddMemoryRequest[]): Promise<Memory[]> {
    const result = await this.post<{ memories: Memory[]; count: number }>(
      "/api/v1/memories",
      { memories },
    );
    return result.memories;
  }

  /** List memories with filters. */
  async listMemories(opts: ListOptions): Promise<MemoryList> {
    const params = new URLSearchParams();
    if (opts.agent_id) params.set("agent_id", opts.agent_id);
    if (opts.session_id) params.set("session_id", opts.session_id);
    if (opts.memory_type) params.set("memory_type", opts.memory_type);
    if (opts.event_type) params.set("event_type", opts.event_type);
    if (opts.tags) params.set("tags", opts.tags);
    if (opts.since) params.set("since", opts.since);
    if (opts.until) params.set("until", opts.until);
    if (opts.limit !== undefined && opts.limit > 0)
      params.set("limit", String(opts.limit));
    if (opts.offset !== undefined && opts.offset > 0)
      params.set("offset", String(opts.offset));
    if (opts.order) params.set("order", opts.order);

    const qs = params.toString();
    return this.get<MemoryList>(qs ? `/api/v1/memories?${qs}` : "/api/v1/memories");
  }

  /** Search memories with structured filters. */
  async searchMemories(query: SearchQuery): Promise<SearchResult> {
    return this.post<SearchResult>("/api/v1/search", query);
  }

  // --- Agents ---

  /** Register a new agent. */
  async registerAgent(req: RegisterAgentRequest): Promise<Agent> {
    return this.post<Agent>("/api/v1/agents", req);
  }

  /** Get an agent by ID. */
  async getAgent(id: string): Promise<Agent> {
    return this.get<Agent>(`/api/v1/agents/${encodeURIComponent(id)}`);
  }

  /** Get memory stats for an agent. */
  async getAgentStats(id: string): Promise<AgentStats> {
    return this.get<AgentStats>(`/api/v1/agents/${encodeURIComponent(id)}/stats`);
  }

  // --- Sessions ---

  /** Start a new session. */
  async createSession(req: CreateSessionRequest): Promise<Session> {
    return this.post<Session>("/api/v1/sessions", req);
  }

  /** Get a session by ID. */
  async getSession(id: string): Promise<Session> {
    return this.get<Session>(`/api/v1/sessions/${encodeURIComponent(id)}`);
  }

  /** List sessions, optionally filtered by agent. */
  async listSessions(agentId?: string): Promise<SessionList> {
    const qs = agentId ? `?agent_id=${encodeURIComponent(agentId)}` : "";
    return this.get<SessionList>(`/api/v1/sessions${qs}`);
  }

  /** Get LLM-formatted session context. */
  async getSessionContext(
    id: string,
    opts?: ContextOptions,
  ): Promise<SessionContext> {
    return this.post<SessionContext>(
      `/api/v1/sessions/${encodeURIComponent(id)}/context`,
      opts ?? {},
    );
  }

  /** Close a session. */
  async closeSession(id: string): Promise<Session> {
    return this.put<Session>(`/api/v1/sessions/${encodeURIComponent(id)}`, { status: "closed" });
  }

  // --- Private HTTP helpers ---

  private async post<T>(path: string, body: unknown): Promise<T> {
    return this.request<T>("POST", path, body);
  }

  private async get<T>(path: string): Promise<T> {
    return this.request<T>("GET", path);
  }

  private async put<T>(path: string, body: unknown): Promise<T> {
    return this.request<T>("PUT", path, body);
  }

  private async request<T>(
    method: "GET" | "POST" | "PUT",
    path: string,
    body?: unknown,
  ): Promise<T> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    const headers: Record<string, string> = {
      "x-api-key": this.apiKey,
    };

    let fetchBody: string | undefined;
    if (body !== undefined) {
      headers["Content-Type"] = "application/json";
      fetchBody = JSON.stringify(body);
    }

    let response: Response;
    try {
      response = await fetch(`${this.baseURL}${path}`, {
        method,
        headers,
        body: fetchBody,
        signal: controller.signal,
      });
    } catch (err: unknown) {
      const error = err as Error;
      if (error.name === "AbortError") {
        throw new MemtraceError(
          `Request timed out after ${this.timeout}ms`,
          0,
        );
      }
      throw new MemtraceError(
        `Request failed: ${error.message}`,
        0,
      );
    } finally {
      clearTimeout(timeoutId);
    }

    const text = await response.text();

    if (response.status >= 400) {
      this.handleError(response.status, text);
    }

    return JSON.parse(text) as T;
  }

  private handleError(status: number, body: string): never {
    let message = "";
    try {
      const parsed = JSON.parse(body);
      message = parsed.error || "";
    } catch {
      message = body;
    }

    if (!message) {
      message = `HTTP ${status}`;
    }

    switch (status) {
      case 401:
        throw new AuthenticationError(message);
      case 404:
        throw new NotFoundError(message);
      case 409:
        throw new ConflictError(message);
      case 503:
        // Distinguish "this org has no arc instance" from a generic 503 so
        // callers can catch the typed error. Match on the server error text
        // — the only 503 Memtrace itself emits today is the no-arc-instance one.
        if (message.toLowerCase().includes("arc instance")) {
          throw new NoArcInstanceError(message);
        }
        throw new MemtraceError(message, status);
      default:
        throw new MemtraceError(message, status);
    }
  }
}
