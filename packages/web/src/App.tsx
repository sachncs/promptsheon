import { HashRouter, Routes, Route } from 'react-router-dom';
import { Layout } from './components/Layout';
import { DashboardView } from './views/DashboardView';
import { WorkspaceList } from './views/WorkspaceList';
import { ProjectList } from './views/ProjectList';
import { CapabilityList } from './views/CapabilityList';
import { CapabilityDetail } from './views/CapabilityDetail';
import { OperationsHub } from './views/OperationsHub';
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
          <Route path="/operations" element={<OperationsHub />} />
          <Route path="/settings" element={<SettingsView />} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Routes>
    </HashRouter>
  );
}
