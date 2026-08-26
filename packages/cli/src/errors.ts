import { ExitCode, EXIT } from './version.js';

/**
 * Wrapper around every API failure so the dispatcher can map the
 * cause to a stable exit code. Throwing one of these from a command
 * implementation is the documented way to fail.
 */
export class CliError extends Error {
  constructor(
    public readonly code: ExitCode,
    message: string,
    public readonly detail?: unknown,
  ) {
    super(message);
    this.name = 'CliError';
  }
}

export class NetworkError extends CliError {
  constructor(message: string, detail?: unknown) {
    super(EXIT.NETWORK_ERROR, message, detail);
    this.name = 'NetworkError';
  }
}

export class ApiError extends CliError {
  constructor(
    public readonly status: number,
    message: string,
    public readonly responseBody: unknown,
  ) {
    super(EXIT.API_ERROR, `${status}: ${message}`, responseBody);
    this.name = 'ApiError';
  }
}

export class NotFoundError extends CliError {
  constructor(what: string, id: string) {
    super(EXIT.NOT_FOUND, `${what} ${id} not found`);
    this.name = 'NotFoundError';
  }
}

export class ConflictError extends CliError {
  constructor(message: string) {
    super(EXIT.CONFLICT, message);
    this.name = 'ConflictError';
  }
}

export class AuthError extends CliError {
  constructor(message = 'PROMPTSHEON_API_KEY missing or rejected') {
    super(EXIT.AUTH_ERROR, message);
    this.name = 'AuthError';
  }
}

export class BadArgsError extends CliError {
  constructor(message: string) {
    super(EXIT.BAD_ARGS, message);
    this.name = 'BadArgsError';
  }
}