-- Postgres migration 100: enable per-workspace Row Level Security.
--
-- Every per-workspace table (workspaces, projects, capabilities,
-- capability_versions, executions) is forced through SET LOCAL
-- app.current_workspace so a query that escapes its workspace
-- returns zero rows even when the WHERE clause is missing.
--
-- This migration is the database-side companion to
-- internal/store/postgres/postgres.go's withWorkspace helper.
-- Production tenants must run this before opening the daemon; the
-- test suite (gated on PROMPTSHEON_RUN_PG_TESTS=1) proves the
-- escaping query returns zero rows.

ALTER TABLE workspaces        ENABLE ROW LEVEL SECURITY;
ALTER TABLE projects         ENABLE ROW LEVEL SECURITY;
ALTER TABLE capabilities     ENABLE ROW LEVEL SECURITY;
ALTER TABLE capability_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE executions       ENABLE ROW LEVEL SECURITY;

-- The policies are permissive enough for testing and single-tenant
-- deployments. Multi-tenant production deployments REPLACE these
-- policies with role-aware policies that consult workspace_members.

CREATE POLICY workspace_isolation_select ON workspaces
    FOR SELECT USING (id = current_setting('app.current_workspace', true));

CREATE POLICY project_isolation ON projects
    FOR ALL USING (workspace_id = current_setting('app.current_workspace', true));

CREATE POLICY capability_isolation ON capabilities
    FOR ALL USING (workspace_id IN (
        SELECT id FROM workspaces WHERE id = current_setting('app.current_workspace', true)
    ));

CREATE POLICY version_isolation ON capability_versions
    FOR ALL USING (capability_id IN (
        SELECT c.id FROM capabilities c
        JOIN projects p ON p.id = c.project_id
        WHERE p.workspace_id = current_setting('app.current_workspace', true)
    ));

CREATE POLICY execution_isolation ON executions
    FOR ALL USING (capability_version_id IN (
        SELECT v.id FROM capability_versions v
        JOIN capabilities c ON c.id = v.capability_id
        JOIN projects p ON p.id = c.project_id
        WHERE p.workspace_id = current_setting('app.current_workspace', true)
    ));
