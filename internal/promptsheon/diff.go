package promptsheon

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// DiffResult is the structured output of DiffIntelligence. The
// CommitA and CommitB hashes identify the two sides of the diff
// and the TreeDiffs are per-entry comparisons.
type DiffResult struct {
	CommitA  string
	CommitB  string
	TreeDiffs []DiffEntry
}

// DiffEntry is one row of a DiffResult. Status is one of "added",
// "removed", "modified", or "metric" — the formatter upper-cases
// these for display.
type DiffEntry struct {
	Name        string
	Type        string
	Status      string
	OldHash     string
	NewHash     string
	TextDiff    string
	MetricDiffs []MetricDiff
}

// MetricDiff is a single numeric change inside a commit's
// telemetry. Delta is NewVal - OldVal; the formatter uses it to
// print "+0.05" style markers.
type MetricDiff struct {
	Key    string
	OldVal float64
	NewVal float64
	Delta  float64
}

// DiffIntelligence compares two commit hashes and returns the
// per-entry changes between their trees plus any telemetry
// changes recorded on the commits themselves.
//
// The function refuses to operate on non-commit inputs: a tree or
// blob hash returns ErrInvalidHash because the diff semantics
// only make sense at the commit level.
func DiffIntelligence(a, b string) (*DiffResult, error) {
	if !IsInitialized() {
		return nil, ErrRepoNotInitialized
	}
	cleanA := sanitizeHash(a)
	cleanB := sanitizeHash(b)
	if err := validateHash(cleanA); err != nil {
		return nil, fmt.Errorf("diff: a: %w", err)
	}
	if err := validateHash(cleanB); err != nil {
		return nil, fmt.Errorf("diff: b: %w", err)
	}

	objA, err := ReadObject(cleanA)
	if err != nil {
		return nil, fmt.Errorf("diff: read a: %w", err)
	}
	if !objA.IsCommit() {
		return nil, fmt.Errorf("%w: a is not a commit", ErrInvalidHash)
	}
	objB, err := ReadObject(cleanB)
	if err != nil {
		return nil, fmt.Errorf("diff: read b: %w", err)
	}
	if !objB.IsCommit() {
		return nil, fmt.Errorf("%w: b is not a commit", ErrInvalidHash)
	}

	result := &DiffResult{CommitA: cleanA, CommitB: cleanB}

	// Walk the trees recursively. We flatten the trees into a
	// name → (oldHash, newHash) map keyed by full path so
	// insertion, removal, and modification are easy to detect.
	oldEntries := flattenTree(objA.TreeHash, "")
	newEntries := flattenTree(objB.TreeHash, "")

	allNames := make(map[string]struct{}, len(oldEntries)+len(newEntries))
	for n := range oldEntries {
		allNames[n] = struct{}{}
	}
	for n := range newEntries {
		allNames[n] = struct{}{}
	}
	names := make([]string, 0, len(allNames))
	for n := range allNames {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		old, oldOK := oldEntries[name]
		new, newOK := newEntries[name]
		switch {
		case oldOK && !newOK:
			result.TreeDiffs = append(result.TreeDiffs, DiffEntry{
				Name:    name,
				Type:    "blob",
				Status:  "removed",
				OldHash: old.hash,
			})
		case !oldOK && newOK:
			result.TreeDiffs = append(result.TreeDiffs, DiffEntry{
				Name:    name,
				Type:    "blob",
				Status:  "added",
				NewHash: new.hash,
			})
		case old.hash != new.hash:
			textDiff := ""
			if old.kind == "blob" && new.kind == "blob" {
				textDiff = computeTextDiff(old.content, new.content, name)
			}
			result.TreeDiffs = append(result.TreeDiffs, DiffEntry{
				Name:     name,
				Type:     "blob",
				Status:   "modified",
				OldHash:  old.hash,
				NewHash:  new.hash,
				TextDiff: textDiff,
			})
		}
	}

	// Compare telemetry. We append a single metric entry per changed
	// key (rather than one entry per commit) so the formatter can
	// render it compactly.
	metricDiffs := diffTelemetry(objA.Telemetry, objB.Telemetry)
	if len(metricDiffs) > 0 {
		result.TreeDiffs = append(result.TreeDiffs, DiffEntry{
			Name:        "(metric)",
			Status:      "metric",
			MetricDiffs: metricDiffs,
		})
	}

	return result, nil
}

