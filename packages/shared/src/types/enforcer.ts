export interface EnforcerState {
  workspaceId: string;
  kind: 'budget' | 'quota';
  payload: string;
  updatedAt: string;
}
