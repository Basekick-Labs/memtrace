/** Base exception for Memtrace API errors. */
export class MemtraceError extends Error {
  public readonly statusCode: number;

  constructor(message: string, statusCode: number = 0) {
    super(message);
    this.name = "MemtraceError";
    this.statusCode = statusCode;
  }
}

/** Raised on 401 Unauthorized responses. */
export class AuthenticationError extends MemtraceError {
  constructor(message: string = "Invalid or missing API key") {
    super(message, 401);
    this.name = "AuthenticationError";
  }
}

/** Raised on 404 Not Found responses. */
export class NotFoundError extends MemtraceError {
  constructor(message: string = "Resource not found") {
    super(message, 404);
    this.name = "NotFoundError";
  }
}

/** Raised on 409 Conflict responses (e.g. duplicate memory). */
export class ConflictError extends MemtraceError {
  constructor(message: string = "Conflict") {
    super(message, 409);
    this.name = "ConflictError";
  }
}

/**
 * Raised on 503 responses when the caller's organization has no Arc instance
 * configured. Memtrace deployments are multi-tenant; an administrator must
 * run `memtrace org add-arc <org_id>` before that organization can read or
 * write memories.
 */
export class NoArcInstanceError extends MemtraceError {
  constructor(message: string = "no arc instance configured for this org") {
    super(message, 503);
    this.name = "NoArcInstanceError";
  }
}
