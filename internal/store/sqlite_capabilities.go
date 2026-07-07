package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

// ensure SQLite implements CapabilityRepository.
var _ CapabilityRepository = (*SQLite)(nil)

// workspaceCapabilityRepo is a helper that returns the SQLite as CapabilityRepository.
// This avoids the need to change the SQLite type itself.
func (s *SQLite) workspaceCapabilityRepo() CapabilityRepository {
	return s
}

// ---------------------------------------------------------------------------
// Workspaces
// ---------------------------------------------------------------------------

func (s *SQLite) CreateWorkspace(ctx context.Context, w *capability.Workspace) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workspaces (id, name, organization, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		w.ID, w.Name, w.Organization, w.CreatedAt, w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert workspace: %w", err)
	}
	return nil
}

func (s *SQLite) GetWorkspace(ctx context.Context, id string) (*capability.Workspace, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, organization, created_at, updated_at FROM workspaces WHERE id = ?`, id,
	)
	return scanWorkspace(row)
}

func (s *SQLite) ListWorkspaces(ctx context.Context) ([]*capability.Workspace, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, organization, created_at, updated_at FROM workspaces ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer rows.Close()

	var result []*capability.Workspace
	for rows.Next() {
		w, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}

func (s *SQLite) UpdateWorkspace(ctx context.Context, w *capability.Workspace) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE workspaces SET name = ?, organization = ?, updated_at = ? WHERE id = ?`,
		w.Name, w.Organization, w.UpdatedAt, w.ID,
	)
	if err != nil {
		return fmt.Errorf("update workspace: %w", err)
	}
	return nil
}

func (s *SQLite) DeleteWorkspace(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	return nil
}

func scanWorkspace(scanner interface {
	Scan(dest ...any) error
}) (*capability.Workspace, error) {
	var w capability.Workspace
	err := scanner.Scan(&w.ID, &w.Name, &w.Organization, &w.CreatedAt, &w.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan workspace: %w", err)
	}
	return &w, nil
}

// ---------------------------------------------------------------------------
// Projects
// ---------------------------------------------------------------------------

func (s *SQLite) CreateProject(ctx context.Context, p *capability.Project) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.WorkspaceID, p.Name, p.Description, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

func (s *SQLite) GetProject(ctx context.Context, id string) (*capability.Project, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, workspace_id, name, description, created_at, updated_at FROM projects WHERE id = ?`, id,
	)
	return scanProject(row)
}

func (s *SQLite) ListProjects(ctx context.Context, workspaceID string) ([]*capability.Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workspace_id, name, description, created_at, updated_at FROM projects WHERE workspace_id = ? ORDER BY name`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var result []*capability.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s *SQLite) UpdateProject(ctx context.Context, p *capability.Project) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE projects SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.Description, p.UpdatedAt, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

func (s *SQLite) DeleteProject(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

func scanProject(scanner interface {
	Scan(dest ...any) error
}) (*capability.Project, error) {
	var p capability.Project
	err := scanner.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}
	return &p, nil
}

// ---------------------------------------------------------------------------
// Capabilities
// ---------------------------------------------------------------------------

func (s *SQLite) CreateCapability(ctx context.Context, c *capability.Capability) error {
	tags, err := marshalOrErr(c.Tags)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO capabilities (id, project_id, name, description, owner, tags, state, current_version_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.ProjectID, c.Name, c.Description, c.Owner, string(tags),
		string(c.State), c.CurrentVersionID, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert capability: %w", err)
	}
	return nil
}

