/**
 * Tag — an immutable pointer (frozen commit oid) within a
 * repository. Tags name releases and act as anchors for
 * provenance / rollback.
 */

export interface Tag {
  id: string;
  repositoryId: string;
  name: string;
  commitOid: string;
  message: string | null;
  taggerId: string;
  createdAt: string;
}

export interface TagCreateInput {
  repositoryId: string;
  name: string;
  commitOid: string;
  message?: string | null;
  taggerId: string;
}
