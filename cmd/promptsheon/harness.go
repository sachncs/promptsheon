package main

// harness.go: the promptsheon CLI's harness-engineering commands
// (datasets, preconditions, evals). Mirrors the workspace / project /
// capability / release subcommands and lives in package main so it
// shares the httpGet / httpPost helpers in http.go.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const opRun = "run"

func cmdDataset(args []string) error {
	if len(args) == 0 {
		return usageErrorf("promptsheon dataset <list|create|get|put-cases|delete>")
	}
	server := serverURL()
	switch args[0] {
	case opList:
		if len(args) < 2 {
			return usageErrorf("promptsheon dataset list <capability_id>")
		}
		var ds []any
		if err := httpGet(server+"/api/v1/capabilities/"+args[1]+"/datasets", &ds); err != nil {
			return err
		}
		for _, d := range ds {
			fmt.Printf("%+v\n", d)
		}
	case opCreate:
		if len(args) < 2 {
			return usageErrorf("promptsheon dataset create <capability_id> --name <n> [--file cases.json]")
		}
		capabilityID := args[1]
		name, casesPath, err := readFlag(args[2:], "--name")
		if err != nil {
			return err
		}
		if name == "" {
			return usageErrorf("--name is required")
		}
		body := map[string]any{"name": name}
		if casesPath != "" {
			cases, err := loadCasesFile(casesPath)
			if err != nil {
				return err
			}
			body["cases"] = cases
		}
		var result any
		if err := httpPost(server+"/api/v1/capabilities/"+capabilityID+"/datasets", body, &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case opGet:
		if len(args) < 2 {
			return usageErrorf("promptsheon dataset get <id>")
		}
		var result any
		if err := httpGet(server+"/api/v1/datasets/"+args[1], &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case "put-cases":
		if len(args) < 3 {
			return usageErrorf("promptsheon dataset put-cases <id> <file>")
		}
		cases, err := loadCasesFile(args[2])
		if err != nil {
			return err
		}
		body := map[string]any{"cases": cases}
		var result any
		if err := httpPost(server+"/api/v1/datasets/"+args[1]+"/cases", body, &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case opDelete:
		if len(args) < 2 {
			return usageErrorf("promptsheon dataset delete <id>")
		}
		if err := httpDelete(server + "/api/v1/datasets/" + args[1]); err != nil {
			return err
		}
		fmt.Println("deleted")
	default:
		return fmt.Errorf("unknown dataset subcommand: %s", args[0])
	}
	return nil
}

func cmdPrecondition(args []string) error {
	if len(args) == 0 {
		return usageErrorf("promptsheon precondition <list|add|delete>")
	}
	server := serverURL()
	switch args[0] {
	case opList:
		if len(args) < 2 {
			return usageErrorf("promptsheon precondition list <capability_id>")
		}
		var ps []any
		if err := httpGet(server+"/api/v1/capabilities/"+args[1]+"/preconditions", &ps); err != nil {
			return err
		}
		for _, p := range ps {
			fmt.Printf("%+v\n", p)
		}
	case "add":
		if len(args) < 2 {
			return usageErrorf("promptsheon precondition add <capability_id> --name <n> --cmd <c> [--timeout 60]")
		}
		capabilityID := args[1]
		name, cmd, err := readTwoFlags(args[2:], "--name", "--cmd")
		if err != nil {
			return err
		}
		if name == "" || cmd == "" {
			return usageErrorf("--name and --cmd are required")
		}
		timeout := readStrFlag(args[2:], "--timeout")
		body := map[string]any{"name": name, "command": cmd, "timeout_sec": 60}
		if timeout != "" {
			var t int
			fmt.Sscanf(timeout, "%d", &t)
			if t > 0 {
				body["timeout_sec"] = t
			}
		}
		var result any
		if err := httpPost(server+"/api/v1/capabilities/"+capabilityID+"/preconditions", body, &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case opDelete:
		if len(args) < 2 {
			return usageErrorf("promptsheon precondition delete <id>")
		}
		if err := httpDelete(server + "/api/v1/preconditions/" + args[1]); err != nil {
			return err
		}
		fmt.Println("deleted")
	default:
		return fmt.Errorf("unknown precondition subcommand: %s", args[0])
	}
	return nil
}

func cmdEval(args []string) error {
	if len(args) == 0 {
		return usageErrorf("promptsheon eval <list|run|get>")
	}
	server := serverURL()
	switch args[0] {
	case opRun:
		if len(args) < 2 {
			return usageErrorf("promptsheon eval run <release_id> --dataset <id> [--scorer exact_match]")
		}
		releaseID := args[1]
		dataset, scorer, err := readTwoFlags(args[2:], "--dataset", "--scorer")
		if err != nil {
			return err
		}
		if dataset == "" {
			return usageErrorf("--dataset is required")
		}
		body := map[string]any{"dataset_id": dataset}
		if scorer != "" {
			body["scorer"] = scorer
		}
		var result any
		if err := httpPost(server+"/api/v1/releases/"+releaseID+"/evals", body, &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	case opList:
		if len(args) < 2 {
			return usageErrorf("promptsheon eval list <release_id>")
		}
		var rs []any
		if err := httpGet(server+"/api/v1/releases/"+args[1]+"/evals", &rs); err != nil {
			return err
		}
		for _, r := range rs {
			fmt.Printf("%+v\n", r)
		}
	case opGet:
		if len(args) < 2 {
			return usageErrorf("promptsheon eval get <id>")
		}
		var result any
		if err := httpGet(server+"/api/v1/evals/"+args[1], &result); err != nil {
			return err
		}
		fmt.Printf("%+v\n", result)
	default:
		return fmt.Errorf("unknown eval subcommand: %s", args[0])
	}
	return nil
}

// ----- helpers -----

// readFlag returns the value of a single --flag from args.
func readFlag(args []string, flag string) (string, string, error) {
	value := ""
	path := ""
	for i := 0; i < len(args); i++ {
		if args[i] == flag && i+1 < len(args) {
			value = args[i+1]
			i++
		}
		if args[i] == "--file" && i+1 < len(args) {
			path = args[i+1]
			i++
		}
	}
	return value, path, nil
}

// readStrFlag returns the value of a single --flag from args.
func readStrFlag(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

// readTwoFlags returns the values of two distinct --flag from args.
func readTwoFlags(args []string, a, b string) (string, string, error) {
	av, bv := "", ""
	for i := 0; i < len(args); i++ {
		if args[i] == a && i+1 < len(args) {
			av = args[i+1]
			i++
		}
		if args[i] == b && i+1 < len(args) {
			bv = args[i+1]
			i++
		}
	}
	return av, bv, nil
}

// loadCasesFile reads a JSON file with shape
// [{"inputs": ..., "expected": ...}, ...] (or just the array) and
// returns it as []harness.DatasetCase-shaped JSON. We accept both
// shapes for ergonomic CLI use.
func loadCasesFile(path string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") {
		var wrapped struct {
			Cases []map[string]any `json:"cases"`
		}
		if err := json.Unmarshal(data, &wrapped); err != nil {
			return nil, err
		}
		return wrapped.Cases, nil
	}
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil, err
	}
	return arr, nil
}

// keep fmt referenced even when timeout parsing is empty.
var _ = fmt.Sprintf
