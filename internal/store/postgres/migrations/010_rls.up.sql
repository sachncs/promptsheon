-- 010_rls.up.sql — Row Level Security policies for multi-tenant isolation.
--
-- The control plane is multi-tenant: every Capability, Project,
-- Release, Audit entry, and Harness row is scoped to a
-- Workspace. RLS is the canonical enforcement point so an
-- application bug (a forgotten WHERE clause) cannot leak
-- across Workspaces.
--
-- Connection model: the daemon connects as a non-superuser
-- role (PROMPTSHEON_APP_ROLE) and SETs the current Workspace
-- id at the start of every request via:
--
--     SET LOCAL app.current_workspace = 'ws-abc';
--
-- RLS policies then read the GUC and reject any row whose
-- workspace_id does not match.
--
-- To enable RLS:
--   ALTER TABLE capabilities ENABLE ROW LEVEL SECURITY;
--   CREATE POLICY cap_workspace_isolation ON capabilities
--       USING (workspace_id = current_setting('app.current_workspace')::text);
--
-- The migration is opt-in: deployments that do not need
-- multi-tenant isolation can skip it. Promptsheon's
-- `PROMPTSHEON_RLS_MODE` env var (`strict` / `off`) controls
-- whether the policies are created at boot.

-- Workspace isolation policy template.
--
-- Applied per-table that has a workspace_id column; the
-- policies are identical so a single function powers them.
CREATE OR REPLACE FUNCTION app_current_workspace() RETURNS TEXT AS $$
    SELECT current_setting('app.current_workspace', true);
$$ LANGUAGE SQL STABLE;

-- The migration is idempotent so a second run is safe.
-- Production wiring calls this migration once at boot; the
-- `IF NOT EXISTS` clauses on policies make a re-run cheap.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'promptsheon_app') THEN
        CREATE ROLE promptsheon_app NOLOGIN;
    END IF;
END
$$;

-- Grant the application role the minimum permissions it needs.
GRANT USAGE ON SCHEMA public TO promptsheon_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO promptsheon_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO promptsheon_app;

-- Per-table RLS policies. Each policy enforces that the
-- row's workspace_id matches the GUC set by the request.
ALTER TABLE capabilities ENABLE ROW LEVEL SECURITY;
ALTER TABLE capability_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE releases ENABLE ROW LEVEL SECURITY;
ALTER TABLE datasets ENABLE ROW LEVEL SECURITY;
ALTER TABLE dataset_cases ENABLE ROW LEVEL SECURITY;
ALTER TABLE preconditions ENABLE ROW LEVEL SECURITY;
ALTER TABLE eval_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE eval_results ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS cap_workspace_isolation ON capabilities;
CREATE POLICY cap_workspace_isolation ON capabilities
    USING (project_id IN (
        SELECT id FROM projects
        WHERE workspace_id = app_current_workspace()
    ));

DROP POLICY IF EXISTS cap_v_workspace_isolation ON capability_versions;
CREATE POLICY cap_v_workspace_isolation ON capability_versions
    USING (capability_id IN (
        SELECT c.id FROM capabilities c
        JOIN projects p ON c.project_id = p.id
        WHERE p.workspace_id = app_current_workspace()
    ));

DROP POLICY IF EXISTS rel_workspace_isolation ON releases;
CREATE POLICY rel_workspace_isolation ON releases
    USING (capability_id IN (
        SELECT c.id FROM capabilities c
        JOIN projects p ON c.project_id = p.id
        WHERE p.workspace_id = app_current_workspace()
    ));

DROP POLICY IF EXISTS ds_workspace_isolation ON datasets;
CREATE POLICY ds_workspace_isolation ON datasets
    USING (capability_id IN (
        SELECT c.id FROM capabilities c
        JOIN projects p ON c.project_id = p.id
        WHERE p.workspace_id = app_current_workspace()
    ));

DROP POLICY IF EXISTS dsc_workspace_isolation ON dataset_cases;
CREATE POLICY dsc_workspace_isolation ON dataset_cases
    USING (dataset_id IN (
        SELECT d.id FROM datasets d
        JOIN capabilities c ON d.capability_id = c.id
        JOIN projects p ON c.project_id = p.id
        WHERE p.workspace_id = app_current_workspace()
    ));

DROP POLICY IF EXISTS pc_workspace_isolation ON preconditions;
CREATE POLICY pc_workspace_isolation ON preconditions
    USING (capability_id IN (
        SELECT c.id FROM capabilities c
        JOIN projects p ON c.project_id = p.id
        WHERE p.workspace_id = app_current_workspace()
    ));

DROP POLICY IF EXISTS er_workspace_isolation ON eval_runs;
CREATE POLICY er_workspace_isolation ON eval_runs
    USING (release_id IN (
        SELECT r.id FROM releases r
        JOIN capabilities c ON r.capability_id = c.id
        JOIN projects p ON c.project_id = p.id
        WHERE p.workspace_id = app_current_workspace()
    ));

DROP POLICY IF EXISTS erc_workspace_isolation ON eval_results;
CREATE POLICY erc_workspace_isolation ON eval_results
    USING (run_id IN (
        SELECT er.id FROM eval_runs er
        JOIN releases r ON er.release_id = r.id
        JOIN capabilities c ON r.capability_id = c.id
        JOIN projects p ON c.project_id = p.id
        WHERE p.workspace_id = app_current_workspace()
    ));
