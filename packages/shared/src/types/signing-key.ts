/**
 * SigningKey — an operator-managed ed25519 public key registered
 * against an organisation. The private key never leaves the
 * operator's machine; signatures are produced externally and
 * posted to /api/commits/:oid/sign.
 */

export interface SigningKey {
  id: string;
  organizationId: string;
  label: string;
  fingerprint: string;
  publicKeyPem: string;
  createdBy: string;
  createdAt: string;
  deactivatedAt: string | null;
}

export interface SigningKeyCreateInput {
  organizationId: string;
  label: string;
  publicKeyPem: string;
  createdBy: string;
}
