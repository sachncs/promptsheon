import { AppError, ErrorCode } from './types.js';

export function wrapError(
  error: unknown,
  code: ErrorCode = ErrorCode.INTERNAL_ERROR,
  context?: string,
): AppError {
  if (error instanceof AppError) return error;
  const message = context
    ? `${context}: ${error instanceof Error ? error.message : String(error)}`
    : error instanceof Error
      ? error.message
      : String(error);
  return new AppError(code, message);
}

export function notFound(resource: string, id: string): AppError {
  return new AppError(ErrorCode.NOT_FOUND, `${resource} ${id} not found`);
}

export function validationError(message: string, details?: unknown): AppError {
  return new AppError(ErrorCode.VALIDATION_ERROR, message, details, 422);
}

export function unauthorized(message = 'authentication required'): AppError {
  return new AppError(ErrorCode.UNAUTHORIZED, message, undefined, 401);
}

export function forbidden(message = 'insufficient permissions'): AppError {
  return new AppError(ErrorCode.FORBIDDEN, message, undefined, 403);
}

export function conflict(message: string): AppError {
  return new AppError(ErrorCode.CONFLICT, message, undefined, 409);
}
