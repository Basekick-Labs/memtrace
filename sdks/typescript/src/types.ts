// --- Memory ---

/** Supported memory types. */
export type MemoryType = "episodic" | "session" | "decision" | "entity";

/** A stored memory event. */
export interface Memory {
  time: string;
  org_id: string;
  agent_id: string;
  session_id?: string;
  memory_type: MemoryType;
  event_type: string;
  content: string;
  metadata?: Record<string, unknown>;
  tags?: string[];
  dedup_key?: string;
  importance?: number;
  parent_id?: string;
}

/** Request body for adding a single memory. */
export interface AddMemoryRequest {
  agent_id: string;
  session_id?: string;
  memory_type: MemoryType;
  event_type: string;
  content: string;
  metadata?: Record<string, unknown>;
  tags?: string[];
  dedup_key?: string;
  importance?: number;
  parent_id?: string;
}

/** Query parameters for listing memories. */
export interface ListOptions {
  agent_id?: string;
  session_id?: string;
  memory_type?: string;
  event_type?: string;
  tags?: string;
  since?: string;
  until?: string;
  limit?: number;
  offset?: number;
  order?: "asc" | "desc";
}

/** Response for listing memories. */
export interface MemoryList {
  memories: Memory[];
  count: number;
  has_more: boolean;
}

/** Request body for searching memories. */
export interface SearchQuery {
  agent_id?: string;
  session_id?: string;
  memory_types?: string[];
  event_types?: string[];
  tags?: string[];
  content_contains?: string;
  since?: string;
  until?: string;
  min_importance?: number;
  limit?: number;
  order?: "asc" | "desc";
}

/** Response for searching memories. */
export interface SearchResult {
  results: Memory[];
  count: number;
  query_time_ms: number;
}

// --- Agents ---

/** A registered agent. */
export interface Agent {
  id: string;
  org_id: string;
  name: string;
  description?: string;
  config?: Record<string, unknown>;
  created_at: string;
  last_active_at?: string;
}

/** Request body for registering an agent. */
export interface RegisterAgentRequest {
  name: string;
  description?: string;
  config?: Record<string, unknown>;
}

/** Memory statistics for an agent. */
export interface AgentStats {
  agent_id: string;
  memory_count: number;
  memories_24h: number;
  errors_24h: number;
  session_count: number;
  active_sessions: number;
  last_active_at?: string;
  memory_types: Record<string, number>;
}

// --- Sessions ---

/** A bounded context for agent work. */
export interface Session {
  id: string;
  org_id: string;
  agent_id: string;
  status: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  closed_at?: string;
}

/** Response for listing sessions. */
export interface SessionList {
  sessions: Session[];
  count: number;
}

/** Request body for creating a session. */
export interface CreateSessionRequest {
  agent_id: string;
  metadata?: Record<string, unknown>;
}

/** Options for session context retrieval. */
export interface ContextOptions {
  max_tokens?: number;
  include_types?: string[];
  since?: string;
}

/** LLM-ready formatted context for a session. */
export interface SessionContext {
  session_id: string;
  context: string;
  memory_count: number;
}
