package promptsheon

import (
	"sort"
)

// ObjectType identifies the kind of object stored in the CAS.
type ObjectType string

// Object type tags. The CLI displays these to humans, the binary protocol
// stores them as JSON, and the diff engine branches on them. Keep them
// lowercase ASCII to round-trip through the JSON serializer unchanged.
const (
	TypeBlob   ObjectType = "blob"
	TypeTree   ObjectType = "tree"
	TypeCommit ObjectType = "commit"
)

// TelemetryKV is a single key/value pair attached to a commit. The
// canonical type is a map[string]any in the public API but the
// serialized form is a slice so the order is stable across
// representations.
type TelemetryKV struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// TreeEntry is one row in a tree object. The Name is unique within a
// tree; Type and Hash point at the object the row names.
type TreeEntry struct {
	Name string     `json:"name"`
	Type ObjectType `json:"type"`
	Hash string     `json:"hash"`
}

// Object is the union type for blob, tree, and commit objects. The
// fields used depend on the object's kind:
//
//   - Blob:   Data is set; everything else is zero.
//   - Tree:   Entries is set; everything else is zero.
//   - Commit: TreeHash, Parents, Author, Message, Timestamp, and
//     Telemetry are set; Data and Entries are zero.
//
// The kind is inferred from which fields are populated, not stored
// explicitly. This keeps the JSON shape tiny and matches how the
// CLI and the diff engine already inspect the struct.
type Object struct {
	Data      string        `json:"data,omitempty"`
	Entries   []TreeEntry   `json:"entries,omitempty"`
	TreeHash  string        `json:"tree_hash,omitempty"`
	Parents   []string      `json:"parents,omitempty"`
	Author    string        `json:"author,omitempty"`
	Message   string        `json:"message,omitempty"`
	Timestamp int64         `json:"timestamp,omitempty"`
	Telemetry []TelemetryKV `json:"telemetry,omitempty"`
}

// NewBlobObject returns a fresh Object holding raw blob content.
func NewBlobObject(data string) *Object {
	return &Object{Data: data}
}

// NewTreeObject returns a fresh Object whose entries are sorted by
// name. Sorting at construction time guarantees the hash is
// independent of the order callers pass entries in, which is the
// property the determinism test relies on.
func NewTreeObject(entries []TreeEntry) *Object {
	sorted := make([]TreeEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return &Object{Entries: sorted}
}

// NewCommitObject returns a fresh commit object. Parents are sorted
// lexicographically so the resulting hash is stable regardless of
// the order callers pass them in. A nil or empty telemetry map
// produces a zero-length slice; both round-trip through JSON to the
// same bytes.
func NewCommitObject(treeHash string, parents []string, author, message string, telemetry map[string]any) *Object {
	sortedParents := append([]string(nil), parents...)
	sort.Strings(sortedParents)

	var tel []TelemetryKV
	if len(telemetry) > 0 {
		keys := make([]string, 0, len(telemetry))
		for k := range telemetry {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		tel = make([]TelemetryKV, 0, len(keys))
		for _, k := range keys {
			tel = append(tel, TelemetryKV{Key: k, Value: telemetry[k]})
		}
	}

	return &Object{
		TreeHash:  treeHash,
		Parents:   sortedParents,
		Author:    author,
		Message:   message,
		Timestamp: nowUnixNano(),
		Telemetry: tel,
	}
}

// Type returns the kind of object stored in o. The kind is inferred
// from which fields are populated; the order of checks matters
// because a commit has a TreeHash but no Data, while a blob has
// Data but no TreeHash.
func (o *Object) Type() ObjectType {
	switch {
	case o.TreeHash != "":
		return TypeCommit
	case len(o.Entries) > 0:
		return TypeTree
	default:
		return TypeBlob
	}
}

// IsBlob reports whether o is a blob.
func (o *Object) IsBlob() bool { return o.Type() == TypeBlob }

// IsTree reports whether o is a tree.
func (o *Object) IsTree() bool { return o.Type() == TypeTree }

// IsCommit reports whether o is a commit.
func (o *Object) IsCommit() bool { return o.Type() == TypeCommit }

// shortHash returns the first 12 hex characters of hash, the
// standard short form used by Git-style output. The caller is
// responsible for ensuring hash is at least 12 characters long.
func shortHash(hash string) string {
	if len(hash) < 12 {
		return hash
	}
	return hash[:12]
}
