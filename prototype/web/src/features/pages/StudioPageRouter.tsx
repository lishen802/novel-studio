import type { PageId } from '../../app/navigation'
import type { Project } from '../../domain/types'
import { AgentsPage, ReviewPage, RunsPage, VersionsPage, WorkflowPage } from './AgentPages'
import { CanonPage, CharactersPage, ManuscriptPage, OutlinePage, SettingsPage, WorldPage } from './AuthoringPages'
import { GraphPage, TimelinePage } from './VisualizationPages'
import { WorldMapPage } from '../map/WorldMapPage'

export function StudioPageRouter({ page, project, onChange, toast, createSnapshot, restoreSnapshot }: { page: PageId; project: Project; onChange: (project: Project) => void; toast: (message: string) => void; createSnapshot: (name: string) => string; restoreSnapshot: (id: string) => void }) {
  switch (page) {
    case 'manuscript': return <ManuscriptPage project={project} onChange={onChange} toast={toast} />
    case 'outline': return <OutlinePage project={project} onChange={onChange} toast={toast} />
    case 'characters': return <CharactersPage project={project} onChange={onChange} toast={toast} />
    case 'world': return <WorldPage project={project} onChange={onChange} />
    case 'canon': return <CanonPage project={project} onChange={onChange} toast={toast} />
    case 'timeline': return <TimelinePage project={project} onChange={onChange} toast={toast} />
    case 'graph': return <GraphPage project={project} onChange={onChange} toast={toast} />
    case 'map': return <WorldMapPage project={project} onChange={onChange} toast={toast} />
    case 'agents': return <AgentsPage project={project} onChange={onChange} toast={toast} />
    case 'workflow': return <WorkflowPage project={project} onChange={onChange} toast={toast} />
    case 'review': return <ReviewPage project={project} onChange={onChange} toast={toast} />
    case 'runs': return <RunsPage project={project} onChange={onChange} />
    case 'versions': return <VersionsPage project={project} onChange={onChange} createSnapshot={createSnapshot} restoreSnapshot={restoreSnapshot} toast={toast} />
    case 'settings': return <SettingsPage project={project} onChange={onChange} toast={toast} />
    default: return null
  }
}
