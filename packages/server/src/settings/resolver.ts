import type { AppConfig } from '@promptsheon/shared';
import type { SystemConfigRepo } from '../repos/system-config.js';
import type { SettingsNotifier } from './notifier.js';

export class SettingsResolver {
  private cache = new Map<string, unknown>();
  private cacheExpiry = new Map<string, number>();
  private readonly CACHE_TTL_MS = 30_000;
  private notifier?: SettingsNotifier;

  constructor(
    private defaults: Record<string, unknown>,
    private envSource: Record<string, string>,
    private dbRepo: SystemConfigRepo,
  ) {}

  async get<T = unknown>(key: string): Promise<T> {
    const cached = this.cache.get(key);
    const expiry = this.cacheExpiry.get(key);
    if (cached !== undefined && expiry && Date.now() < expiry) {
      return cached as T;
    }

    const dbValue = this.dbRepo.get(key);
    if (dbValue && !dbValue.tombstone) {
      const parsed = JSON.parse(dbValue.value);
      this.setCache(key, parsed);
      return parsed as T;
    }

    const envKey = key.replace(/\./g, '_').toUpperCase();
    const envValue = this.envSource[`PROMPTSHEON_${envKey}`] ?? this.envSource[envKey];
    if (envValue !== undefined) {
      try {
        const parsed = JSON.parse(envValue);
        this.setCache(key, parsed);
        return parsed as T;
      } catch {
        this.setCache(key, envValue);
        return envValue as T;
      }
    }

    const defaultValue = this.defaults[key];
    if (defaultValue !== undefined) {
      return defaultValue as T;
    }

    return undefined as T;
  }

  async set(key: string, value: unknown, updatedBy?: string): Promise<void> {
    await this.dbRepo.set(key, JSON.stringify(value), updatedBy);
    this.invalidateCache(key);
    this.notifier?.notify(key);
  }

  async delete(key: string, updatedBy?: string): Promise<void> {
    await this.dbRepo.set(key, 'null', updatedBy, true);
    this.invalidateCache(key);
    this.notifier?.notify(key);
  }

  async list(prefix?: string): Promise<Array<{ key: string; value: unknown }>> {
    const configs = this.dbRepo.list(prefix);
    return configs
      .filter((c) => !c.tombstone)
      .map((c) => ({ key: c.key, value: JSON.parse(c.value) }));
  }

  async merge(updates: Record<string, unknown>, updatedBy?: string): Promise<void> {
    for (const [key, value] of Object.entries(updates)) {
      await this.set(key, value, updatedBy);
    }
  }

  setNotifier(notifier: SettingsNotifier): void {
    this.notifier = notifier;
  }

  private setCache(key: string, value: unknown): void {
    this.cache.set(key, value);
    this.cacheExpiry.set(key, Date.now() + this.CACHE_TTL_MS);
  }

  private invalidateCache(key: string): void {
    this.cache.delete(key);
    this.cacheExpiry.delete(key);
  }
}
