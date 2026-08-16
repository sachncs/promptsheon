package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sachncs/promptsheon/errf"
	"github.com/sachncs/promptsheon/promptsheon/capability"
	"github.com/sachncs/promptsheon/promptsheon/harness"
	"github.com/sachncs/promptsheon/promptsheon/llm"
	"github.com/sachncs/promptsheon/promptsheon/metrics"
	"github.com/sachncs/promptsheon/promptsheon/models"
	"github.com/sachncs/promptsheon/promptsheon/release"
	"github.com/sachncs/promptsheon/promptsheon/store"

	"github.com/sachncs/promptsheon/promptsheon/evolve"
)

// selfEvolveConfig is one parsed entry from the
// PROMPTSHEON_SELF_EVOLVE env var. The format is
// "cap_id:dataset_id:threshold:target_env:max_revisions:cooldown_sec"
// and multiple entries are separated by ";".
type selfEvolveConfig struct {
	capID        string
	datasetID    string
	threshold    float64
	targetEnv    string
	maxRevisions int
	cooldownSec  int
}

// wireSelfEvolve parses PROMPTSHEON_SELF_EVOLVE and starts
// one background evolver loop per entry. The env var
// wins on boot; the persisted capability config is the
// runtime source of truth. Each loop is bound to
// rootCtx so the daemon's shutdown signal stops them.
func wireSelfEvolve(
	rootCtx context.Context,
	db *store.SQLite,
	releaseSvc *release.Service,
	evalRunner *harness.EvalRunner,
	repos *store.DB,
	logger *slog.Logger,
	providers *llm.Registry,
	metrics *metrics.Collector,
	cfg string,
) {
	for _, raw := range splitEntries(cfg, ';') {
		entry, ok := parseSelfEvolveEntry(raw)
		if !ok {
			logger.Warn("self_evolve: skipping malformed entry", "entry", raw)
			continue
		}
		if err := db.UpdateSelfEvolveConfig(rootCtx, entry.capID, capability.SelfEvolveConfig{
			Enabled:      true,
			MinScore:     entry.threshold,
			MaxRevisions: entry.maxRevisions,
			CooldownSec:  entry.cooldownSec,
			TargetEnv:    entry.targetEnv,
			DatasetID:    entry.datasetID,
		}); err != nil {
			logger.Warn("self_evolve: UpdateSelfEvolveConfig failed", "capability_id", entry.capID, "err", err)
			continue
		}
		loop, err := buildEvolver(db, releaseSvc, repos, providers, logger, metrics, entry.capID)
		if err != nil {
			logger.Error("self_evolve: build evolver failed",
				"capability_id", entry.capID, "err", err)
			continue
		}
		go loop.run(rootCtx)
		logger.Info("self_evolve: started",
			"capability_id", entry.capID, "dataset_id", entry.datasetID,
			"threshold", entry.threshold, "target_env", entry.targetEnv,
			"max_revisions", entry.maxRevisions, "cooldown_sec", entry.cooldownSec)
	}
}

// buildEvolver wires the evolver's collaborators. The
// auditor records every state change to the daemon's
// audit chain; the CAS loader reads/writes the local
// .promptsheon directory.
func buildEvolver(
	db *store.SQLite,
	releaseSvc *release.Service,
	repos *store.DB,
	providers *llm.Registry,
	logger *slog.Logger,
	metrics *metrics.Collector,
	capabilityID string,
) (*selfEvolveLoop, error) {
	repo := newEvolverRepoAdapter(db)
	invoke := makeEvolverLLMInvoke(providers, logger)
	loader := evolve.NewCasPromptLoader()
	activator := &evolverActivatorAdapter{
		svc:          releaseSvc,
		selfApprover: releaseSvc.SelfApprove,
		repo:         repos,
	}
	auditor := &evolverAuditorAdapter{auditor: newEvolverAuditor(repos, logger)}
	revision := evolve.NewLLMRevisionStrategy(invoke)
	validator := evolve.NewHarnessValidator(repo, invoke)
	promoter, perr := evolve.NewPromoter(repo, loader, activator, auditor)
	if perr != nil {
		return nil, errf.Errorf("build promoter: %w", perr)
	}
	ev := evolve.NewEvolver(repo, loader, revision, validator, promoter, auditor, logger)
	return &selfEvolveLoop{
		ev:       ev,
		capID:    capabilityID,
		logger:   logger,
		metrics:  metrics,
		interval: 60 * time.Second,
	}, nil
}

// makeEvolverLLMInvoke returns the LLMInvokeFn the
// revision + validator strategies call. The provider is
// the first registered one (typically "openai" or
// "anthropic" — set by the daemon's env). The model is
// overridable via PROMPTSHEON_SELF_EVOLVE_MODEL.
func makeEvolverLLMInvoke(providers *llm.Registry, logger *slog.Logger) evolve.LLMInvokeFn {
	return func(ctx context.Context, req evolve.LLMInvokeRequest) (string, error) {
		names := providers.Providers()
		if len(names) == 0 {
			return "", errf.Errorf("self_evolve: no LLM providers registered")
		}
		p, err := providers.Get(names[0])
		if err != nil {
			return "", errf.Errorf("self_evolve: get provider: %w", err)
		}
		modelName := os.Getenv("PROMPTSHEON_SELF_EVOLVE_MODEL")
		if modelName == "" {
			modelName = "MiniMax-M2.7"
		}
		resp, err := p.Complete(ctx, &llm.Request{
			Messages: []llm.Message{
				{Role: "system", Content: req.System},
				{Role: "user", Content: req.User},
			},
			Model:     modelName,
			MaxTokens: 4096,
		})
		if err != nil {
			logger.Debug("self_evolve: LLM call failed", "err", err)
			return "", err
		}
		return resp.Content, nil
	}
}