func (s *SQLite) GetCapability(ctx context.Context, id string) (*capability.Capability, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, name, description, owner, tags, state, current_version_id, created_at, updated_at
		 FROM capabilities WHERE id = ?`, id,
	)
	return scanCapability(row)
}

func (s *SQLite) ListCapabilities(ctx context.Context, projectID string) ([]*capability.Capability, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, name, description, owner, tags, state, current_version_id, created_at, updated_at
		 FROM capabilities WHERE project_id = ? ORDER BY name`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list capabilities: %w", err)
	}
	defer rows.Close()

	var result []*capability.Capability
	for rows.Next() {
		c, err := scanCapability(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (s *SQLite) UpdateCapability(ctx context.Context, c *capability.Capability) error {
	tags, err := marshalOrErr(c.Tags)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE capabilities SET name = ?, description = ?, owner = ?, tags = ?, state = ?, current_version_id = ?, updated_at = ?
		 WHERE id = ?`,
		c.Name, c.Description, c.Owner, string(tags), string(c.State), c.CurrentVersionID, c.UpdatedAt, c.ID,
	)
	if err != nil {
		return fmt.Errorf("update capability: %w", err)
	}
	return nil
}

func (s *SQLite) DeleteCapability(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM capabilities WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete capability: %w", err)
	}
	return nil
}

func scanCapability(scanner interface {
	Scan(dest ...any) error
}) (*capability.Capability, error) {
	var c capability.Capability
	var tagsStr string
	err := scanner.Scan(&c.ID, &c.ProjectID, &c.Name, &c.Description, &c.Owner, &tagsStr, &c.State, &c.CurrentVersionID, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan capability: %w", err)
	}
	mustUnmarshal([]byte(tagsStr), &c.Tags)
	return &c, nil
}

// ---------------------------------------------------------------------------
// Capability Versions
// ---------------------------------------------------------------------------

// marshalCapabilityVersionJSONFields marshals the JSON fields of a version.
func marshalCapabilityVersionJSONFields(v *capability.CapabilityVersion) (prompt, modelPolicy, contextContract, knowledge, memory, guardrails, tools, mcp, runtimePolicy, evalSuite string, err error) {
	var b []byte
	b, err = marshalOrErr(v.Prompt)
	if err != nil {
		return
	}
	prompt = string(b)

	b, err = marshalOrErr(v.ModelPolicy)
	if err != nil {
		return
	}
	modelPolicy = string(b)

	b, err = marshalOrErr(v.ContextContract)
	if err != nil {
		return
	}
	contextContract = string(b)

	b, err = marshalOrErr(v.Knowledge)
	if err != nil {
		return
	}
	knowledge = string(b)

	b, err = marshalOrErr(v.Memory)
	if err != nil {
		return
	}
	memory = string(b)

	b, err = marshalOrErr(v.Guardrails)
	if err != nil {
		return
	}
	guardrails = string(b)

	b, err = marshalOrErr(v.Tools)
	if err != nil {
		return
	}
	tools = string(b)

	b, err = marshalOrErr(v.MCPServers)
	if err != nil {
		return
	}
	mcp = string(b)

	b, err = marshalOrErr(v.RuntimePolicy)
	if err != nil {
		return
	}
	runtimePolicy = string(b)

	b, err = marshalOrErr(v.EvaluationSuite)
	if err != nil {
		return
	}
	evalSuite = string(b)

	return
}

func (s *SQLite) CreateVersion(ctx context.Context, v *capability.CapabilityVersion) error {
	promptJSON, modelPolicyJSON, contextContractJSON, knowledgeJSON, memoryJSON,
		guardrailsJSON, toolsJSON, mcpJSON, runtimePolicyJSON, evalSuiteJSON, err :=
		marshalCapabilityVersionJSONFields(v)
	if err != nil {
		return fmt.Errorf("marshal version JSON fields: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO capability_versions
		 (id, capability_id, version, prompt, model_policy, context_contract, knowledge,
		  memory, guardrails, tools, mcp_servers, runtime_policy, evaluation_suite, created_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.CapabilityID, v.Version, promptJSON, modelPolicyJSON, contextContractJSON,
		knowledgeJSON, memoryJSON, guardrailsJSON, toolsJSON, mcpJSON, runtimePolicyJSON,
		evalSuiteJSON, v.CreatedAt, v.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("insert version: %w", err)
	}
	return nil
}

func (s *SQLite) GetVersion(ctx context.Context, id string) (*capability.CapabilityVersion, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_id, version, prompt, model_policy, context_contract, knowledge,
		 memory, guardrails, tools, mcp_servers, runtime_policy, evaluation_suite, created_at, created_by
		 FROM capability_versions WHERE id = ?`, id,
	)
	return scanCapabilityVersion(row)
}

func (s *SQLite) ListVersions(ctx context.Context, capabilityID string) ([]*capability.CapabilityVersion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, capability_id, version, prompt, model_policy, context_contract, knowledge,
		 memory, guardrails, tools, mcp_servers, runtime_policy, evaluation_suite, created_at, created_by
		 FROM capability_versions WHERE capability_id = ? ORDER BY version DESC`, capabilityID,
	)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var result []*capability.CapabilityVersion
	for rows.Next() {
		v, err := scanCapabilityVersion(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (s *SQLite) GetLatestVersion(ctx context.Context, capabilityID string) (*capability.CapabilityVersion, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_id, version, prompt, model_policy, context_contract, knowledge,
		 memory, guardrails, tools, mcp_servers, runtime_policy, evaluation_suite, created_at, created_by
		 FROM capability_versions WHERE capability_id = ? ORDER BY version DESC LIMIT 1`, capabilityID,
	)
	return scanCapabilityVersion(row)
}

