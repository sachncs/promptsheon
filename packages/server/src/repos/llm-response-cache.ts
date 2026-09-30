import type Database from 'better-sqlite3';

export interface PersistedLlmResponse {
  hash: string;
  prompt: string;
  model: string;
  temperature: number;
  provider: string;
  baseUrl?: string;
  content: string;
  promptTokens: number;
  completionTokens: number;
  costUsd: number;
  createdAt: string;
}

interface CacheRow {
  cache_hash: string;
  model: string;
  temperature: number;
  provider: string;
  base_url: string | null;
  content: string;
  prompt_tokens: number;
  completion_tokens: number;
  cost_usd: number;
  created_at: string;
}

function toResponse(row: CacheRow): PersistedLlmResponse {
  return {
    hash: row.cache_hash,
    prompt: '',
    model: row.model,
    temperature: row.temperature,
    provider: row.provider,
    ...(row.base_url === null ? {} : { baseUrl: row.base_url }),
    content: row.content,
    promptTokens: row.prompt_tokens,
    completionTokens: row.completion_tokens,
    costUsd: row.cost_usd,
    createdAt: row.created_at,
  };
}

/** SQLite persistence tier for the gateway's content-addressed response cache. */
export class LlmResponseCacheRepo {
  constructor(private readonly db: Database.Database) {}

  get(hash: string): PersistedLlmResponse | null {
    const row = this.db.prepare('SELECT * FROM llm_response_cache WHERE cache_hash = ?').get(hash) as CacheRow | undefined;
    return row ? toResponse(row) : null;
  }

  set(entry: PersistedLlmResponse): void {
    this.db.prepare(`
      INSERT INTO llm_response_cache
        (cache_hash, model, temperature, provider, base_url, content, prompt_tokens,
         completion_tokens, cost_usd, created_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
      ON CONFLICT(cache_hash) DO UPDATE SET
        model = excluded.model,
        temperature = excluded.temperature,
        provider = excluded.provider,
        base_url = excluded.base_url,
        content = excluded.content,
        prompt_tokens = excluded.prompt_tokens,
        completion_tokens = excluded.completion_tokens,
        cost_usd = excluded.cost_usd,
        created_at = excluded.created_at
    `).run(entry.hash, entry.model, entry.temperature, entry.provider, entry.baseUrl ?? null, entry.content, entry.promptTokens, entry.completionTokens, entry.costUsd, entry.createdAt);
  }

  trim(maxEntries: number): void {
    this.db.prepare(`
      DELETE FROM llm_response_cache
      WHERE cache_hash NOT IN (
        SELECT cache_hash FROM llm_response_cache ORDER BY created_at DESC LIMIT ?
      )
    `).run(maxEntries);
  }
}
