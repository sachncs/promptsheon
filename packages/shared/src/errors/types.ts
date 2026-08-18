export enum ErrorCode {
  NOT_FOUND = 'NOT_FOUND',
  VALIDATION_ERROR = 'VALIDATION_ERROR',
  UNAUTHORIZED = 'UNAUTHORIZED',
  FORBIDDEN = 'FORBIDDEN',
  CONFLICT = 'CONFLICT',
  RATE_LIMITED = 'RATE_LIMITED',
  INTERNAL_ERROR = 'INTERNAL_ERROR',
  BAD_REQUEST = 'BAD_REQUEST',
  DEPENDENCY_FAILED = 'DEPENDENCY_FAILED',
  CAS_ERROR = 'CAS_ERROR',
  LLM_ERROR = 'LLM_ERROR',
  SELF_EVOLVE_ERROR = 'SELF_EVOLVE_ERROR',
  RELEASE_ERROR = 'RELEASE_ERROR',
  APPROVAL_REQUIRED = 'APPROVAL_REQUIRED',
  IDEMPOTENCY_CONFLICT = 'IDEMPOTENCY_CONFLICT',
}

export interface ErrorResponse {
  error: {
    code: ErrorCode;
    message: string;
    details?: unknown;
    traceId?: string;
  };
}

export class AppError extends Error {
  constructor(
    public readonly code: ErrorCode,
    message: string,
    public readonly details?: unknown,
    public readonly statusCode: number = 500,
  ) {
    super(message);
    this.name = 'AppError';
  }
}
