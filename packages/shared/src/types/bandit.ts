export interface BanditArm {
  id: string;
  armId: string;
  capabilityId: string;
  name: string;
  weight: number;
  pulls: number;
  wins: number;
  createdAt: string;
  updatedAt: string;
}

export interface BanditState {
  id: string;
  capabilityId: string;
  algorithm: string;
  arms: BanditArm[];
  createdAt: string;
  updatedAt: string;
}
