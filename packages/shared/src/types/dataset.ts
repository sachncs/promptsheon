export interface Dataset {
  id: string;
  capabilityId: string;
  name: string;
  description: string;
  createdAt: string;
  updatedAt: string;
}

export interface DatasetCase {
  id: string;
  datasetId: string;
  seq: number;
  inputs: string;
  expected: string;
  description: string;
}
