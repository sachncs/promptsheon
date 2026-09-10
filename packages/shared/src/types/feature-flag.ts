export interface FeatureFlag {
  name: string;
  enabled: boolean;
  description: string;
  value: unknown;
  updatedAt: string;
}
