export interface Org {
  id: string;
  name: string;
  slug: string;
  createdAt: string;
  updatedAt: string;
}

export interface Team {
  id: string;
  orgId: string;
  name: string;
  createdAt: string;
}

export type OrgRole = 'admin' | 'approver' | 'editor' | 'viewer';

export interface OrgMember {
  orgId: string;
  userId: string;
  role: OrgRole;
  joinedAt: string;
}

export interface TeamMember {
  teamId: string;
  userId: string;
  joinedAt: string;
}