import { useState } from 'react';
import { AlertCircle, FileText, FolderOpen, Loader2, Upload } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { studio, type ChapterView, type ProjectView, type WorkbenchProjection } from '../api/studio';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { useApp } from '../store/useApp';

interface ImportedProjectResult {
  project: ProjectView;
  chapters: ChapterView[];
  workbench: WorkbenchProjection;
}

/** 导入入口使用后端的一体化用例，任一步失败都不会伪造已导入状态。 */
export default function Import() {
  const navigate = useNavigate();
  const { dispatch, refreshBootstrap } = useApp();
  const [sourcePath, setSourcePath] = useState('');
  const [title, setTitle] = useState('');
  const [genre, setGenre] = useState('');
  const [copyrightConfirmed, setCopyrightConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const chooseSource = async () => {
    try {
      const path = await studio.chooseManuscriptToImport();
      if (!path) return;
      setSourcePath(path);
      const filename = path.split(/[\\/]/).pop() || '';
      if (!title) setTitle(filename.replace(/\.(txt|md|markdown)$/i, ''));
      setError(null);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '无法选择小说文件');
    }
  };

  const importProject = async () => {
    if (!sourcePath || !title.trim() || !copyrightConfirmed) return;
    setBusy(true);
    try {
      const parentDirectory = await studio.chooseProjectParent();
      if (!parentDirectory) return;
      const result = await studio.createImportedProject({
        sourcePath,
        parentDirectory,
        title: title.trim(),
        genre: genre.trim(),
        copyrightConfirmed,
      }) as unknown as ImportedProjectResult;
      dispatch({ type: 'SET_BACKEND_PROJECT', payload: result.project });
      await refreshBootstrap();
      navigate('/chapters');
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '导入失败，请检查文件后重试');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="max-w-2xl mx-auto space-y-6 animate-fade-up">
      <div>
        <h1 className="text-2xl font-display text-text">导入小说并续写</h1>
        <p className="text-sm text-text-sub mt-1">确定性切分和正文写入在本地完成，不会自动调用模型。</p>
      </div>

      <Card padding="lg">
        <button
          onClick={() => void chooseSource()}
          className="w-full border-2 border-dashed border-border rounded-xl p-10 text-center hover:border-primary/40 hover:bg-primary-subtle transition-colors"
        >
          <div className="w-12 h-12 rounded-full bg-surface border border-border flex items-center justify-center mx-auto mb-4">
            <FolderOpen className="w-6 h-6 text-text-muted" />
          </div>
          <p className="text-sm font-medium text-text">选择 TXT 或 Markdown 小说文件</p>
          <p className="text-xs text-text-sub mt-1">点击后使用系统文件选择器</p>
          {sourcePath && (
            <span className="mt-4 inline-flex items-center gap-2 px-3 py-1.5 bg-surface border border-border rounded-md text-xs text-text">
              <FileText className="w-3.5 h-3.5" />{sourcePath}
            </span>
          )}
        </button>

        <div className="grid grid-cols-2 gap-4 mt-6">
          <div>
            <label className="text-xs font-medium text-text-sub mb-1.5 block">书名</label>
            <Input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="作品名称" />
          </div>
          <div>
            <label className="text-xs font-medium text-text-sub mb-1.5 block">题材</label>
            <Input value={genre} onChange={(event) => setGenre(event.target.value)} placeholder="可选" />
          </div>
        </div>

        <label className="flex items-start gap-3 mt-5 cursor-pointer">
          <input
            type="checkbox"
            checked={copyrightConfirmed}
            onChange={(event) => setCopyrightConfirmed(event.target.checked)}
            className="mt-0.5 w-4 h-4"
          />
          <span className="text-sm text-text-sub">我确认拥有处理该材料的权利，或已获得必要授权。</span>
        </label>

        <div className="mt-5 flex items-start gap-3 p-3 bg-primary-subtle border border-primary/10 rounded-lg">
          <AlertCircle className="w-4 h-4 text-primary shrink-0 mt-0.5" />
          <p className="text-xs text-text-sub">导入完成后会直接进入真实正文页面。整书档案生成仍属后续阶段，不会显示模拟进度或候选。</p>
        </div>

        {error && <p className="mt-4 text-sm text-danger">{error}</p>}

        <div className="flex justify-end mt-6">
          <Button
            variant="primary"
            size="lg"
            onClick={() => void importProject()}
            disabled={!sourcePath || !title.trim() || !copyrightConfirmed || busy}
          >
            {busy ? <Loader2 className="w-4 h-4 animate-spin" /> : <Upload className="w-4 h-4" />}
            {busy ? '正在导入…' : '开始导入'}
          </Button>
        </div>
      </Card>
    </div>
  );
}
