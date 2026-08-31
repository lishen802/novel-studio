import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ArrowLeft, FlaskConical, Play, RefreshCw, Square, Trash2, TriangleAlert } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { Button } from '../components/ui/Button';
import { Modal } from '../components/ui/Modal';
import { studio, type HarnessCaseView, type HarnessDatasetView, type HarnessPreviewView, type HarnessRunView } from '../api/studio';
import { useApp } from '../store/useApp';
import { formatBudget, reasoningLabel } from '../budget';

const localRaterID = 'local-author';
const acceptanceOptions = [
  ['direct', '可直接接受'], ['minor_changes', '少量修改'], ['major_changes', '大量修改'], ['rewrite', '整章/整案重写'],
] as const;

type RatingDraft = Record<string, { scores: Record<string, number>; acceptance: string; note: string }>;

export default function Lab() {
  const navigate = useNavigate();
  const { state } = useApp();
  const [catalog, setCatalog] = useState<HarnessDatasetView | null>(null);
  const [runs, setRuns] = useState<HarnessRunView[]>([]);
  const [selectedRun, setSelectedRun] = useState<HarnessRunView | null>(null);
  const [caseView, setCaseView] = useState<HarnessCaseView | null>(null);
  const [profile, setProfile] = useState('');
  const [variant, setVariant] = useState('minimal');
  const [mode, setMode] = useState('reduced');
  const [selectedCaseIDs, setSelectedCaseIDs] = useState<string[]>([]);
  const [preview, setPreview] = useState<HarnessPreviewView | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<HarnessRunView | null>(null);
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [confirmed, setConfirmed] = useState(false);
  const [ratings, setRatings] = useState<RatingDraft>({});
  const [baseRunID, setBaseRunID] = useState('');
  const [comparison, setComparison] = useState<Record<string, unknown> | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const caseHeading = useRef<HTMLHeadingElement>(null);

  const refresh = useCallback(async () => {
    try {
      const [nextCatalog, nextRuns] = await Promise.all([studio.listCreativeHarnessDatasets(), studio.listCreativeHarnessRuns()]);
      setCatalog(nextCatalog);
      setRuns(nextRuns);
      setSelectedRun((current) => current ? nextRuns.find((item) => item.run.id === current.run.id) || nextRuns[0] || null : nextRuns[0] || null);
      setError('');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '实验室加载失败');
    }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);
  useEffect(() => {
    const timer = window.setInterval(() => {
      if (selectedRun?.run.status === 'running') void refresh();
    }, 1800);
    return () => window.clearInterval(timer);
  }, [refresh, selectedRun?.run.status]);
  useEffect(() => {
    if (!profile && state.bootstrap?.profiles[0]) setProfile(`${state.bootstrap.profiles[0].profileId}:${state.bootstrap.profiles[0].version}`);
  }, [profile, state.bootstrap?.profiles]);
  useEffect(() => {
    if (!catalog) return;
    setSelectedCaseIDs(catalog.dataset.cases.filter((item) => mode === 'full' || Boolean(item.ci_reduced)).map((item) => String(item.case_id)));
  }, [catalog, mode]);

  const selectedProfile = useMemo(() => {
    const [profileId, version] = profile.split(':');
    return { profileId, profileVersion: Number(version || 0) };
  }, [profile]);

  async function previewRun() {
    setBusy(true); setError('');
    try {
      const result = await studio.previewCreativeHarnessRun({ ...selectedProfile, variantId: variant, mode, caseIds: selectedCaseIDs, seed: 20260824 });
      setPreview(result); setConfirmed(false); setConfirmOpen(true);
    } catch (cause) { setError(cause instanceof Error ? cause.message : '运行预览失败'); }
    finally { setBusy(false); }
  }

  async function createAndRun() {
    if (!preview) return;
    setBusy(true); setError('');
    try {
      const created = await studio.createCreativeHarnessRun({ ...selectedProfile, variantId: variant, mode, caseIds: selectedCaseIDs, seed: 20260824, confirmationDigest: preview.confirmationDigest, costConfirmed: confirmed });
      await studio.executeCreativeHarnessRun(created.run.id);
      setConfirmOpen(false); setSelectedRun(created); await refresh();
    } catch (cause) { setError(cause instanceof Error ? cause.message : '创建运行失败'); }
    finally { setBusy(false); }
  }

  async function loadCase(caseId: string) {
    if (!selectedRun) return;
    setBusy(true); setError('');
    try {
      const loaded = await studio.getCreativeHarnessCaseResult(selectedRun.run.id, caseId, localRaterID);
      setCaseView(loaded);
      const draft: RatingDraft = {};
      for (const candidate of loaded.candidates) {
        draft[candidate.slot] = { scores: Object.fromEntries(loaded.case.rating_dimensions.map((dimension) => [dimension, 3])), acceptance: 'minor_changes', note: '' };
      }
      setRatings(draft);
      window.setTimeout(() => caseHeading.current?.focus(), 0);
    } catch (cause) { setError(cause instanceof Error ? cause.message : '结果加载失败'); }
    finally { setBusy(false); }
  }

  async function submitRating() {
    if (!selectedRun || !caseView) return;
    setBusy(true); setError('');
    try {
      const revealed = await studio.saveCreativeHarnessRating({
        runId: selectedRun.run.id, caseId: caseView.case.case_id, localRaterId: localRaterID,
        ratings: caseView.candidates.map((candidate) => ({ slot: candidate.slot, ...ratings[candidate.slot] })),
      });
      setCaseView(revealed); await refresh();
    } catch (cause) { setError(cause instanceof Error ? cause.message : '评分保存失败'); }
    finally { setBusy(false); }
  }

  async function compareRuns() {
    if (!selectedRun || !baseRunID) return;
    setBusy(true); setError('');
    try { setComparison(await studio.compareCreativeHarnessRuns(baseRunID, selectedRun.run.id)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : '版本比较失败'); }
    finally { setBusy(false); }
  }

  async function exportReport() {
    if (!selectedRun) return;
    setBusy(true); setError('');
    try {
      const payload = await studio.exportCreativeHarnessReport(selectedRun.run.id);
      const url = URL.createObjectURL(new Blob([payload], { type: 'application/json' }));
      const anchor = document.createElement('a');
      anchor.href = url; anchor.download = `harness-${selectedRun.run.id}.json`; anchor.click();
      URL.revokeObjectURL(url);
    } catch (cause) { setError(cause instanceof Error ? cause.message : '报告导出失败'); }
    finally { setBusy(false); }
  }

  async function confirmDeleteRun() {
    if (!deleteTarget) return;
    setDeleteBusy(true); setError('');
    try {
      const deletedID = deleteTarget.run.id;
      await studio.deleteCreativeHarnessRun(deletedID);
      setDeleteTarget(null);
      setCaseView(null);
      setComparison(null);
      setBaseRunID((current) => current === deletedID ? '' : current);
      await refresh();
    } catch (cause) { setError(cause instanceof Error ? cause.message : '删除实验运行失败'); }
    finally { setDeleteBusy(false); }
  }

  const completedCaseIDs = selectedRun?.completedCaseIds || [];
  const reportStats: Array<[string, unknown]> = selectedRun?.report ? [
    ['Case', selectedRun.report.case_count], ['候选', selectedRun.report.candidate_count],
    ['通过', selectedRun.report.passed_candidates], ['输入 Token', selectedRun.report.input_tokens],
    ['输出 Token', selectedRun.report.output_tokens],
  ] : [];

  return (
    <div className="max-w-[1500px] mx-auto space-y-5 animate-fade-up pb-10">
      <header className="flex items-start justify-between gap-5">
        <div>
          <button onClick={() => navigate('/')} className="inline-flex items-center gap-1 text-sm text-text-sub hover:text-text mb-3"><ArrowLeft className="w-4 h-4" />返回首页</button>
          <div className="flex items-center gap-3"><FlaskConical className="w-7 h-7 text-primary" /><h1 className="text-2xl font-display font-semibold text-text">智能体实验室</h1></div>
          <p className="text-sm text-text-sub mt-2">固定原创测试集、真实模型运行、匿名盲评与版本回归。</p>
        </div>
        <div role="note" className="max-w-md rounded-xl border border-warning/25 bg-warning/10 px-4 py-3 text-sm text-text-sub">
          <strong className="text-text">隔离实验：</strong>实验结果不会写入作品正文或任何正式资产。
        </div>
      </header>

      {error && <div role="alert" className="flex items-center justify-between rounded-lg border border-danger/25 bg-danger/10 px-4 py-3 text-sm text-danger"><span className="flex items-center gap-2"><TriangleAlert className="w-4 h-4" />{error}</span><button onClick={() => void refresh()} className="underline">重试加载</button></div>}

      <section aria-labelledby="lab-config" className="bg-surface border border-border rounded-xl p-5 shadow-sm">
        <div className="flex items-center justify-between mb-4"><h2 id="lab-config" className="font-medium text-text">1. 测试集与运行配置</h2><span className="text-xs text-text-muted">{catalog ? `${catalog.dataset.dataset_id}@${catalog.dataset.version} · ${catalog.dataset.cases.length} cases` : '加载中…'}</span></div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <label className="text-sm text-text-sub">参考创作方案<select value={profile} onChange={(event) => setProfile(event.target.value)} className="mt-1 w-full rounded-md border border-border bg-surface px-3 py-2 text-text"><option value="">请选择</option>{state.bootstrap?.profiles.map((item) => <option key={`${item.profileId}:${item.version}`} value={`${item.profileId}:${item.version}`}>{item.name} · v{item.version}</option>)}</select></label>
          <label className="text-sm text-text-sub">工作流变体<select value={variant} onChange={(event) => setVariant(event.target.value)} className="mt-1 w-full rounded-md border border-border bg-surface px-3 py-2 text-text">{catalog?.variants.map((item) => <option key={item.id} value={item.id}>{item.id} · {item.workflow_id}@{item.workflow_version}</option>)}</select></label>
          <label className="text-sm text-text-sub">测试范围<select value={mode} onChange={(event) => setMode(event.target.value)} className="mt-1 w-full rounded-md border border-border bg-surface px-3 py-2 text-text"><option value="reduced">快速集</option><option value="full">完整 12 cases</option></select></label>
        </div>
        {(() => {
          const selected = state.bootstrap?.profiles.find((item) => `${item.profileId}:${item.version}` === profile);
          if (!selected) return null;
          return (
            <p className="mt-3 text-xs text-text-muted">
              使用方案冻结预算：运行 Token {formatBudget(selected.maxRunTokens, Boolean(selected.runBudgetUnlimited))} · 上下文字符 {formatBudget(selected.maxContextChars, Boolean(selected.contextBudgetUnlimited))}。测试不再单独设置限额。
            </p>
          );
        })()}
        <div className="mt-4"><Button variant="primary" onClick={() => void previewRun()} disabled={busy || !profile || !catalog || selectedCaseIDs.length === 0} className="w-full md:w-auto">预览费用与运行</Button></div>
        {catalog && <fieldset className="mt-4"><legend className="text-sm font-medium text-text mb-2">选择 case（{selectedCaseIDs.length}）</legend><div className="grid grid-cols-2 lg:grid-cols-4 gap-2">{catalog.dataset.cases.filter((item) => mode === 'full' || Boolean(item.ci_reduced)).map((item) => { const id = String(item.case_id); return <label key={id} className="flex items-start gap-2 rounded-md border border-border p-2 text-xs text-text-sub"><input type="checkbox" checked={selectedCaseIDs.includes(id)} onChange={(event) => setSelectedCaseIDs((current) => event.target.checked ? [...current, id] : current.filter((value) => value !== id))}/><span>{id}<br/><span className="text-text-muted">{String(item.genre)} · {String(item.task_kind)}</span></span></label>; })}</div></fieldset>}
      </section>

      <div className="grid grid-cols-1 xl:grid-cols-[360px_1fr] gap-5">
        <section aria-labelledby="lab-runs" className="bg-surface border border-border rounded-xl p-4 shadow-sm">
          <div className="flex items-center justify-between mb-3"><h2 id="lab-runs" className="font-medium text-text">2. 运行观察器</h2><button aria-label="刷新运行" onClick={() => void refresh()}><RefreshCw className="w-4 h-4 text-text-muted" /></button></div>
          {runs.length === 0 ? <p className="text-sm text-text-muted py-8 text-center">尚无实验运行</p> : <div className="space-y-2">{runs.map((item) => <div key={item.run.id} className={`group relative rounded-lg border ${selectedRun?.run.id === item.run.id ? 'border-primary bg-primary-subtle' : 'border-border hover:bg-surface-hover'}`}><button onClick={() => { setSelectedRun(item); setCaseView(null); }} className="w-full p-3 pr-10 text-left"><div className="flex justify-between text-sm"><span className="font-medium text-text">{String(item.run.binding.variant_id || 'minimal')}</span><span className="text-text-sub">{item.run.status}</span></div><div className="mt-2 h-1.5 rounded-full bg-surface-hover overflow-hidden"><div className="h-full bg-primary" style={{ width: `${item.totalCases ? item.completedCases / item.totalCases * 100 : 0}%` }} /></div><p className="text-xs text-text-muted mt-2">{item.completedCases}/{item.totalCases} cases · {item.run.public_stage_id || '等待开始'} · {(item.run.input_tokens || 0) + (item.run.output_tokens || 0)} Token · {item.run.calls || 0} 次调用</p></button>{item.run.status !== 'running' && <button aria-label={`删除运行 ${item.run.id}`} title="删除运行" onClick={() => setDeleteTarget(item)} className="absolute right-2 top-2 rounded-md p-1.5 text-text-muted opacity-70 hover:bg-danger/10 hover:text-danger hover:opacity-100 focus:opacity-100"><Trash2 className="h-4 w-4" /></button>}</div>)}</div>}
          {selectedRun && <div className="flex gap-2 mt-4">{selectedRun.run.status === 'running' ? <Button onClick={() => void studio.cancelCreativeHarnessRun(selectedRun.run.id).then(refresh)}><Square className="w-3.5 h-3.5" />取消</Button> : ['queued', 'recoverable', 'failed', 'cancelled'].includes(selectedRun.run.status) ? <Button variant="primary" onClick={() => void studio.executeCreativeHarnessRun(selectedRun.run.id).then(refresh)}><Play className="w-3.5 h-3.5" />{selectedRun.completedCases ? '恢复运行' : '开始运行'}</Button> : null}</div>}
          {selectedRun?.run.failure && <p className="mt-3 text-xs text-danger">{selectedRun.run.failure}</p>}
        </section>

        <section aria-labelledby="lab-results" className="bg-surface border border-border rounded-xl p-5 shadow-sm min-h-[380px]">
          <h2 id="lab-results" className="font-medium text-text mb-4">3. 匿名结果与人工盲评</h2>
          {!selectedRun ? <p className="text-sm text-text-muted">选择一个运行查看结果。</p> : !caseView ? <div><p className="text-sm text-text-sub mb-3">选择已完成的 case。评分提交前不显示模型、工作流或 baseline / initial / revised 身份。</p><div className="flex flex-wrap gap-2">{completedCaseIDs.map((id) => <button key={id} onClick={() => void loadCase(id)} className="rounded-md border border-border px-3 py-2 text-sm hover:border-border-strong">{id}</button>)}</div></div> : <div>
            <h3 ref={caseHeading} tabIndex={-1} className="font-display text-xl text-text focus:outline-none">{caseView.case.case_id} · {caseView.case.genre}</h3>
            <p className="text-sm text-text-sub mt-1 mb-5">{caseView.case.chapter_goal || caseView.case.task_kind}</p>
            <div className="space-y-5">{caseView.candidates.map((candidate) => <article key={candidate.slot} className="rounded-xl border border-border p-4"><div className="flex justify-between mb-3"><h4 className="font-medium text-text">{candidate.slot}</h4>{caseView.revealed?.[candidate.slot] && <span className="rounded-full bg-success/10 text-success px-2 py-0.5 text-xs">已揭示：{caseView.revealed[candidate.slot]}</span>}</div><pre className="whitespace-pre-wrap font-display text-[15px] leading-7 text-text max-h-80 overflow-auto bg-atmosphere rounded-lg p-4">{candidate.content}</pre>{candidate.quality_report_json && <details className="mt-3 rounded-lg border border-border p-3 text-xs text-text-sub"><summary className="cursor-pointer font-medium text-text">查看公开问题与正文定位</summary><pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap">{candidate.quality_report_json}</pre></details>}{!caseView.rating && ratings[candidate.slot] && <fieldset className="mt-4"><legend className="text-sm font-medium text-text mb-2">为 {candidate.slot} 评分</legend><div className="grid grid-cols-2 lg:grid-cols-3 gap-2">{caseView.case.rating_dimensions.map((dimension) => <label key={dimension} className="text-xs text-text-sub">{dimension}<select aria-label={`${candidate.slot} ${dimension}`} value={ratings[candidate.slot].scores[dimension]} onChange={(event) => setRatings((current) => ({ ...current, [candidate.slot]: { ...current[candidate.slot], scores: { ...current[candidate.slot].scores, [dimension]: Number(event.target.value) } } }))} className="ml-2 rounded border border-border bg-surface px-2 py-1">{[1,2,3,4,5].map((score) => <option key={score}>{score}</option>)}</select></label>)}</div><label className="block text-sm text-text-sub mt-3">接受决定<select value={ratings[candidate.slot].acceptance} onChange={(event) => setRatings((current) => ({ ...current, [candidate.slot]: { ...current[candidate.slot], acceptance: event.target.value } }))} className="ml-2 rounded border border-border bg-surface px-2 py-1">{acceptanceOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label></fieldset>}</article>)}</div>
            {!caseView.rating ? <Button variant="primary" className="mt-5" onClick={() => void submitRating()} disabled={busy}>提交整组评分并揭示来源</Button> : <div className="mt-5 space-y-3"><div className="rounded-lg border border-success/20 bg-success/10 p-3 text-sm text-success">评分已保存。候选身份与本次冻结模型/工作流现已揭示，重复提交会被拒绝。</div>{caseView.binding && <details className="rounded-lg border border-border p-3 text-xs text-text-sub"><summary className="cursor-pointer font-medium text-text">查看冻结模型与工作流来源</summary><pre className="mt-2 overflow-auto whitespace-pre-wrap">{JSON.stringify(caseView.binding, null, 2)}</pre></details>}</div>}
          </div>}
        </section>
      </div>

      {selectedRun?.report && <section className="bg-surface border border-border rounded-xl p-5 shadow-sm"><div className="flex items-center justify-between gap-3 mb-3"><h2 className="font-medium text-text">4. 回归报告</h2><Button size="sm" onClick={() => void exportReport()} disabled={busy}>导出 JSON 报告</Button></div><div className="grid grid-cols-2 md:grid-cols-5 gap-3 text-sm">{reportStats.map(([label, value]) => <div key={label} className="rounded-lg bg-atmosphere p-3"><p className="text-text-muted text-xs">{label}</p><p className="text-lg text-text mt-1">{String(value ?? 0)}</p></div>)}</div><div className="mt-4 rounded-lg border border-border p-3 text-sm text-text-sub">人工盲评：{selectedRun.ratings?.submissions || 0} 组 · 可直接接受/少量修改 {selectedRun.ratings?.candidateRatings ? Math.round((selectedRun.ratings.acceptable / selectedRun.ratings.candidateRatings) * 100) : 0}% · 重写 {selectedRun.ratings?.candidateRatings ? Math.round((selectedRun.ratings.rewrite / selectedRun.ratings.candidateRatings) * 100) : 0}%</div><p className="text-xs text-text-muted mt-4">导出包含冻结模型与资源版本、内容哈希、门禁、用量和退化证据，不包含 API Key、完整 Prompt 或生成正文。版本比较保留退化 case，不以总平均掩盖。</p></section>}

      {selectedRun?.report && <section aria-labelledby="lab-compare" className="bg-surface border border-border rounded-xl p-5 shadow-sm"><h2 id="lab-compare" className="font-medium text-text mb-3">5. 版本比较与退化下钻</h2><div className="flex flex-wrap items-end gap-3"><label className="text-sm text-text-sub">基准运行<select value={baseRunID} onChange={(event) => setBaseRunID(event.target.value)} className="block mt-1 min-w-72 rounded-md border border-border bg-surface px-3 py-2"><option value="">选择另一条已完成运行</option>{runs.filter((item) => item.report && item.run.id !== selectedRun.run.id).map((item) => <option key={item.run.id} value={item.run.id}>{String(item.run.binding.variant_id || 'minimal')} · {item.run.id.slice(0, 8)}</option>)}</select></label><Button onClick={() => void compareRuns()} disabled={!baseRunID || busy}>比较冻结版本</Button></div>{comparison && <pre aria-label="版本比较结果" className="mt-4 max-h-72 overflow-auto whitespace-pre-wrap rounded-lg bg-atmosphere p-4 text-xs text-text-sub">{JSON.stringify(comparison, null, 2)}</pre>}</section>}

      <Modal open={Boolean(deleteTarget)} onClose={() => !deleteBusy && setDeleteTarget(null)} title="确认删除实验运行">
        <div className="space-y-4">
          <p className="text-sm text-text-sub">确认删除这条实验运行？此操作会同时删除对应的结果、评分和报告，且无法撤销。</p>
          <p className="rounded-md bg-surface-hover px-3 py-2 text-xs text-text-muted">{deleteTarget?.run.id}</p>
          <div className="flex justify-end gap-2"><Button variant="ghost" onClick={() => setDeleteTarget(null)} disabled={deleteBusy}>取消</Button><Button variant="danger" onClick={() => void confirmDeleteRun()} disabled={deleteBusy}>{deleteBusy ? '删除中...' : '确认删除运行'}</Button></div>
        </div>
      </Modal>

      <Modal open={confirmOpen} onClose={() => setConfirmOpen(false)} title="确认真实模型运行">
        {preview && <div className="space-y-4">
          <div className="border border-border rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-surface-hover text-text-sub">
                <tr>
                  <th className="text-left px-3 py-2 font-medium">角色</th>
                  <th className="text-left px-3 py-2 font-medium">Provider</th>
                  <th className="text-left px-3 py-2 font-medium">模型</th>
                  <th className="text-left px-3 py-2 font-medium">推理强度</th>
                </tr>
              </thead>
              <tbody>
                {(preview.roles || []).map((row) => (
                  <tr key={row.role} className="border-t border-border">
                    <td className="px-3 py-2 text-text">{row.role}</td>
                    <td className="px-3 py-2 text-text-sub">{row.provider}</td>
                    <td className="px-3 py-2 text-text-sub">{row.model}</td>
                    <td className="px-3 py-2 text-text-sub">{reasoningLabel(row.reasoning_effort || 'auto')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="grid grid-cols-2 gap-3 text-sm">
            <p>最大调用次数<br/><strong>{preview.estimatedCalls}</strong></p>
            <p>单次运行 Token 预算<br/><strong>{formatBudget(preview.maxRunTokens || 0, Boolean(preview.runBudgetUnlimited))}</strong></p>
            <p>本次实验最大 Token<br/><strong>{formatBudget(preview.maximumOutputTokens || 0, Boolean(preview.runBudgetUnlimited))}</strong></p>
            <p>上下文字符预算<br/><strong>{formatBudget(preview.maxContextChars || 0, Boolean(preview.contextBudgetUnlimited))}</strong></p>
            <p>工作流<br/><strong>{preview.workflowId}@{preview.workflowVersion}</strong></p>
          </div>
          <p className="rounded-lg bg-warning/10 p-3 text-xs text-text-sub">{preview.currencyEstimate}</p>
          <label className="flex items-start gap-2 text-sm text-text"><input type="checkbox" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} className="mt-1"/>我确认上述 Provider、模型、推理强度与方案预算，并同意发起可能产生费用的真实调用。</label>
          <div className="flex justify-end gap-2"><Button onClick={() => setConfirmOpen(false)}>取消</Button><Button variant="primary" disabled={!confirmed || busy} onClick={() => void createAndRun()}>创建并运行</Button></div>
        </div>}
      </Modal>
    </div>
  );
}
