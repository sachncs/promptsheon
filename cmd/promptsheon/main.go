// Package main is the promptsheon CLI binary.
//
// The CLI is structured as follows:
//
//   main.go      - entry point, dispatcher, common flags, LLM commands,
//                 and the workspace/project/capability/release CRUD commands.
//   cas.go       - the git-style content-addressable storage commands
//                 (init, hash-object, commit, log, branch, graph, ...).
//   http.go      - tiny localhost-only HTTP helpers used by the CRUD
//                 subcommands to talk to a running promptsheond.
//
// Splitting per-command keeps individual files small and reviewable.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sachncs/promptsheon/internal/buildinfo"
	"github.com/sachncs/promptsheon/internal/llm"
)

// Shared subcommand names. The CAS file and the API CRUD files each
// declare their own; these are used across files.
const (
	opList    = "list"
	opCreate  = "create"
	opGet     = "get"
	opDelete  = "delete"
	flagProv  = "--provider"
	cmdTest   = "test"
)

func main() {
	if handleEarlyExit() {
		return
	}
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	if err := dispatchCommand(cmd, args); err != nil {
		handleCmdError(err)
	}
}

func handleEarlyExit() bool {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "--version", "-version", "-V":
			info := buildinfo.Get()
			fmt.Printf("promptsheon %s (commit %s, built %s, %s/%s)\n",
				info.Version, info.Commit, info.BuildTime, info.OS, info.Arch)
			return true
		case "--help", "-help", "-h":
			printUsage()
			return true
		}
	}
	return false
}

func dispatchCommand(cmd string, args []string) error {
	h, ok := commandHandlers[cmd]
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
	return h(args)
}

var commandHandlers = map[string]func([]string) error{
	"init":          func(_ []string) error { return cmdInit() },
	"hash-object":   cmdHashObject,
	"write-object":  cmdWriteObject,
	"read-object":   cmdReadObject,
	"commit":        cmdCommit,
	"log":           cmdLog,
	"checkout":      cmdCheckout,
	"branch":        cmdBranch,
	"delete-branch": cmdDeleteBranch,
	"diff":          cmdDiff,
	"status":        func(_ []string) error { return cmdStatus() },
	"show":          cmdShow,
	"ls-tree":       cmdLsTree,
	"cat-file":      cmdCatFile,
	"graph":         func(_ []string) error { return cmdGraph() },
	"stats":         func(_ []string) error { return cmdStats() },
	"verify":        func(_ []string) error { return cmdVerify() },
	"run":           cmdRun,
	"provider":      cmdProvider,
	"workspace":     cmdWorkspace,
	"project":       cmdProject,
	"capability":    cmdCapability,
	"release":       cmdRelease,
	"help":          func(_ []string) error { printUsage(); return nil },
}

