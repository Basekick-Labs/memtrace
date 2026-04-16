export { Memtrace } from "./client";
export type { MemtraceOptions } from "./client";

export {
  MemtraceError,
  AuthenticationError,
  NotFoundError,
  ConflictError,
} from "./errors";

export type {
  MemoryType,
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
