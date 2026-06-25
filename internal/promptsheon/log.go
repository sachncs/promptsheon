package promptsheon

import (
	"errors"
	"fmt"
	"sort"
)

// LogEntry is one row in the output of Log. The Timestamp is in
// nanoseconds since the Unix epoch so the CLI can format it
// however it likes.
type LogEntry struct {
	Hash      string
	Author    string
	Message   string
	Timestamp int64
	Parents   []string
	Telemetry []TelemetryKV
}

// Log returns up to n commits, newest first, reachable from the
// current HEAD. The traversal follows the first parent of each
// commit (the linear history) and visits merge commits only when
// they appear on that path. A repository with no commits returns
// (nil, nil) so callers can render the "no commits" UI without a
// separate "is empty" check.
func Log(n int) ([]*LogEntry, error) {
	if !IsInitialized() {
		return nil, ErrRepoNotInitialized
	}
	head, err := GetCurrentCommitHash()
	if err != nil {
		return nil, err
	}
	if head == "" {
		return nil, nil
	}

	visited := make(map[string]struct{})
	queue := []string{head}
	out := make([]*LogEntry, 0, n)

	for len(queue) > 0 && len(out) < n {
		hash := queue[0]
		queue = queue[1:]
		if _, seen := visited[hash]; seen {
			continue
		}
		visited[hash] = struct{}{}

		obj, err := ReadObject(hash)
		if err != nil {
			return nil, fmt.Errorf("log: read %s: %w", shortHash(hash), err)
		}
		if !obj.IsCommit() {
			return nil, fmt.Errorf("log: %s is not a commit object", shortHash(hash))
		}

		parents := append([]string(nil), obj.Parents...)
		entry := &LogEntry{
			Hash:      hash,
			Author:    obj.Author,
			Message:   obj.Message,
			Timestamp: obj.Timestamp,
			Parents:   parents,
			Telemetry: append([]TelemetryKV(nil), obj.Telemetry...),
		}
		out = append(out, entry)
		// Follow only the first parent for the linear log; the
		// merge commit is still recorded as we visit it.
		queue = append(queue, parents...)
	}
	return out, nil
}

// GraphNode is one commit in the visualisation produced by
// BuildGraph. The Branches field is the list of branch names that
// point directly at this commit (typically at most one); IsHEAD
// marks the commit HEAD currently resolves to.
type GraphNode struct {
	Hash     string
	Parents  []string
	Branches []string
	IsHEAD   bool
	Message  string
}

// BuildGraph returns the full commit DAG reachable from any
// branch, decorated with branch labels and the HEAD marker. The
// order is stable: parents appear before children so the CLI's
// graph renderer can lay out the columns without re-sorting.
//
// The traversal walks every branch's tip and follows all parents
// (not just the first), so merge commits are visited from every
// path. The total number of returned nodes is the number of
// distinct reachable commits.
func BuildGraph() ([]*GraphNode, error) {
	if !IsInitialized() {
		return nil, ErrRepoNotInitialized
	}

	refs, err := ListRefDetails()
	if err != nil {
		return nil, err
	}
	headHash, err := GetCurrentCommitHash()
	if err != nil {
		return nil, err
	}

	// Build the commit map and the reverse parent map in a single
	// pass so we can topologically order the commits later.
	commits := make(map[string]*Object)
	branchOf := make(map[string][]string) // hash → branch names
	for _, r := range refs {
		if r.Hash == "" {
			continue
		}
		branchOf[r.Hash] = append(branchOf[r.Hash], r.Name)
		queue := []string{r.Hash}
		for len(queue) > 0 {
			h := queue[0]
			queue = queue[1:]
			if _, seen := commits[h]; seen {
				continue
			}
			obj, err := ReadObject(h)
			if err != nil {
				return nil, fmt.Errorf("build graph: read %s: %w", shortHash(h), err)
			}
			if !obj.IsCommit() {
				return nil, fmt.Errorf("build graph: %s is not a commit", shortHash(h))
			}
			commits[h] = obj
			for _, p := range obj.Parents {
				queue = append(queue, p)
			}
		}
	}

	if len(commits) == 0 {
		return nil, nil
	}

	// Topological order: parents before children. We Kahn-sort by
	// indegree so the result is deterministic across runs.
	indeg := make(map[string]int)
	children := make(map[string][]string)
	for h, obj := range commits {
		if _, ok := indeg[h]; !ok {
			indeg[h] = 0
		}
		for _, p := range obj.Parents {
			indeg[h]++
			children[p] = append(children[p], h)
		}
	}

	var ready []string
	for h, d := range indeg {
		if d == 0 {
			ready = append(ready, h)
		}
	}
	sort.Strings(ready) // stable input order

	order := make([]string, 0, len(commits))
	for len(ready) > 0 {
		h := ready[0]
		ready = ready[1:]
		order = append(order, h)
		// Process children in sorted order so the result is stable
		// even when a commit has multiple children.
		kids := append([]string(nil), children[h]...)
		sort.Strings(kids)
		for _, c := range kids {
			indeg[c]--
			if indeg[c] == 0 {
				ready = append(ready, c)
			}
		}
	}

	nodes := make([]*GraphNode, 0, len(order))
	for _, h := range order {
		obj := commits[h]
		branches := append([]string(nil), branchOf[h]...)
		sort.Strings(branches)
		nodes = append(nodes, &GraphNode{
			Hash:     h,
			Parents:  append([]string(nil), obj.Parents...),
			Branches: branches,
			IsHEAD:   h == headHash,
			Message:  obj.Message,
		})
	}
	return nodes, nil
}

// errIsRefNotFound is a tiny helper to keep the call sites
// readable. It is duplicated here to avoid yet another tiny
// import dance in this small file.
func errIsRefNotFound(err error) bool {
	return errors.Is(err, ErrRefNotFound)
}
