import type { AppConfig } from '@promptsheon/shared';
import type { MembershipRepo } from '../repos/org.js';
import type { UserRepo } from '../repos/user.js';
import type { VaultRepo } from '../repos/vault.js';
import type { SettingsResolver } from '../settings/resolver.js';

export type LlmProvider = 'openai' | 'anthropic' | 'bedrock' | 'custom';

export interface LlmSettingsInput {
  provider: LlmProvider;
  model: string;
  apiKey?: string;
  bedrock?: { region: string; accessKeyId: string; secretAccessKey: string };
  baseUrl?: string;
}

interface SecretDefinition {
  settingKey: string;
  vaultName: string;
  environmentKey: string;
}

const SECRET_DEFINITIONS: Record<string, SecretDefinition> = {
  'llm.openaiApiKey': { settingKey: 'llm.openaiApiKey', vaultName: 'llm-openai-api-key', environmentKey: 'OPENAI_API_KEY' },
  'llm.anthropicApiKey': { settingKey: 'llm.anthropicApiKey', vaultName: 'llm-anthropic-api-key', environmentKey: 'ANTHROPIC_API_KEY' },
  'llm.customApiKey': { settingKey: 'llm.customApiKey', vaultName: 'llm-custom-api-key', environmentKey: 'LLM_CUSTOM_KEY' },
  'llm.bedrockAccessKeyId': { settingKey: 'llm.bedrockAccessKeyId', vaultName: 'llm-bedrock-access-key-id', environmentKey: 'AWS_ACCESS_KEY_ID' },
  'llm.bedrockSecretAccessKey': { settingKey: 'llm.bedrockSecretAccessKey', vaultName: 'llm-bedrock-secret-access-key', environmentKey: 'AWS_SECRET_ACCESS_KEY' },
};

/** Owns durable provider configuration and keeps credentials out of system_config. */
export class LlmSettingsService {
  constructor(
    private readonly settings: SettingsResolver,
    private readonly vault: VaultRepo,
    private readonly users: UserRepo,
    private readonly memberships: MembershipRepo,
  ) {}

  async save(input: LlmSettingsInput, updatedBy = 'bootstrap'): Promise<void> {
    const orgId = this.adminOrganizationId();
    if (!orgId) throw new Error('an administrator organization is required before configuring an LLM');
    const actorId = this.adminUserId() ?? updatedBy;
    await this.settings.set('llm.provider', input.provider, actorId);
    await this.settings.set('llm.model', input.model, actorId);

    if (input.provider === 'openai' && input.apiKey) {
      await this.saveSecret('llm.openaiApiKey', input.apiKey, orgId, actorId);
    } else if (input.provider === 'anthropic' && input.apiKey) {
      await this.saveSecret('llm.anthropicApiKey', input.apiKey, orgId, actorId);
    } else if (input.provider === 'custom' && input.apiKey && input.baseUrl) {
      await this.saveSecret('llm.customApiKey', input.apiKey, orgId, actorId);
      await this.settings.set('llm.baseUrl', input.baseUrl, actorId);
    } else if (input.provider === 'bedrock' && input.bedrock) {
      await this.saveSecret('llm.bedrockAccessKeyId', input.bedrock.accessKeyId, orgId, actorId);
      await this.saveSecret('llm.bedrockSecretAccessKey', input.bedrock.secretAccessKey, orgId, actorId);
      await this.settings.set('llm.bedrockRegion', input.bedrock.region, actorId);
    }
    await this.hydrateEnvironment();
  }

  async hydrateConfig(config: AppConfig): Promise<void> {
    const provider = await this.settings.get<string>('llm.provider');
    const model = await this.settings.get<string>('llm.model');
    if (provider) config.llm.defaultProvider = provider;
    if (model) config.llm.defaultModel = model;
    const baseUrl = await this.settings.get<string>('llm.baseUrl');
    if (baseUrl) config.llm.baseUrl = baseUrl;
    await this.hydrateEnvironment();
  }

  async hasCredentials(provider: string | undefined): Promise<boolean> {
    if (!provider) return false;
    if (provider === 'openai') return Boolean(await this.readSecret('llm.openaiApiKey'));
    if (provider === 'anthropic') return Boolean(await this.readSecret('llm.anthropicApiKey'));
    if (provider === 'custom') return Boolean(await this.readSecret('llm.customApiKey'));
    if (provider === 'bedrock') {
      return Boolean(await this.readSecret('llm.bedrockAccessKeyId')) && Boolean(await this.readSecret('llm.bedrockSecretAccessKey'));
    }
    return false;
  }

  private async hydrateEnvironment(): Promise<void> {
    for (const definition of Object.values(SECRET_DEFINITIONS)) {
      const value = await this.readSecret(definition.settingKey);
      if (value) process.env[definition.environmentKey] = value;
    }
    const region = await this.settings.get<string>('llm.bedrockRegion');
    if (region) process.env['AWS_DEFAULT_REGION'] = region;
    const baseUrl = await this.settings.get<string>('llm.baseUrl');
    if (baseUrl) process.env['LLM_BASE_URL'] = baseUrl;
  }

  private async saveSecret(settingKey: string, value: string, orgId: string, updatedBy: string): Promise<void> {
    const definition = SECRET_DEFINITIONS[settingKey];
    if (!definition) throw new Error(`unknown LLM secret setting: ${settingKey}`);
    this.vault.set(orgId, definition.vaultName, value, updatedBy);
    await this.settings.set(settingKey, `vault://${orgId}/${definition.vaultName}`, updatedBy);
  }

  private async readSecret(settingKey: string): Promise<string | null> {
    const definition = SECRET_DEFINITIONS[settingKey];
    if (!definition) return null;
    const stored = await this.settings.get<string>(settingKey);
    if (!stored) return null;
    const reference = /^vault:\/\/([^/]+)\/(.+)$/.exec(stored);
    if (reference) return this.vault.resolve(reference[1]!, reference[2]!);

    // Migrate credentials written by pre-vault versions on first startup.
    const orgId = this.adminOrganizationId();
    if (!orgId) return stored;
    this.vault.set(orgId, definition.vaultName, stored, this.adminUserId() ?? 'settings-migration');
    await this.settings.set(settingKey, `vault://${orgId}/${definition.vaultName}`, this.adminUserId() ?? 'settings-migration');
    return stored;
  }

  private adminOrganizationId(): string | null {
    const admin = this.adminUserId();
    return admin ? this.memberships.findOrgsForUser(admin)[0] ?? null : null;
  }

  private adminUserId(): string | null {
    return this.users.list().find((user) => user.role === 'admin')?.id ?? null;
  }
}
