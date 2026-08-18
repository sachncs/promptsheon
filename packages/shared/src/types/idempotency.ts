export interface IdempotencyRecord {
  key: string;
  statusCode: number;
  headers: string;
  body: Buffer;
  expiresAt: string;
}
