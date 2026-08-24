import { Routes, Route, Navigate } from 'react-router-dom';
import { Layout } from './components/Layout';
import Home from './pages/Home';
import Cockpit from './pages/Cockpit';
import Chapters from './pages/Chapters';
import Review from './pages/Review';
import Import from './pages/Import';
import Versions from './pages/Versions';
import Settings from './pages/Settings';
import ProjectSettings from './pages/ProjectSettings';
import Lab from './pages/Lab';
import ArchiveLayout from './pages/archive/ArchiveLayout';
import Outline from './pages/archive/Outline';
import Structure from './pages/archive/Structure';
import Characters from './pages/archive/Characters';
import World from './pages/archive/World';
import Timeline from './pages/archive/Timeline';
import Foreshadowing from './pages/archive/Foreshadowing';

function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Home />} />
        <Route path="cockpit" element={<Cockpit />} />
        <Route path="chapters" element={<Chapters />} />
        <Route path="review" element={<Review />} />
        <Route path="import" element={<Import />} />
        <Route path="versions" element={<Versions />} />
        <Route path="settings" element={<Settings />} />
        <Route path="project-settings" element={<ProjectSettings />} />
        <Route path="lab" element={<Lab />} />
        <Route path="archive" element={<ArchiveLayout />}>
          <Route index element={<Navigate to="outline" replace />} />
          <Route path="outline" element={<Outline />} />
          <Route path="structure" element={<Structure />} />
          <Route path="characters" element={<Characters />} />
          <Route path="world" element={<World />} />
          <Route path="timeline" element={<Timeline />} />
          <Route path="foreshadowing" element={<Foreshadowing />} />
        </Route>
      </Route>
    </Routes>
  );
}

export default App;
