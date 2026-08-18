export interface Migration {
  version: number;
  name: string;
  up: string;
  down?: string;
}

export const MIGRATIONS: Migration[] = [
  { version: 0, name: '000_down', down: '' },
  { version: 1, name: '001_core_schema' },
  { version: 2, name: '002_audit_chain' },
  { version: 3, name: '003_indexes' },
  { version: 4, name: '004_data_cleanup' },
  { version: 5, name: '005_seed' },
  { version: 6, name: '006_security' },
  { version: 7, name: '007_views_and_triggers' },
  { version: 8, name: '008_destructive_cleanup' },
  { version: 9, name: '009_vault_state' },
  { version: 10, name: '010_ws_state' },
  { version: 11, name: '011_audit_archive' },
  { version: 12, name: '012_enforcer_state' },
  { version: 13, name: '013_idempotency_cache' },
  { version: 14, name: '014_system_config' },
  { version: 15, name: '015_seed_settings' },
  { version: 16, name: '016_bandit_arm_counters' },
  { version: 17, name: '017_system_config_crdt' },
  { version: 18, name: '018_capability_contract' },
  { version: 19, name: '019_self_evolve' },
  { version: 20, name: '020_audit_chain_state_immutability' },
  { version: 21, name: '021_canary' },
];
