package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"promptsheon/internal/llm"
	"promptsheon/internal/promptsheon"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit()
	case "hash-object":
		err = cmdHashObject(args)
	case "write-object":
		err = cmdWriteObject(args)
	case "read-object":
		err = cmdReadObject(args)
	case "commit":
		err = cmdCommit(args)
	case "log":
		err = cmdLog(args)
	case "checkout":
		err = cmdCheckout(args)
	case "branch":
		err = cmdBranch(args)
	case "delete-branch":
		err = cmdDeleteBranch(args)
	case "diff":
		err = cmdDiff(args)
	case "status":
		err = cmdStatus()
	case "show":
		err = cmdShow(args)
	case "ls-tree":
		err = cmdLsTree(args)
	case "cat-file":
		err = cmdCatFile(args)
	case "graph":
		err = cmdGraph()
	case "stats":
		err = cmdStats()
	case "verify":
		err = cmdVerify()
	case "run":
		err = cmdRun(args)
	case "provider":
		err = cmdProvider(args)
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Promptsheon — Version Control for AI Intelligence

Usage:
  init                        Initialize a new repository
  hash-object <data>          Compute SHA-256 hash of content
  write-object <data>         Write a blob object to CAS store
  read-object <hash>          Read and display an object
  commit <tree> [msg]         Create a commit (uses current branch)
  log [n]                     Show commit history
  checkout <ref|hash>         Switch to a branch or commit
  branch [name] [hash]        Create or list branches
  delete-branch <name>        Delete a branch
  diff <hashA> <hashB>        Diff two intelligence states
  status                      Show repository state summary
  show <hash>                 Show object details (type-aware)
  ls-tree <hash>              List tree entries
  cat-file <hash>             Output blob content to stdout
  graph                       Show commit DAG visualization
  stats                       Show repository statistics
  verify                      Check repository integrity
  run --provider <p> --model <m> --prompt <text>   Run a prompt via LLM
  provider list               List registered LLM providers
  provider test <name>        Test an LLM provider`)
}

func cmdInit() error {
	if promptsheon.IsInitialized() {
		return fmt.Errorf("repository already initialized")
	}
	if err := promptsheon.Init(); err != nil {
		return err
	}
	fmt.Println("initialized empty promptsheon repository in .promptsheon/")
	return nil
}

func cmdHashObject(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: promptsheon hash-object <data>")
	}
	data := strings.Join(args, " ")
	obj := promptsheon.NewBlobObject(data)
	hash, err := promptsheon.ObjectHash(obj)
	if err != nil {
		return fmt.Errorf("compute hash: %w", err)
	}
	fmt.Println(hash)
	return nil
}

func cmdWriteObject(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: promptsheon write-object <data>")
	}
	data := strings.Join(args, " ")
	obj := promptsheon.NewBlobObject(data)
	hash, err := promptsheon.WriteObject(obj)
	if err != nil {
		return err
	}
	fmt.Println(hash)
	return nil
}

func cmdReadObject(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: promptsheon read-object <hash>")
	}
	obj, err := promptsheon.ReadObject(args[0])
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
		return fmt.Errorf("usage: promptsheon commit <tree-hash> [message]")
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
	currentHash, err := promptsheon.GetCurrentCommitHash()
	if err == nil && currentHash != "" {
		parentHashes = []string{currentHash}
	}

	telemetry, telErr := parseTelemetry()

	result, err := promptsheon.Commit(treeHash, parentHashes, author, message, telemetry)
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

	entries, err := promptsheon.Log(count)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("no commits")
		return nil
	}

	ref, _ := promptsheon.GetCurrentRef()
	refDisplay := ref
	if refDisplay == "" {
		refDisplay = "(detached HEAD)"
	}

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
		return fmt.Errorf("usage: promptsheon checkout <ref|hash>")
	}
	if err := promptsheon.Checkout(args[0]); err != nil {
		return err
	}

	head, _ := promptsheon.ReadHEAD()
	if promptsheon.IsHEADDetached(head) {
		fmt.Printf("HEAD is now at %s\n", head[:12])
	} else {
		fmt.Printf("switched to branch '%s'\n", promptsheon.HEADRefName(head))
	}
	return nil
}

func cmdBranch(args []string) error {
	if len(args) == 0 {
		refs, err := promptsheon.ListRefs()
		if err != nil {
			return err
		}
		sort.Strings(refs)
		currentRef, _ := promptsheon.GetCurrentRef()
		for _, r := range refs {
			hash, _ := promptsheon.ReadRef(r)
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

	if err := promptsheon.CreateBranch(name, targetHash); err != nil {
		return err
	}
	fmt.Printf("created branch '%s'\n", name)
	return nil
}

func cmdDeleteBranch(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: promptsheon delete-branch <name>")
	}
	if err := promptsheon.DeleteBranch(args[0]); err != nil {
		return err
	}
	fmt.Printf("deleted branch '%s'\n", args[0])
	return nil
}

func cmdDiff(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: promptsheon diff <hashA> <hashB>")
	}
	result, err := promptsheon.DiffIntelligence(args[0], args[1])
	if err != nil {
		return err
	}
	fmt.Print(promptsheon.FormatDiff(result))
	return nil
}

// ---------------------------------------------------------------------------
// Visualization commands
// ---------------------------------------------------------------------------

func cmdStatus() error {
	if !promptsheon.IsInitialized() {
		return fmt.Errorf("not a promptsheon repository")
	}

	ref, err := promptsheon.GetCurrentRef()
	if err != nil {
		return fmt.Errorf("read ref: %w", err)
	}
	headHash, err := promptsheon.GetCurrentCommitHash()
	if err != nil {
		return fmt.Errorf("read HEAD: %w", err)
	}
	refs, err := promptsheon.ListRefs()
	if err != nil {
		return fmt.Errorf("list refs: %w", err)
	}

	headLine := ""
	if headHash == "" {
		headLine = "no commits yet"
	} else if ref == "" {
		headLine = fmt.Sprintf("detached HEAD at %s", headHash[:12])
	} else {
		headLine = fmt.Sprintf("on branch %s", ref)
	}
	fmt.Printf("HEAD: %s\n", headLine)

	if headHash != "" {
		obj, err := promptsheon.ReadObject(headHash)
		if err == nil {
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
			h, _ := promptsheon.ReadRef(r)
			shortHash := ""
			if len(h) >= 12 {
				shortHash = h[:12]
			}
			fmt.Printf("%s %s  %s\n", prefix, r, shortHash)
		}
	}

	stats, err := promptsheon.GetStats()
	if err == nil {
		fmt.Printf("objects: %d (blobs: %d, trees: %d, commits: %d)\n",
			stats.TotalObjects, stats.BlobCount, stats.TreeCount, stats.CommitCount)
	}

	return nil
}

func cmdShow(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: promptsheon show <hash>")
	}
	hash := args[0]

	obj, err := promptsheon.ReadObject(hash)
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
		treeObj, err := promptsheon.ReadObject(obj.TreeHash)
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
		return fmt.Errorf("usage: promptsheon ls-tree <hash>")
	}
	hash := args[0]

	obj, err := promptsheon.ReadObject(hash)
	if err != nil {
		return err
	}
	if len(obj.Entries) == 0 {
		return fmt.Errorf("object %s is not a tree", hash[:12])
	}

	var walkTree func(hash string, prefix string) error
	walkTree = func(hash string, prefix string) error {
		obj, err := promptsheon.ReadObject(hash)
		if err != nil {
			return err
		}
		for _, e := range obj.Entries {
			path := e.Name
			if prefix != "" {
				path = prefix + "/" + e.Name
			}
			fmt.Printf("%-6s %s  %s\n", e.Type, e.Hash[:12], path)
			if e.Type == promptsheon.TypeTree {
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
		return fmt.Errorf("usage: promptsheon cat-file <hash>")
	}
	obj, err := promptsheon.ReadObject(args[0])
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
	nodes, err := promptsheon.BuildGraph()
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		fmt.Println("no commits")
		return nil
	}

	refs, _ := promptsheon.ListRefDetails()
	headRef, _ := promptsheon.GetCurrentRef()

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

func assignGraphColumns(nodes []*promptsheon.GraphNode, refs []promptsheon.RefDetail, headRef string) map[string]int {
	columns := make(map[string]int)
	nodeMap := make(map[string]*promptsheon.GraphNode)
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

func walkChain(hash string, col int, columns map[string]int, nodeMap map[string]*promptsheon.GraphNode) {
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

func renderGraph(nodes []*promptsheon.GraphNode, columns map[string]int, maxCol int) string {
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
			} else {
				labels = append(labels, "HEAD")
			}
			labels = append(labels, node.Branches[1:]...)
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

func drawConnLines(buf *bytes.Buffer, prevCol, col, maxCol int, colRemaining []int, node *promptsheon.GraphNode) {
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

func drawMergeForkLines(buf *bytes.Buffer, node *promptsheon.GraphNode, columns map[string]int, maxCol int, colRemaining []int) {
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
			if c == minPC && c == maxPC {
				buf.WriteString("\\ ")
			} else if c == minPC {
				buf.WriteString("| ")
			} else if c == maxPC {
				buf.WriteString("/ ")
			} else {
				buf.WriteString("  ")
			}
		default:
			buf.WriteString("  ")
		}
	}
	buf.WriteString("\n")
}

func cmdStats() error {
	stats, err := promptsheon.GetStats()
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
	result, err := promptsheon.Verify()
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

func cmdRun(args []string) error {
	provider := "openai"
	model := "gpt-3.5-turbo"
	prompt := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider", "-p":
			if i+1 < len(args) {
				provider = args[i+1]
				i++
			}
		case "--model", "-m":
			if i+1 < len(args) {
				model = args[i+1]
				i++
			}
		case "--prompt":
			if i+1 < len(args) {
				prompt = args[i+1]
				i++
			}
		default:
			if prompt == "" {
				prompt = args[i]
			}
		}
	}

	if prompt == "" {
		return fmt.Errorf("usage: promptsheon run --provider <name> --model <model> --prompt <text>")
	}

	llm.LoadFromEnv()
	p, err := llm.Global.Get(provider)
	if err != nil {
		return fmt.Errorf("provider not available: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Running with %s/%s...\n", provider, model)
	start := time.Now()
	resp, err := p.Complete(context.Background(), &llm.Request{
		Model: model,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 2048,
	})
	if err != nil {
		return err
	}

	fmt.Println(resp.Content)
	fmt.Fprintf(os.Stderr, "\nTokens: %d (prompt: %d, completion: %d) | Latency: %v\n",
		resp.Usage.TotalTokens, resp.Usage.PromptTokens, resp.Usage.CompletionTokens,
		time.Since(start).Round(time.Millisecond))
	return nil
}

func cmdProvider(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: promptsheon provider <list|test>")
	}

	switch args[0] {
	case "list":
		llm.LoadFromEnv()
		providers := llm.Global.Providers()
		fmt.Println("Registered providers:")
		for _, name := range providers {
			fmt.Printf("  - %s\n", name)
		}
	case "test":
		if len(args) < 2 {
			return fmt.Errorf("usage: promptsheon provider test <name>")
		}
		llm.LoadFromEnv()
		name := args[1]
		p, err := llm.Global.Get(name)
		if err != nil {
			return fmt.Errorf("provider not available: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Testing %s...\n", name)
		start := time.Now()
		resp, err := p.Complete(context.Background(), &llm.Request{
			Model: "gpt-3.5-turbo",
			Messages: []llm.Message{
				{Role: "user", Content: "Say hello in one word."},
			},
			MaxTokens: 10,
		})
		latency := time.Since(start)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		fmt.Printf("OK: %q\n", resp.Content)
		fmt.Fprintf(os.Stderr, "Latency: %v | Tokens: %d\n", latency.Round(time.Millisecond), resp.Usage.TotalTokens)
	default:
		return fmt.Errorf("unknown provider subcommand: %s", args[0])
	}
	return nil
}
