package main

// cas.go: the promptsheon CLI's git-style content-addressable storage
// commands. These predate the v0.1.0 capability/release model and are
// retained for backwards compatibility with v0.0.x workflows. The
// capability/release commands live in capability.go and release.go.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	promptsheoncas "github.com/sachncs/promptsheon/pkg/cas"
)

func cmdInit() error {
	if promptsheoncas.IsInitialized() {
		return fmt.Errorf("repository already initialized")
	}
	if err := promptsheoncas.Init(); err != nil {
		return err
	}
	fmt.Println("initialized empty promptsheon repository in .promptsheon/")
	return nil
}

func cmdHashObject(args []string) error {
	if len(args) < 1 {
		return usageErrorf("promptsheon hash-object <data>")
	}
	data := strings.Join(args, " ")
	obj := promptsheoncas.NewBlobObject(data)
	hash, err := promptsheoncas.ObjectHash(obj)
	if err != nil {
		return fmt.Errorf("compute hash: %w", err)
	}
	fmt.Println(hash)
	return nil
}

func cmdWriteObject(args []string) error {
	if len(args) < 1 {
		return usageErrorf("promptsheon write-object <data>")
	}
	data := strings.Join(args, " ")
	obj := promptsheoncas.NewBlobObject(data)
	hash, err := promptsheoncas.WriteObject(obj)
	if err != nil {
		return err
	}
	fmt.Println(hash)
	return nil
}

func cmdReadObject(args []string) error {
	if len(args) < 1 {
		return usageErrorf("promptsheon read-object <hash>")
	}
	obj, err := promptsheoncas.ReadObject(args[0])
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal object: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func cmdCommit(args []string) error {
	if len(args) < 1 {
		return usageErrorf("promptsheon commit <tree-hash> [message]")
	}
	treeHash := args[0]
	message := "commit"
	if len(args) > 1 {
		message = strings.Join(args[1:], " ")
	}

	author := os.Getenv("PROMPTSHEON_AUTHOR")
	if author == "" {
		author = "promptsheon-user"
	}

	var parentHashes []string
	currentHash, err := promptsheoncas.GetCurrentCommitHash()
	if err == nil && currentHash != "" {
		parentHashes = []string{currentHash}
	}

	telemetry, telErr := parseTelemetry()

	result, err := promptsheoncas.Commit(treeHash, parentHashes, author, message, telemetry)
	if err != nil {
		return err
	}
	if telErr != nil {
		fmt.Fprintf(os.Stderr, "warning: telemetry ignored: %v\n", telErr)
	}

	fmt.Printf("[%s", result.Hash[:12])
	if result.Ref != "" {
		fmt.Printf(" %s", result.Ref)
	}
	fmt.Printf("] %s\n", message)
	return nil
}

func parseTelemetry() (map[string]any, error) {
	tel := os.Getenv("PROMPTSHEON_TELEMETRY")
	if tel == "" {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(tel), &m); err != nil {
		return nil, fmt.Errorf("PROMPTSHEON_TELEMETRY: %w", err)
	}
	return m, nil
}

func cmdLog(args []string) error {
	count := 30
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
			count = n
		}
	}

	entries, err := promptsheoncas.Log(count)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("no commits")
		return nil
	}

	ref, _ := promptsheoncas.GetCurrentRef()

	for i, e := range entries {
		refMarker := ""
		if i == 0 && ref != "" {
			refMarker = fmt.Sprintf(" (HEAD -> %s)", ref)
		} else if i == 0 {
			refMarker = " (HEAD)"
		}
		fmt.Printf("commit %s%s\n", e.Hash, refMarker)
		fmt.Printf("Author: %s\n", e.Author)
		fmt.Printf("Date:   %d\n\n", e.Timestamp/1_000_000_000)
		fmt.Printf("    %s\n", e.Message)

		if len(e.Telemetry) > 0 {
			fmt.Println()
			for _, kv := range e.Telemetry {
				fmt.Printf("    [telemetry] %s: %v\n", kv.Key, kv.Value)
			}
		}

		if i < len(entries)-1 {
			fmt.Println()
		}
	}
	return nil
}

func cmdCheckout(args []string) error {
	if len(args) < 1 {
		return usageErrorf("promptsheon checkout <ref|hash>")
	}
	if err := promptsheoncas.Checkout(args[0]); err != nil {
		return err
	}

	head, _ := promptsheoncas.ReadHEAD()
	if promptsheoncas.IsHEADDetached(head) {
		fmt.Printf("HEAD is now at %s\n", head[:12])
	} else {
		fmt.Printf("switched to branch '%s'\n", promptsheoncas.HEADRefName(head))
	}
	return nil
}

