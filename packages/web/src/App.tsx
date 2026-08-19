import { HashRouter, Routes, Route } from 'react-router-dom';
import { Layout } from './components/Layout';
import { DashboardView } from './views/DashboardView';
import { WorkspaceList } from './views/WorkspaceList';
import { ProjectList } from './views/ProjectList';
import { CapabilityList } from './views/CapabilityList';
import { CapabilityDetail } from './views/CapabilityDetail';
import { ReleaseList } from './views/ReleaseList';
import { ExecutionView } from './views/ExecutionView';
import { ExecutionList } from './views/ExecutionList';
import { DatasetDetail } from './views/DatasetDetail';
import { EvalList } from './views/EvalList';
import { PreconditionList } from './views/PreconditionList';
import { AlertRuleList } from './views/AlertRuleList';
import { AlertList } from './views/AlertList';
import { ScheduleList } from './views/ScheduleList';
import { SelfEvolveView } from './views/SelfEvolveView';
import { CompilerView } from './views/CompilerView';
import { ManifestView } from './views/ManifestView';
import { ApprovalDetail } from './views/ApprovalDetail';
import { WebhookList } from './views/WebhookList';
import { UserList } from './views/UserList';
import { ApiKeyList } from './views/ApiKeyList';
import { FeatureFlagList } from './views/FeatureFlagList';
import { AuditList } from './views/AuditList';
import { OperationsHub } from './views/OperationsHub';
import { GoalsDashboard } from './views/GoalsDashboard';
import { SettingsView } from './views/SettingsView';
import { NotFound } from './views/NotFound';

export function App() {
  return (
    <HashRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<DashboardView />} />
          <Route path="/workspaces" element={<WorkspaceList />} />
          <Route path="/workspaces/:workspaceId/projects" element={<ProjectList />} />
          <Route path="/projects/:projectId/capabilities" element={<CapabilityList />} />
          <Route path="/capabilities/:capabilityId" element={<CapabilityDetail />} />
          <Route path="/capabilities/:capabilityId/releases" element={<ReleaseList />} />
          <Route path="/capabilities/:capabilityId/executions" element={<ExecutionList />} />
          <Route path="/capabilities/:capabilityId/datasets" element={<DatasetDetail />} />
          <Route path="/capabilities/:capabilityId/eval" element={<EvalList />} />
          <Route path="/capabilities/:capabilityId/self-evolve" element={<SelfEvolveView />} />
          <Route path="/capabilities/:capabilityId/preconditions" element={<PreconditionList />} />
          <Route path="/datasets/:id/cases" element={<DatasetDetail />} />
          <Route path="/manifests/:versionId" element={<ManifestView />} />
          <Route path="/approvals/:releaseId" element={<ApprovalDetail />} />
          <Route path="/compiler" element={<CompilerView />} />
          <Route path="/alerts/rules" element={<AlertRuleList />} />
          <Route path="/alerts/active" element={<AlertList />} />
          <Route path="/schedules" element={<ScheduleList />} />
          <Route path="/webhooks" element={<WebhookList />} />
          <Route path="/users" element={<UserList />} />
          <Route path="/api-keys" element={<ApiKeyList />} />
          <Route path="/feature-flags" element={<FeatureFlagList />} />
          <Route path="/audit" element={<AuditList />} />
          <Route path="/operations" element={<OperationsHub />} />
          <Route path="/goals" element={<GoalsDashboard />} />
          <Route path="/executions/new" element={<ExecutionView />} />
          <Route path="/executions/:id" element={<ExecutionView />} />
          <Route path="/settings" element={<SettingsView />} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Routes>
    </HashRouter>
  );
}
