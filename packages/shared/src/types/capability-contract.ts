export interface CapabilityContract {
  capabilityId: string;
  blastRadius: string;
  successRubric: string;
  autoPromotable: boolean;
  inputSchema: string;
  outputSchema: string;
  sloMaxP95Ms: number;
  sloMinSuccess: number;
  sloMaxHallu: number;
  updatedAt: string;
}