func cmdBranch(args []string) error {
	if len(args) == 0 {
		refs, err := promptsheoncas.ListRefs()
		if err != nil {
			return err
		}
		sort.Strings(refs)
		currentRef, _ := promptsheoncas.GetCurrentRef()
		for _, r := range refs {
			hash, _ := promptsheoncas.ReadRef(r)
			prefix := "  "
			if r == currentRef {
				prefix = "* "
			}
			shortHash := ""
			if len(hash) >= 12 {
				shortHash = hash[:12]
			}
			fmt.Printf("%s%s  %s\n", prefix, r, shortHash)
		}
		return nil
	}

	name := args[0]
	targetHash := ""
	if len(args) > 1 {
		targetHash = args[1]
	}

	if err := promptsheoncas.CreateBranch(name, targetHash); err != nil {
		return err
	}
	fmt.Printf("created branch '%s'\n", name)
	return nil
}

func cmdDeleteBranch(args []string) error {
	if len(args) < 1 {
		return usageErrorf("promptsheon delete-branch <name>")
	}
	if err := promptsheoncas.DeleteBranch(args[0]); err != nil {
		return err
	}
	fmt.Printf("deleted branch '%s'\n", args[0])
	return nil
}

func cmdDiff(args []string) error {
	if len(args) < 2 {
		return usageErrorf("promptsheon diff <hashA> <hashB>")
	}
	result, err := promptsheoncas.DiffIntelligence(args[0], args[1])
	if err != nil {
		return err
	}
	fmt.Print(promptsheoncas.FormatDiff(result))
	return nil
}

func cmdStatus() error {
	if !promptsheoncas.IsInitialized() {
		return fmt.Errorf("not a promptsheon repository")
	}

	ref, err := promptsheoncas.GetCurrentRef()
	if err != nil {
		return fmt.Errorf("read ref: %w", err)
	}
	headHash, err := promptsheoncas.GetCurrentCommitHash()
	if err != nil {
		return fmt.Errorf("read HEAD: %w", err)
	}
	refs, err := promptsheoncas.ListRefs()
	if err != nil {
		return fmt.Errorf("list refs: %w", err)
	}

	headLine := ""
	switch {
	case headHash == "":
		headLine = "no commits yet"
	case ref == "":
		headLine = fmt.Sprintf("detached HEAD at %s", headHash[:12])
	default:
		headLine = fmt.Sprintf("on branch %s", ref)
	}
	fmt.Printf("HEAD: %s\n", headLine)

	if headHash != "" {
		obj, e := promptsheoncas.ReadObject(headHash)
		if e == nil {
			ts := time.Unix(0, obj.Timestamp)
			fmt.Printf("last commit: %s (%s)\n", headHash[:12], ts.Format(time.RFC3339))
			if obj.Message != "" {
				fmt.Printf("  %s\n", obj.Message)
			}
		}
	}

	fmt.Printf("branches: %d\n", len(refs))
	if len(refs) > 0 {
		for _, r := range refs {
			prefix := "  "
			if r == ref {
				prefix = " *"
			}
			h, _ := promptsheoncas.ReadRef(r)
			shortHash := ""
			if len(h) >= 12 {
				shortHash = h[:12]
			}
			fmt.Printf("%s %s  %s\n", prefix, r, shortHash)
		}
	}

	stats, err := promptsheoncas.GetStats()
	if err == nil {
		fmt.Printf("objects: %d (blobs: %d, trees: %d, commits: %d)\n",
			stats.TotalObjects, stats.BlobCount, stats.TreeCount, stats.CommitCount)
	}

	return nil
}

func cmdShow(args []string) error {
	if len(args) < 1 {
		return usageErrorf("promptsheon show <hash>")
	}
	hash := args[0]

	obj, err := promptsheoncas.ReadObject(hash)
	if err != nil {
		return err
	}

	switch {
	case obj.Data != "":
		fmt.Printf("blob %s\n\n", hash[:12])
		fmt.Println(obj.Data)

	case len(obj.Entries) > 0:
		fmt.Printf("tree %s\n\n", hash[:12])
		fmt.Printf("%-12s %-6s  %s\n", "hash", "type", "name")
		fmt.Println(strings.Repeat("-", 50))
		for _, e := range obj.Entries {
			fmt.Printf("%-12s %-6s  %s\n", e.Hash[:12], e.Type, e.Name)
		}

	case obj.TreeHash != "":
		fmt.Printf("commit %s\n", hash[:12])
		fmt.Printf("tree:    %s\n", obj.TreeHash[:12])
		if len(obj.Parents) > 0 {
			for i, p := range obj.Parents {
				label := "parent:"
				if i > 0 {
					label = "       "
				}
				fmt.Printf("%s %s\n", label, p[:12])
			}
		} else {
			fmt.Println("parent:  (root commit)")
		}
		fmt.Printf("author:  %s\n", obj.Author)
		ts := time.Unix(0, obj.Timestamp)
		fmt.Printf("date:    %s\n", ts.Format(time.RFC3339))
		fmt.Println()
		fmt.Printf("    %s\n", obj.Message)
		if len(obj.Telemetry) > 0 {
			fmt.Println()
			for _, kv := range obj.Telemetry {
				fmt.Printf("    [telemetry] %s: %v\n", kv.Key, kv.Value)
			}
		}

		fmt.Println()
		fmt.Println("Tree contents:")
		treeObj, err := promptsheoncas.ReadObject(obj.TreeHash)
		if err == nil {
			for _, e := range treeObj.Entries {
				fmt.Printf("  %-6s  %-12s  %s\n", e.Type, e.Hash[:12], e.Name)
			}
		}

	default:
		fmt.Printf("object %s (empty/unknown)\n", hash[:12])
	}

	return nil
}

