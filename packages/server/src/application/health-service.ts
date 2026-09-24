/** Infrastructure probe required by liveness and readiness checks. */
export interface HealthProbe {
  ping(): boolean;
  quickCheck(): boolean;
}

/** Application boundary for database health and readiness decisions. */
export class HealthService {
  constructor(private readonly probe: HealthProbe) {}

  isHealthy(): boolean {
    return this.probe.ping();
  }

  isReady(): boolean {
    return this.probe.quickCheck();
  }
}