func handleCmdError(err error) {
	if errors.Is(err, errUsage) {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Run 'promptsheon help' for usage.")
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

// errUsage is the sentinel error returned by command functions
// when the arguments don't match the expected shape. main()
// detects it and exits with status 2 (the conventional EX_USAGE
// from sysexits.h) plus a hint to run 'promptsheon help'.
var errUsage = errors.New("invalid arguments")

// usageErrorf returns an errUsage that formats its message as a
// per-command usage hint, e.g. "usage: promptsheon commit <tree>
// [message]". The wrapping keeps the message visible to the user
// while the sentinel lets main() pick the right exit code.
func usageErrorf(format string) error {
	return fmt.Errorf("%w: %s", errUsage, format)
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
  delete-branch <name>        Delete branch
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
  provider test <name>        Test an LLM provider
  workspace <list|create|get|delete>  Manage workspaces via API
  project <list|create|get|delete>    Manage projects via API
  capability <list|create|get|delete> Manage capabilities via API
  release <list|get|vote|activate|rollback|invoke|approval>  Manage releases via API`)
}

// ---------------------------------------------------------------------------
// LLM subcommands
// ---------------------------------------------------------------------------

func cmdRun(args []string) error {
	provider := "openai"
	model := "gpt-3.5-turbo"
	prompt := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case flagProv, "-p":
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
		return usageErrorf("promptsheon run --provider <name> --model <model> --prompt <text>")
	}

	providers := llm.NewRegistry()
	providers.LoadFromEnv()
	p, err := providers.Get(provider)
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
		return usageErrorf("promptsheon provider <list|test>")
	}

	switch args[0] {
	case opList:
		providers := llm.NewRegistry()
		providers.LoadFromEnv()
		names := providers.Providers()
		fmt.Println("Registered providers:")
		for _, name := range names {
			fmt.Printf("  - %s\n", name)
		}
	case cmdTest:
		if len(args) < 2 {
			return usageErrorf("promptsheon provider test <name>")
		}
		providers := llm.NewRegistry()
		providers.LoadFromEnv()
		name := args[1]
		p, err := providers.Get(name)
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

// ---------------------------------------------------------------------------
// API CRUD subcommands (workspace / project / capability / release)
// ---------------------------------------------------------------------------

func cmdWorkspace(args []string) error {
	if len(args) == 0 {
		return usageErrorf("promptsheon workspace <list|create|get|delete>")
	}
	server := serverURL()
	switch args[0] {
	case opList:
		var workspaces []any
		if err := httpGet(server+"/api/v1/workspaces", &workspaces); err != nil {
			return err
		}
		for _, w := range workspaces {
			fmt.Printf("%+v\n", w)
		}
	case opCreate:
		if len(args) < 2 {
			return usageErrorf("promptsheon workspace create <name>")
		}
		var result any
		if err := httpPost(server+"/api/v1/workspaces", map[string]string{cliKeyName: args[1]}, &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case opGet:
		if len(args) < 2 {
			return usageErrorf("promptsheon workspace get <id>")
		}
		var result any
		if err := httpGet(server+"/api/v1/workspaces/"+args[1], &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case opDelete:
		if len(args) < 2 {
			return usageErrorf("promptsheon workspace delete <id>")
		}
		if err := httpDelete(server + "/api/v1/workspaces/" + args[1]); err != nil {
			return err
		}
		fmt.Println("deleted")
	default:
		return fmt.Errorf("unknown workspace subcommand: %s", args[0])
	}
	return nil
}

func cmdProject(args []string) error {
	if len(args) == 0 {
		return usageErrorf("promptsheon project <list|create|get|delete>")
	}
	server := serverURL()
	switch args[0] {
	case opList:
		if len(args) < 2 {
			return usageErrorf("promptsheon project list <workspace_id>")
		}
		var projects []any
		if err := httpGet(server+"/api/v1/workspaces/"+args[1]+"/projects", &projects); err != nil {
			return err
		}
		for _, p := range projects {
			fmt.Printf("%+v\n", p)
		}
	case opCreate:
		if len(args) < 3 {
			return usageErrorf("promptsheon project create <workspace_id> <name>")
		}
		var result any
		if err := httpPost(server+"/api/v1/workspaces/"+args[1]+"/projects", map[string]string{cliKeyName: args[2]}, &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case opGet:
		if len(args) < 2 {
			return usageErrorf("promptsheon project get <id>")
		}
		var result any
		if err := httpGet(server+"/api/v1/projects/"+args[1], &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case opDelete:
		if len(args) < 2 {
			return usageErrorf("promptsheon project delete <id>")
		}
		if err := httpDelete(server + "/api/v1/projects/" + args[1]); err != nil {
			return err
		}
		fmt.Println("deleted")
	default:
		return fmt.Errorf("unknown project subcommand: %s", args[0])
	}
	return nil
}

func cmdCapability(args []string) error {
	if len(args) == 0 {
		return usageErrorf("promptsheon capability <list|create|get|delete>")
	}
	server := serverURL()
	switch args[0] {
	case opList:
		if len(args) < 2 {
			return usageErrorf("promptsheon capability list <project_id>")
		}
		var caps []any
		if err := httpGet(server+"/api/v1/projects/"+args[1]+"/capabilities", &caps); err != nil {
			return err
		}
		for _, c := range caps {
			fmt.Printf("%+v\n", c)
		}
	case opCreate:
		if len(args) < 3 {
			return usageErrorf("promptsheon capability create <project_id> <name>")
		}
		var result any
		if err := httpPost(server+"/api/v1/projects/"+args[1]+"/capabilities", map[string]string{cliKeyName: args[2]}, &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case opGet:
		if len(args) < 2 {
			return usageErrorf("promptsheon capability get <id>")
		}
		var result any
		if err := httpGet(server+"/api/v1/capabilities/"+args[1], &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case opDelete:
		if len(args) < 2 {
			return usageErrorf("promptsheon capability delete <id>")
		}
		if err := httpDelete(server + "/api/v1/capabilities/" + args[1]); err != nil {
			return err
		}
		fmt.Println("deleted")
	default:
		return fmt.Errorf("unknown capability subcommand: %s", args[0])
	}
	return nil
}

func cmdRelease(args []string) error {
	if len(args) == 0 {
		return usageErrorf("promptsheon release <list|create|get|vote|activate|rollback|invoke|approval>")
	}
	server := serverURL()
	switch args[0] {
	case opList:
		if len(args) < 2 {
			return usageErrorf("promptsheon release list <capability_id>")
		}
		var rs []any
		if err := httpGet(server+"/api/v1/capabilities/"+args[1]+"/releases", &rs); err != nil {
			return err
		}
		for _, r := range rs {
			fmt.Printf("%+v\n", r)
		}
	case opCreate:
		if len(args) < 3 {
			return usageErrorf("promptsheon release create <version_id> <environment>")
		}
		var result any
		body := map[string]string{"environment": args[2]}
		if err := httpPost(server+"/api/v1/versions/"+args[1]+"/releases", body, &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case opGet:
		if len(args) < 2 {
			return usageErrorf("promptsheon release get <id>")
		}
		var result any
		if err := httpGet(server+"/api/v1/releases/"+args[1], &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case "vote":
		if len(args) < 4 {
			return usageErrorf("promptsheon release vote <id> <identity> <approve|reject|abstain>")
		}
		var result any
		body := map[string]string{"identity": args[2], "decision": args[3]}
		if err := httpPost(server+"/api/v1/releases/"+args[1]+"/votes", body, &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case "activate":
		if len(args) < 2 {
			return usageErrorf("promptsheon release activate <id>")
		}
		var result any
		if err := httpPost(server+"/api/v1/releases/"+args[1]+"/activate", map[string]string{}, &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case "rollback":
		if len(args) < 2 {
			return usageErrorf("promptsheon release rollback <id>")
		}
		var result any
		if err := httpPost(server+"/api/v1/releases/"+args[1]+"/rollback", map[string]string{}, &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case "invoke":
		if len(args) < 2 {
			return usageErrorf("promptsheon release invoke <id> --model <m>")
		}
		model := ""
		for i := 2; i < len(args); i++ {
			if args[i] == "--model" && i+1 < len(args) {
				model = args[i+1]
				break
			}
		}
		body := map[string]string{"model": model}
		var result any
		if err := httpPost(server+"/api/v1/releases/"+args[1]+"/invoke", body, &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case "approval":
		if len(args) < 2 {
			return usageErrorf("promptsheon release approval <id>")
		}
		var result any
		if err := httpGet(server+"/api/v1/releases/"+args[1]+"/approval", &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	default:
		return fmt.Errorf("unknown release subcommand: %s", args[0])
	}
	return nil
}