func cmdLsTree(args []string) error {
	if len(args) < 1 {
		return usageErrorf("promptsheon ls-tree <hash>")
	}
	hash := args[0]

	obj, err := promptsheoncas.ReadObject(hash)
	if err != nil {
		return err
	}
	if len(obj.Entries) == 0 {
		return fmt.Errorf("object %s is not a tree", hash[:12])
	}

	var walkTree func(hash string, prefix string) error
	walkTree = func(hash string, prefix string) error {
		obj, err := promptsheoncas.ReadObject(hash)
		if err != nil {
			return err
		}
		for _, e := range obj.Entries {
			path := e.Name
			if prefix != "" {
				path = prefix + "/" + e.Name
			}
			fmt.Printf("%-6s %s  %s\n", e.Type, e.Hash[:12], path)
			if e.Type == promptsheoncas.TypeTree {
				if err := walkTree(e.Hash, path); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return walkTree(hash, "")
}

func cmdCatFile(args []string) error {
	if len(args) < 1 {
		return usageErrorf("promptsheon cat-file <hash>")
	}
	obj, err := promptsheoncas.ReadObject(args[0])
	if err != nil {
		return err
	}
	if obj.Data == "" {
		return fmt.Errorf("object %s is not a blob", args[0][:12])
	}
	_, err = fmt.Print(obj.Data)
	if !strings.HasSuffix(obj.Data, "\n") {
		fmt.Println()
	}
	return err
}

func cmdGraph() error {
	nodes, err := promptsheoncas.BuildGraph()
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		fmt.Println("no commits")
		return nil
	}

	refs, _ := promptsheoncas.ListRefDetails()
	headRef, _ := promptsheoncas.GetCurrentRef()

	columns := assignGraphColumns(nodes, refs, headRef)
	maxCol := 0
	for _, c := range columns {
		if c > maxCol {
			maxCol = c
		}
	}

	fmt.Println(renderGraph(nodes, columns, maxCol))
	return nil
}

func assignGraphColumns(nodes []*promptsheoncas.GraphNode, refs []*promptsheoncas.RefDetail, headRef string) map[string]int {
	columns := make(map[string]int)
	nodeMap := make(map[string]*promptsheoncas.GraphNode)
	for _, n := range nodes {
		nodeMap[n.Hash] = n
	}

	for _, ref := range refs {
		if ref.Name == headRef && ref.Hash != "" {
			walkChain(ref.Hash, 0, columns, nodeMap)
			break
		}
	}

	if len(columns) == 0 && len(nodes) > 0 {
		for _, n := range nodes {
			if n.IsHEAD {
				walkChain(n.Hash, 0, columns, nodeMap)
				break
			}
		}
	}

	col := 1
	for _, ref := range refs {
		if ref.Name == headRef || ref.Hash == "" {
			continue
		}
		walkChain(ref.Hash, col, columns, nodeMap)
		col++
	}

	for _, n := range nodes {
		if _, ok := columns[n.Hash]; !ok {
			columns[n.Hash] = 0
		}
	}

	return columns
}

func walkChain(hash string, col int, columns map[string]int, nodeMap map[string]*promptsheoncas.GraphNode) {
	for hash != "" {
		if _, assigned := columns[hash]; assigned {
			return
		}
		columns[hash] = col
		node, ok := nodeMap[hash]
		if !ok || len(node.Parents) == 0 {
			return
		}
		hash = node.Parents[0]
	}
}

func renderGraph(nodes []*promptsheoncas.GraphNode, columns map[string]int, maxCol int) string {
	var buf bytes.Buffer

	colRemaining := make([]int, maxCol+1)
	for _, n := range nodes {
		colRemaining[columns[n.Hash]]++
	}

	for i, node := range nodes {
		col := columns[node.Hash]
		colRemaining[col]--

		prevCol := -1
		if i > 0 {
			prevCol = columns[nodes[i-1].Hash]
		}

		if prevCol >= 0 && prevCol != col {
			drawConnLines(&buf, prevCol, col, maxCol, colRemaining, node)
		}

		for c := 0; c <= maxCol; c++ {
			switch {
			case c == col:
				buf.WriteString("o ")
			case colRemaining[c] > 0:
				buf.WriteString("| ")
			default:
				buf.WriteString("  ")
			}
		}

		buf.WriteString(node.Hash[:12])

		var labels []string
		if node.IsHEAD {
			if len(node.Branches) > 0 {
				labels = append(labels, "HEAD -> "+node.Branches[0])
				labels = append(labels, node.Branches[1:]...)
			} else {
				labels = append(labels, "HEAD")
			}
		} else {
			labels = append(labels, node.Branches...)
		}
		if len(labels) > 0 {
			buf.WriteString(" (")
			buf.WriteString(strings.Join(labels, ", "))
			buf.WriteString(")")
		}

		if node.Message != "" {
			buf.WriteString(" ")
			buf.WriteString(node.Message)
		}
		buf.WriteString("\n")

		if i == 0 || col != columns[nodes[i-1].Hash] {
			drawMergeForkLines(&buf, node, columns, maxCol, colRemaining)
		}
	}

	return buf.String()
}

func drawConnLines(buf *bytes.Buffer, prevCol, col, maxCol int, colRemaining []int, _ *promptsheoncas.GraphNode) {
	for c := 0; c <= maxCol; c++ {
		switch {
		case c == prevCol && c < col:
			buf.WriteString("| ")
		case c == prevCol && c > col:
			buf.WriteString("| ")
		case c == col && c > prevCol:
			buf.WriteString("\\ ")
		case c == col && c < prevCol:
			buf.WriteString("/ ")
		case colRemaining[c] > 0:
			buf.WriteString("| ")
		default:
			buf.WriteString("  ")
		}
	}
	buf.WriteString("\n")
}

func drawMergeForkLines(buf *bytes.Buffer, node *promptsheoncas.GraphNode, columns map[string]int, maxCol int, colRemaining []int) {
	if len(node.Parents) <= 1 {
		return
	}

	col := columns[node.Hash]
	parentCols := make([]int, 0, len(node.Parents))
	for _, p := range node.Parents {
		if pc, ok := columns[p]; ok && pc != col {
			parentCols = append(parentCols, pc)
		}
	}
	if len(parentCols) == 0 {
		return
	}

	sort.Ints(parentCols)
	minPC, maxPC := parentCols[0], parentCols[len(parentCols)-1]

	anyBelow := false
	for _, r := range colRemaining {
		if r > 0 {
			anyBelow = true
			break
		}
	}

	for c := 0; c <= maxCol; c++ {
		switch {
		case c == col:
			if anyBelow {
				buf.WriteString("| ")
			} else {
				buf.WriteString("  ")
			}
		case c >= minPC && c <= maxPC:
			switch {
			case minPC == maxPC:
				buf.WriteString("\\ ")
			case c == minPC:
				buf.WriteString("| ")
			case c == maxPC:
				buf.WriteString("/ ")
			default:
				buf.WriteString("  ")
			}
		default:
			buf.WriteString("  ")
		}
	}
	buf.WriteString("\n")
}

func cmdStats() error {
	stats, err := promptsheoncas.GetStats()
	if err != nil {
		return err
	}

	fmt.Printf("Repository objects:\n")
	fmt.Printf("  blobs:   %d\n", stats.BlobCount)
	fmt.Printf("  trees:   %d\n", stats.TreeCount)
	fmt.Printf("  commits: %d\n", stats.CommitCount)
	fmt.Printf("  total:   %d\n", stats.TotalObjects)
	fmt.Printf("  storage: %s\n", formatBytes(stats.StorageBytes))
	fmt.Printf("  refs:    %d\n", stats.RefCount)
	return nil
}

func formatBytes(b int64) string {
	switch {
	case b < 1024:
		return fmt.Sprintf("%d B", b)
	case b < 1024*1024:
		return fmt.Sprintf("%.1f KiB", float64(b)/1024)
	default:
		return fmt.Sprintf("%.1f MiB", float64(b)/(1024*1024))
	}
}

func cmdVerify() error {
	result, err := promptsheoncas.Verify()
	if err != nil {
		return err
	}

	fmt.Printf("Integrity check:\n")
	fmt.Printf("  total objects:  %d\n", result.TotalObjects)
	fmt.Printf("  verified:       %d\n", result.VerifiedObjects)
	fmt.Printf("  corrupted:      %d\n", result.CorruptedObjects)
	fmt.Printf("  broken refs:    %d\n", result.BrokenRefs)
	fmt.Printf("  orphaned:       %d\n", result.OrphanedObjects)

	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}
		return fmt.Errorf("integrity check failed with %d issues", len(result.Errors))
	}

	fmt.Println("\nrepository is healthy")
	return nil
}
