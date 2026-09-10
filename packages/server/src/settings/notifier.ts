type Callback = (key: string) => void;

export class SettingsNotifier {
  private listeners = new Set<Callback>();

  subscribe(callback: Callback): () => void {
    this.listeners.add(callback);
    return () => {
      this.listeners.delete(callback);
    };
  }

  notify(key: string): void {
    for (const cb of this.listeners) {
      try {
        cb(key);
      } catch (e) {
        console.error('SettingsNotifier callback error:', e);
      }
    }
  }
}
