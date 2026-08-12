import { useState, type FormEvent } from "react";
import type { StudioClient } from "../../lib/studio-client";

type ImportWizardDialogProps = {
  client: StudioClient;
  // 提交导入：App 层负责调用 chooseProjectParent 与 createImportedProject。
  // 失败时返回 rejected Promise，对话框据此展示结构化错误，不关闭。
  onSubmit(input: {
    sourcePath: string;
    title: string;
    genre: string;
    copyrightConfirmed: boolean;
  }): Promise<void>;
  onCancel(): void;
};

// ImportWizardDialog 是首页"导入小说并续写"入口的向导对话框。作者依次选择
// 源文件、填写书名并确认版权后，调用 onSubmit 触发后端 createImportedProject。
// 任一步失败时保留对话框并展示结构化错误，不伪造已完成效果。
export function ImportWizardDialog({
  client,
  onSubmit,
  onCancel,
}: ImportWizardDialogProps) {
  const [sourcePath, setSourcePath] = useState<string | null>(null);
  const [title, setTitle] = useState("");
  const [copyrightConfirmed, setCopyrightConfirmed] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // chooseFile 调用 client.chooseManuscriptToImport 选择源文件。应用不修改
  // 作者的原文件，只读取路径用于后续 createImportedProject 调用。
  const chooseFile = async () => {
    setError(null);
    try {
      const path = await client.chooseManuscriptToImport();
      if (path) setSourcePath(path);
    } catch (err) {
      setError(err instanceof Error ? err.message : "文件选择失败。");
    }
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!sourcePath || !title.trim() || !copyrightConfirmed || busy) return;
    setBusy(true);
    setError(null);
    try {
      await onSubmit({
        sourcePath,
        title: title.trim(),
        genre: "",
        copyrightConfirmed,
      });
    } catch (err) {
      // 后端返回的结构化错误（如 IMPORT_SOURCE_UNSAFE）在此展示，
      // 对话框保持打开，不关闭也不伪造完成。
      setError(err instanceof Error ? err.message : "导入失败，请重试。");
    } finally {
      setBusy(false);
    }
  };

  const canSubmit =
    Boolean(sourcePath) && title.trim().length > 0 && copyrightConfirmed && !busy;

  return (
    <div className="dialog-backdrop">
      <form
        className="project-dialog"
        role="dialog"
        aria-label="导入小说并续写"
        onSubmit={submit}
      >
        <p className="eyebrow">本地原稿续写</p>
        <h2>导入小说并续写</h2>
        <p className="welcome-card__description">
          选择已有的小说文件，应用会创建项目并导入确定性章节，不修改原文件。
        </p>
        {error && (
          <p role="alert" className="entry-error">
            {error}
          </p>
        )}
        <div className="dialog-actions">
          <button
            type="button"
            className="secondary-action"
            disabled={busy}
            onClick={() => void chooseFile()}
          >
            选择小说文件
          </button>
          {sourcePath && (
            <span className="dialog-path">已选择：{sourcePath}</span>
          )}
        </div>
        <label>
          书名
          <input
            value={title}
            onChange={(event) => setTitle(event.target.value)}
          />
        </label>
        <label className="dialog-checkbox">
          <input
            type="checkbox"
            checked={copyrightConfirmed}
            onChange={(event) => setCopyrightConfirmed(event.target.checked)}
          />
          我拥有这篇小说的版权
        </label>
        <div className="dialog-actions">
          <button
            type="button"
            className="secondary-action"
            disabled={busy}
            onClick={onCancel}
          >
            取消
          </button>
          <button
            type="submit"
            className="primary-action"
            disabled={!canSubmit}
          >
            {busy ? "正在导入…" : "确认导入"}
          </button>
        </div>
      </form>
    </div>
  );
}
