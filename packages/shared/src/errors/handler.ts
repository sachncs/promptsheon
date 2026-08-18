import type { AppError, ErrorCode, ErrorResponse } from './types.js';

const ERROR_STATUS_MAP: Record<ErrorCode, number> = {
  NOT_FOUND: 404,
  VALIDATION_ERROR: 422,
  UNAUTHORIZED: 401,
  FORBIDDEN: 403,
  CONFLICT: 409,
  RATE_LIMITED: 429,
  INTERNAL_ERROR: 500,
  BAD_REQUEST: 400,
  DEPENDENCY_FAILED: 502,
  CAS_ERROR: 500,
  LLM_ERROR: 502,
  SELF_EVOLVE_ERROR: 500,
  RELEASE_ERROR: 500,
  APPROVAL_REQUIRED: 428,
  IDEMPOTENCY_CONFLICT: 409,
};

export function errorToStatus(error: AppError): number {
  return ERROR_STATUS_MAP[error.code] ?? 500;
}

export function errorToResponse(error: AppError, traceId?: string): ErrorResponse {
  return {
    error: {
      code: error.code,
      message: error.message,
      details: error.details,
      traceId,
    },
  };
}
