package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// cmdSelfEvolve implements the promptsheon CLI's selfevolve
// subcommands. The CLI writes to the daemon's API; the daemon's
// store layer is the source of truth. Three subcommands:
//
//   selfevolve enable <capability>   --dataset <id> [--min-score N]
//                                            [--max-revisions N] [--cooldown-sec N]
//                                            [--target-env dev|staging|prod]
//   selfevolve disable <capability>
//   selfevolve status <capability>
//
// `enable` enables closed-loop self-evolution on a capability by
// PUTting the per-capability config. The daemon's evolver loop
// then watches the capability's active eval runs.
//
// `disable` zeros the enabled flag. The persisted config
// (min_score, max_revisions, etc.) is preserved so a later
// `enable` reuses the same thresholds.
//
// `status` GETs the capability and prints whether self-evolve
// is on, plus the persisted thresholds.
func cmdSelfEvolve(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: promptsheon selfevolve <enable|disable|status> <capability>")
	}
	op := args[0]
	capID := args[1]
	switch op {
	case "enable":
		return selfEvolveEnable(capID, args[2:])
	case "disable":
		return selfEvolveDisable(capID)
	case "status":
		return selfEvolveStatus(capID)
	default:
		return fmt.Errorf("unknown selfevolve subcommand: %s", op)
	}
}

// flagSet is a tiny flag parser that doesn't pull in the
// stdlib flag package (the rest of the CLI uses positional
// args). values is a name → value map. Flags without a
// value use "--name=true" (or any value), and we expose the
// value as the parsed string. This keeps the parser trivial
// and avoids ambiguity around "--flag --other".
func selfEvolveParseFlags(args []string) (flags map[string]bool, values map[string]string, err error) {
	flags = map[string]bool{}
	values = map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			return nil, nil, fmt.Errorf("unexpected positional arg: %s", a)
		}
		name := strings.TrimPrefix(a, "--")
		if eq := strings.Index(name, "="); eq >= 0 {
			values[name[:eq]] = name[eq+1:]
			continue
		}
		// Boolean flags: --name with no following value.
		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			flags[name] = true
			continue
		}
		// Value flag: the next arg is the value.
		values[name] = args[i+1]
		i++
	}
	return flags, values, nil
}

func selfEvolveEnable(capID string, args []string) error {
	_, values, err := selfEvolveParseFlags(args)
	if err != nil {
		return err
	}
	dataset, ok := values["dataset"]
	if !ok || dataset == "" {
		return fmt.Errorf("--dataset is required")
	}
	cfg := map[string]any{
		"dataset_id":    dataset,
		"target_env":    "dev",
		"min_score":     0.9,
		"max_revisions": 10,
		"cooldown_sec":  900,
	}
	if v, ok := values["min-score"]; ok {
		cfg["min_score"] = parseFloat(v)
	}
	if v, ok := values["max-revisions"]; ok {
		cfg["max_revisions"] = parseInt(v)
	}
	if v, ok := values["cooldown-sec"]; ok {
		cfg["cooldown_sec"] = parseInt(v)
	}
	if v, ok := values["target-env"]; ok {
		cfg["target_env"] = v
	}
	// The daemon's API expects enabled=true so the evolver picks
	// it up. The PUT merges with the existing row.
	cfg["enabled"] = true
	return apiUpdateSelfEvolve(capID, cfg)
}

func selfEvolveDisable(capID string) error {
	// Disable by setting enabled=false. Other config is preserved.
	return apiUpdateSelfEvolve(capID, map[string]any{"enabled": false})
}

func selfEvolveStatus(capID string) error {
	body, err := apiGetCapability(capID)
	if err != nil {
		return err
	}
	var cap struct {
		ID        string  `json:"id"`
		SelfEvolve struct {
			Enabled      bool    `json:"enabled"`
			MinScore     float64 `json:"min_score"`
			MaxRevisions int     `json:"max_revisions"`
			CooldownSec  int     `json:"cooldown_sec"`
			TargetEnv    string  `json:"target_env"`
			DatasetID    string  `json:"dataset_id"`
		} `json:"self_evolve"`
	}
	if err := json.Unmarshal(body, &cap); err != nil {
		return fmt.Errorf("decode capability: %w", err)
	}
	fmt.Printf("capability:  %s\n", cap.ID)
	fmt.Printf("enabled:     %v\n", cap.SelfEvolve.Enabled)
	fmt.Printf("target_env:  %s\n", cap.SelfEvolve.TargetEnv)
	fmt.Printf("min_score:   %v\n", cap.SelfEvolve.MinScore)
	fmt.Printf("max_revs:    %d\n", cap.SelfEvolve.MaxRevisions)
	fmt.Printf("cooldown_s:  %d\n", cap.SelfEvolve.CooldownSec)
	fmt.Printf("dataset_id: %s\n", cap.SelfEvolve.DatasetID)
	return nil
}

// apiGetCapability GETs /api/v1/capabilities/{id} and returns
// the raw JSON body. The shared httpGet helper decodes into a
// pointer; we round-trip through json.RawMessage to get the
// bytes back.
func apiGetCapability(id string) ([]byte, error) {
	var raw json.RawMessage
	if err := httpGet(serverURL()+"/api/v1/capabilities/"+id, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// apiUpdateSelfEvolve PUTs the capability's self-evolve
// config. The daemon accepts the partial update; fields not
// in the body are preserved.
func apiUpdateSelfEvolve(capID string, fields map[string]any) error {
	body, _ := json.Marshal(fields)
	req, err := http.NewRequest(http.MethodPut, serverURL()+"/api/v1/capabilities/"+capID+"/self-evolve", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon returned %s: %s", resp.Status, string(b))
	}
	fmt.Printf("self-evolve: capability %s updated\n", capID)
	return nil
}

func parseFloat(s string) float64 {
	var f float64
	_, _ = fmt.Sscan(s, &f)
	return f
}

func parseInt(s string) int {
	var i int
	_, _ = fmt.Sscan(s, &i)
	return i
}
