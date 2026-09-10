export interface WebhookEndpoint {
  id: string;
  url: string;
  events: string;
  active: boolean;
  secretCiphertext: Buffer | null;
  createdAt: string;
}
