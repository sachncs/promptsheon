/**
 * Tree + Blob types — content-addressed units of a repository.
 *
 * Trees are content-addressed mappings of POSIX paths to blob oids.
 * Blobs are the raw bytes of a file. A blob oid is sha256(content);
 * a tree oid is sha256(canonicalJson({path: blob_oid})).
 */

export interface BlobRef {
  oid: string;
  size: number;
}

export interface RepoTreeEntry {
  path: string;
  blob: BlobRef;
}

export interface Tree {
  oid: string;
  entries: RepoTreeEntry[];
}

export interface CommitRequest {
  repositoryId: string;
  ref: string;
  message: string;
  authorId: string;
  parents?: string[];
}