// selfEvolveLoop is the per-capability background loop.
// It ticks on a fixed cadence and calls Evolver.RunOnce.
// The loop exits when the context is cancelled.
type selfEvolveLoop struct {
	ev       *evolve.Evolver
	capID    string
	logger   *slog.Logger
	metrics  *metrics.Collector
	interval time.Duration
}

func (l *selfEvolveLoop) run(ctx context.Context) {
	t := time.NewTicker(l.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			res, err := l.ev.RunOnce(ctx, l.capID)
			if err != nil {
				l.logger.Warn("self_evolve: RunOnce error", "capability_id", l.capID, "err", err)
				continue
			}
			if l.metrics != nil {
				l.metrics.SelfEvolveRunsTotal.Inc()
				l.metrics.SelfEvolveRevisionsTotal.Add(float64(res.Revisions))
				if res.Promoted {
					l.metrics.SelfEvolvePromotedTotal.Inc()
				}
			}
			l.logger.Info("self_evolve: RunOnce result",
				"capability_id", l.capID,
				"promoted", res.Promoted, "skipped", res.Skipped,
				"revisions", res.Revisions, "score", res.Score,
				"duration_ms", res.DurationMS, "reject_reason", res.RejectReason)
		}
	}
}

// splitEntries splits s on sep, ignoring empty pieces
// (so trailing separators don't produce empty entries).
// Used for both PROMPTSHEON_CONTINUOUS_EVAL and
// PROMPTSHEON_SELF_EVOLVE.
func splitEntries(s string, sep rune) []string {
	var out []string
	cur := strings.Builder{}
	for _, r := range s {
		if r == sep {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// parseSelfEvolveEntry parses one entry of the
// "cap:ds:threshold:env:max_revisions:cooldown_sec"
// form. Returns ok=false on any malformed input.
func parseSelfEvolveEntry(s string) (selfEvolveConfig, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return selfEvolveConfig{}, false
	}
	capID := strings.TrimSpace(parts[0])
	dsID := strings.TrimSpace(parts[1])
	threshold, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return selfEvolveConfig{}, false
	}
	targetEnv := strings.TrimSpace(parts[3])
	if targetEnv == "" {
		return selfEvolveConfig{}, false
	}
	maxRevisions, err := strconv.Atoi(parts[4])
	if err != nil || maxRevisions <= 0 {
		return selfEvolveConfig{}, false
	}
	cooldownSec, err := strconv.Atoi(parts[5])
	if err != nil || cooldownSec < 0 {
		return selfEvolveConfig{}, false
	}
	if capID == "" || dsID == "" {
		return selfEvolveConfig{}, false
	}
	return selfEvolveConfig{
		capID:        capID,
		datasetID:    dsID,
		threshold:    threshold,
		targetEnv:    targetEnv,
		maxRevisions: maxRevisions,
		cooldownSec:  cooldownSec,
	}, true
}

// daemonEvolverAuditor writes self_evolve.* audit rows
// to the daemon's tamper-evident audit chain. The
// user_id is the self_evolve admin (created on first
// use); this is stable across restarts.
type daemonEvolverAuditor struct {
	repos  *store.DB
	logger *slog.Logger
}

func newEvolverAuditor(repos *store.DB, logger *slog.Logger) *daemonEvolverAuditor {
	return &daemonEvolverAuditor{repos: repos, logger: logger}
}

func (a *daemonEvolverAuditor) Audit(ctx context.Context, action, target string, detail map[string]any) {
	userID := a.userID(ctx)
	entry := &models.AuditEntry{
		ID:        randomID("audit"),
		UserID:    userID,
		Action:    action,
		Resource:  target,
		Details:   detail,
		Timestamp: time.Now().UTC(),
	}
	if err := a.repos.AppendAudit(ctx, entry); err != nil {
		a.logger.Warn("self_evolve: audit append failed", "action", action, "user_id", userID, "err", err)
	}
}

func (a *daemonEvolverAuditor) userID(ctx context.Context) string {
	const email = "self_evolve@local"
	if u, err := a.repos.GetUserByEmail(ctx, email); err == nil && u != nil {
		return u.ID
	}
	if err := a.repos.CreateUser(ctx, &models.User{
		ID: randomID("user"), Email: email, Name: "self_evolve", Role: "admin",
	}); err != nil {
		a.logger.Warn("self_evolve: create user failed; using sentinel", "err", err)
		return "self_evolve"
	}
	if u, err := a.repos.GetUserByEmail(ctx, email); err == nil && u != nil {
		return u.ID
	}
	return "self_evolve"
}

// randomID returns a 16-char hex string with the given
// prefix. Used for self_evolve audit entry ids. The audit
// chain does not require globally-unique ids (each row's
// hash covers its own content).
func randomID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
