import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { createHash, createPublicKey, verify, type KeyObject } from 'node:crypto';
import type { RepoRepo } from '../repos/repo.js';
import { CommitRepo, deriveCommitOid } from '../repos/commit.js';
import { SigningKeyRepo, fingerprintSpki } from '../repos/signing-key.js';
import { commitInputPayload } from '@promptsheon/shared';
import { parseBody } from './validate.js';
import { registerRouteDoc } from '../openapi.js';

const UploadKeySchema = z.object({
  organizationId: z.string(),
  label: z.string().min(1).max(120),
  publicKeyPem: z.string().min(1),
});

const SignCommitSchema = z.object({
  keyId: z.string(),
  signature: z.string().min(1),
});

const DeactivateKeySchema = z.object({});

export interface SigningDeps {
  repoRepo: RepoRepo;
  commitRepo: CommitRepo;
  signingKeyRepo: SigningKeyRepo;
}

/** Canonical payload the operator signs for a commit. */
export function signPayload(input: {
  commitOid: string;
  ref: string;
  approverId: string;
  timestamp: string;
}): Buffer {
  return Buffer.from(
    commitInputPayload({
      treeOid: '',
      parents: [],
      authorId: `${input.commitOid}\x1f${input.ref}\x1f${input.approverId}\x1f${input.timestamp}`,
      timestamp: '',
      message: 'sign:v1',
    }),
  );
}

/**
 * Strict payload used by /api/commits/:oid/verify. Operators'
 * signing tooling must reproduce exactly:
 *
 *   msg = sha256(commitOid + '\x1f' + ref + '\x1f' + approverId + '\x1f' + timestamp)
 *   sig = ed25519_sign(operator_private_key, msg)
 */
export function signedMessage(input: {
  commitOid: string;
  ref: string;
  approverId: string;
  timestamp: string;
}): Buffer {
  return createHash('sha256')
    .update(`${input.commitOid}\x1f${input.ref}\x1f${input.approverId}\x1f${input.timestamp}`)
    .digest();
}

function loadPublicKey(pem: string): KeyObject {
  return createPublicKey(pem);
}

export function registerSigningRoutes(app: FastifyInstance, deps: SigningDeps): void {
  app.get('/api/orgs/:id/signing-keys', async (request, reply) => {
    const { id } = request.params as { id: string };
    return reply.send(deps.signingKeyRepo.list(id));
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/orgs/:id/signing-keys',
    summary: 'List active operator signing keys',
    tags: ['signing'],
  });

  app.post('/api/orgs/:id/signing-keys', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, UploadKeySchema, request.body);
    if (!parsed.ok) return;
    if (parsed.data.organizationId !== id) {
      return reply.code(422).send({ error: { code: 'BAD_REQUEST', message: 'organizationId mismatch' } });
    }
    try {
      fingerprintSpki(parsed.data.publicKeyPem);
    } catch {
      return reply.code(422).send({ error: { code: 'INVALID_KEY', message: 'public key PEM unparseable' } });
    }
    const userId = (request as unknown as { userId?: string }).userId ?? 'system';
    const created = deps.signingKeyRepo.create({
      organizationId: id,
      label: parsed.data.label,
      publicKeyPem: parsed.data.publicKeyPem,
      createdBy: userId,
    });
    return reply.code(201).send(created);
  });
  registerRouteDoc({
    method: 'post',
    path: '/api/orgs/:id/signing-keys',
    summary: 'Register an operator ed25519 public key (PEM/SPKI)',
    tags: ['signing'],
    body: UploadKeySchema,
  });

  app.delete('/api/orgs/:id/signing-keys/:keyId', async (request, reply) => {
    const { keyId } = request.params as { keyId: string };
    const updated = deps.signingKeyRepo.deactivate(keyId);
    if (!updated) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'signing key not found' } });
    return reply.send(updated);
  });
  registerRouteDoc({
    method: 'delete',
    path: '/api/orgs/:id/signing-keys/:keyId',
    summary: 'Deactivate a signing key (new signatures rejected; history remains valid)',
    tags: ['signing'],
  });

  app.post('/api/commits/:oid/sign', async (request, reply) => {
    const { oid } = request.params as { oid: string };
    const parsed = parseBody(reply, SignCommitSchema, request.body);
    if (!parsed.ok) return;
    const commit = deps.commitRepo.findByOid(oid);
    if (!commit) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'commit not found' } });
    const key = deps.signingKeyRepo.findById(parsed.data.keyId);
    if (!key || key.deactivatedAt) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'signing key not found' } });
    }
    const userId = (request as unknown as { userId?: string }).userId ?? 'system';
    const timestamp = new Date().toISOString();
    const msg = signedMessage({
      commitOid: oid,
      ref: commit.ref,
      approverId: userId,
      timestamp,
    });
    let valid = false;
    try {
      const publicKey = loadPublicKey(key.publicKeyPem);
      valid = verify(null, msg, publicKey, Buffer.from(parsed.data.signature, 'base64'));
    } catch {
      valid = false;
    }
    if (!valid) {
      return reply.code(422).send({ error: { code: 'BAD_SIGNATURE', message: 'signature failed verification' } });
    }
    const updated = deps.commitRepo.attachSignature(oid, parsed.data.signature, parsed.data.keyId, timestamp);
    void deriveCommitOid;
    return reply.send(updated);
  });
  registerRouteDoc({
    method: 'post',
    path: '/api/commits/:oid/sign',
    summary: 'Attach a detached ed25519 signature to a commit',
    tags: ['signing'],
    body: SignCommitSchema,
  });

  app.get('/api/commits/:oid/verify', async (request, reply) => {
    const { oid } = request.params as { oid: string };
    const commit = deps.commitRepo.findByOid(oid);
    if (!commit) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'commit not found' } });
    if (!commit.signature || !commit.signedKeyId || !commit.signedAt) {
      return reply.send({ valid: false, reason: 'unsigned' });
    }
    const key = deps.signingKeyRepo.findById(commit.signedKeyId);
    if (!key || key.deactivatedAt) {
      return reply.send({ valid: false, reason: 'key_deactivated' });
    }
    const msg = signedMessage({
      commitOid: oid,
      ref: commit.ref,
      approverId: commit.authorId,
      timestamp: commit.signedAt,
    });
    let valid = false;
    try {
      valid = verify(null, msg, loadPublicKey(key.publicKeyPem), Buffer.from(commit.signature, 'base64'));
    } catch {
      valid = false;
    }
    return reply.send({
      valid,
      reason: valid ? null : 'bad_signature',
      signer: key.label,
      keyId: key.id,
      fingerprint: key.fingerprint,
      signedAt: commit.signedAt,
    });
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/commits/:oid/verify',
    summary: 'Re-derive the signed payload and verify',
    tags: ['signing'],
  });

  app.post('/api/commits/_sign-helper', async (request, reply) => {
    const { commitOid, ref, approverId, timestamp } = request.body as {
      commitOid?: string; ref?: string; approverId?: string; timestamp?: string;
    };
    if (!commitOid || !ref || !approverId || !timestamp) {
      return reply.code(400).send({ error: { code: 'BAD_REQUEST', message: 'all fields required' } });
    }
    return reply.send({
      message: signedMessage({ commitOid, ref, approverId, timestamp }).toString('base64'),
      algorithm: 'ed25519',
    });
  });

  void SignCommitSchema; void DeactivateKeySchema;
}
