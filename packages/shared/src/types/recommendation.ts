export interface Recommendation {
  id: string;
  capabilityVersionId: string;
  type: string;
  payload: string;
  createdAt: string;
}

export interface Decision {
  id: string;
  recommendationId: string;
  payload: string;
  createdAt: string;
}