// treeEntry is a flattened view of one entry in a tree, with the
// content materialised so the diff engine can produce a text
// diff without re-reading the object.
type treeEntry struct {
	hash    string
	kind    string
	content string
}

// flattenTree walks the tree rooted at root and returns a map
// keyed by the full path of each leaf. Subtrees are recursed
// into; intermediate tree nodes are not emitted on their own.
func flattenTree(root, prefix string) map[string]treeEntry {
	out := make(map[string]treeEntry)
	if root == "" {
		return out
	}
	obj, err := ReadObject(root)
	if err != nil {
		return out
	}
	if !obj.IsTree() {
		return out
	}
	for _, e := range obj.Entries {
		path := e.Name
		if prefix != "" {
			path = prefix + "/" + e.Name
		}
		if e.Type == TypeTree {
			for k, v := range flattenTree(e.Hash, path) {
				out[k] = v
			}
			continue
		}
		child, err := ReadObject(e.Hash)
		if err != nil {
			continue
		}
		out[path] = treeEntry{
			hash:    e.Hash,
			kind:    "blob",
			content: child.Data,
		}
	}
	return out
}

// diffTelemetry returns the metric diffs for telemetry that
// changed between a and b. Telemetry is a []TelemetryKV sorted by
// key, so we can do a merge walk in O(n).
func diffTelemetry(a, b []TelemetryKV) []MetricDiff {
	index := make(map[string]int, len(b))
	for i, kv := range b {
		index[kv.Key] = i
	}
	visited := make(map[string]bool, len(b))
	var out []MetricDiff
	for _, kv := range a {
		j, ok := index[kv.Key]
		if !ok {
			continue
		}
		visited[kv.Key] = true
		oldVal, oldOK := toFloat64(kv.Value)
		newVal, newOK := toFloat64(b[j].Value)
		if !oldOK || !newOK {
			continue
		}
		if oldVal == newVal {
			continue
		}
		out = append(out, MetricDiff{
			Key:    kv.Key,
			OldVal: oldVal,
			NewVal: newVal,
			Delta:  newVal - oldVal,
		})
	}
	for _, kv := range b {
		if visited[kv.Key] {
			continue
		}
		newVal, newOK := toFloat64(kv.Value)
		if !newOK {
			continue
		}
		out = append(out, MetricDiff{
			Key:    kv.Key,
			NewVal: newVal,
			Delta:  newVal,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// FormatDiff renders a DiffResult as a human-readable string. The
// output is plain text with status words upper-cased so the CLI
// can print it directly to a terminal.
func FormatDiff(d *DiffResult) string {
	if d == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("Intelligence Diff\n")
	b.WriteString(fmt.Sprintf("  from %s\n", shortHash(d.CommitA)))
	b.WriteString(fmt.Sprintf("  to   %s\n", shortHash(d.CommitB)))
	b.WriteString("\n")
	for _, e := range d.TreeDiffs {
		status := strings.ToUpper(e.Status)
		switch e.Status {
		case "added":
			fmt.Fprintf(&b, "ADDED    %s  (%s)\n", e.Name, shortHash(e.NewHash))
		case "removed":
			fmt.Fprintf(&b, "REMOVED  %s  (%s)\n", e.Name, shortHash(e.OldHash))
		case "modified":
			fmt.Fprintf(&b, "MODIFIED %s  (%s -> %s)\n", e.Name, shortHash(e.OldHash), shortHash(e.NewHash))
			if e.TextDiff != "" {
				b.WriteString(e.TextDiff)
				if !strings.HasSuffix(e.TextDiff, "\n") {
					b.WriteString("\n")
				}
			}
		case "metric":
			fmt.Fprintf(&b, "METRIC   %s\n", e.Name)
			for _, m := range e.MetricDiffs {
				sign := "+"
				if m.Delta < 0 {
					sign = ""
				}
				fmt.Fprintf(&b, "           %s  %.4f -> %.4f (%s%.4f)\n", m.Key, m.OldVal, m.NewVal, sign, m.Delta)
			}
		default:
			fmt.Fprintf(&b, "%s %s\n", status, e.Name)
		}
	}
	return b.String()
}

// computeTextDiff produces a tiny unified-diff-style rendering of
// the change from old to new. The implementation is a standard LCS
// walk: it finds the longest common subsequence of lines, then
// emits unchanged lines as context and inserted/removed lines with
// +/- markers. A short header summarises the file.
//
// The result always starts with a header line so callers can rely
// on non-empty output even for empty inputs.
func computeTextDiff(oldText, newText, name string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s (old)\n", name)
	fmt.Fprintf(&b, "+++ %s (new)\n", name)

	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	lcs := lcsTable(oldLines, newLines)

	// Walk the LCS table to render the diff. We use the standard
	// backtrack: start at (m, n) and emit in reverse, then
	// reverse the output buffer at the end.
	type op struct {
		kind byte // ' ', '-', '+'
		text string
	}
	var ops []op
	i, j := len(oldLines), len(newLines)
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && oldLines[i-1] == newLines[j-1]:
			ops = append(ops, op{' ', oldLines[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]):
			ops = append(ops, op{'+', newLines[j-1]})
			j--
		case i > 0:
			ops = append(ops, op{'-', oldLines[i-1]})
			i--
		}
	}
	// Reverse into the output.
	for k := len(ops) - 1; k >= 0; k-- {
		o := ops[k]
		if o.kind == ' ' {
			b.WriteString(" ")
			b.WriteString(o.text)
		} else {
			b.WriteByte(o.kind)
			b.WriteString(o.text)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// splitLines splits s on '\n'. Trailing newlines do not produce
// empty trailing entries; this matches the line-by-line mental
// model most diff tools use.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if strings.HasSuffix(s, "\n") {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// lcsTable returns the standard m+1 × n+1 LCS length table for
// the line sequences a and b. Memory is O(m*n); for the prompt
// sizes promptsheon stores this is fine.
func lcsTable(a, b []string) [][]int {
	m, n := len(a), len(b)
	t := make([][]int, m+1)
	for i := range t {
		t[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				t[i][j] = t[i-1][j-1] + 1
			} else if t[i-1][j] >= t[i][j-1] {
				t[i][j] = t[i-1][j]
			} else {
				t[i][j] = t[i][j-1]
			}
		}
	}
	return t
}

// toFloat64 coerces a telemetry value to a float64. The function
// returns (0, false) for non-numeric inputs so the diff engine
// can silently skip them; callers that need stricter validation
// should re-parse the value themselves.
func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case string:
		f, err := parseFloatString(x)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// parseFloatString parses a numeric string. We accept decimal
// floats and integers; scientific notation is supported because
// it is what json.Number produces for large numbers.
func parseFloatString(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	// Quick reject for things json.Number would never produce.
	for _, r := range s {
		if r == '-' || r == '+' || r == '.' || r == 'e' || r == 'E' {
			continue
		}
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-numeric")
		}
	}
	f, err := parseFloat64(s)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("non-finite")
	}
	return f, nil
}

// parseFloat64 is a thin wrapper around strconv.ParseFloat that
// exists so we can substitute a deterministic parser in tests.
var parseFloat64 = func(s string) (float64, error) {
	return jsonNumberParse(s)
}

// jsonNumberParse uses json.Number to parse a string. We round-
// trip through JSON to keep the implementation tiny and to reuse
// the well-tested encoding/json parser.
func jsonNumberParse(s string) (float64, error) {
	var v any
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return 0, err
	}
	if n, ok := v.(json.Number); ok {
		return n.Float64()
	}
	return 0, fmt.Errorf("not a number")
}