func scanCapabilityVersion(scanner interface {
	Scan(dest ...any) error
}) (*capability.CapabilityVersion, error) {
	var v capability.CapabilityVersion
	var promptJSON, modelPolicyJSON, contextContractJSON, knowledgeJSON, memoryJSON string
	var guardrailsJSON, toolsJSON, mcpJSON, runtimePolicyJSON, evalSuiteJSON string

	err := scanner.Scan(&v.ID, &v.CapabilityID, &v.Version,
		&promptJSON, &modelPolicyJSON, &contextContractJSON, &knowledgeJSON,
		&memoryJSON, &guardrailsJSON, &toolsJSON, &mcpJSON,
		&runtimePolicyJSON, &evalSuiteJSON, &v.CreatedAt, &v.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan version: %w", err)
	}

	mustUnmarshal([]byte(promptJSON), &v.Prompt)
	mustUnmarshal([]byte(modelPolicyJSON), &v.ModelPolicy)
	mustUnmarshal([]byte(contextContractJSON), &v.ContextContract)
	mustUnmarshal([]byte(knowledgeJSON), &v.Knowledge)
	mustUnmarshal([]byte(memoryJSON), &v.Memory)
	mustUnmarshal([]byte(guardrailsJSON), &v.Guardrails)
	mustUnmarshal([]byte(toolsJSON), &v.Tools)
	mustUnmarshal([]byte(mcpJSON), &v.MCPServers)
	mustUnmarshal([]byte(runtimePolicyJSON), &v.RuntimePolicy)
	mustUnmarshal([]byte(evalSuiteJSON), &v.EvaluationSuite)

	return &v, nil
}

// ---------------------------------------------------------------------------
// Executions
// ---------------------------------------------------------------------------

func (s *SQLite) CreateExecution(ctx context.Context, e *capability.Execution) error {
	inputs, err := marshalOrErr(e.Inputs)
	if err != nil {
		return fmt.Errorf("marshal execution inputs: %w", err)
	}
	outputs, err := marshalOrErr(e.Outputs)
	if err != nil {
		return fmt.Errorf("marshal execution outputs: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO executions
		 (id, capability_version_id, timestamp, inputs, outputs, model, provider,
		  latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
		  error, trace_id, environment)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.CapabilityVersionID, e.Timestamp, string(inputs), string(outputs),
		e.Model, e.Provider, e.LatencyMs, e.CostUSD, e.PromptTokens, e.CompletionTokens,
		e.TotalTokens, e.Error, e.TraceID, e.Environment,
	)
	if err != nil {
		return fmt.Errorf("insert execution: %w", err)
	}
	return nil
}

func (s *SQLite) GetExecution(ctx context.Context, id string) (*capability.Execution, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, capability_version_id, timestamp, inputs, outputs, model, provider,
		 latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
		 error, trace_id, environment
		 FROM executions WHERE id = ?`, id,
	)
	return scanExecution(row)
}

func (s *SQLite) ListExecutions(ctx context.Context, filter ExecutionFilter) ([]*capability.Execution, error) {
	query := `SELECT id, capability_version_id, timestamp, inputs, outputs, model, provider,
	 latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
	 error, trace_id, environment FROM executions WHERE 1=1`
	args := []any{}

	if filter.CapabilityVersionID != "" {
		query += " AND capability_version_id = ?"
		args = append(args, filter.CapabilityVersionID)
	}

	query += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	defer rows.Close()

	var result []*capability.Execution
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func scanExecution(scanner interface {
	Scan(dest ...any) error
}) (*capability.Execution, error) {
	var e capability.Execution
	var inputsJSON, outputsJSON string

	err := scanner.Scan(&e.ID, &e.CapabilityVersionID, &e.Timestamp,
		&inputsJSON, &outputsJSON, &e.Model, &e.Provider,
		&e.LatencyMs, &e.CostUSD, &e.PromptTokens, &e.CompletionTokens,
		&e.TotalTokens, &e.Error, &e.TraceID, &e.Environment,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan execution: %w", err)
	}

	mustUnmarshal([]byte(inputsJSON), &e.Inputs)
	mustUnmarshal([]byte(outputsJSON), &e.Outputs)

	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	return &e, nil
}
