/**
 * useJobEventReplay subscribes to job:updated notifications and drives
 * cursor-based event replay through the jobEventReducer. Each job's replay
 * state is tracked independently so the workbench can render status, steps,
 * and accumulated token usage incrementally without polling.
 *
 * The hook exposes:
 * - replayStates: per-job JobReplayState keyed by jobId
 * - pullJobEvents: manually trigger a pull (e.g., on mount or selection)
 * - resetJob: drop a job's replay state (e.g., on close)
 *
 * Behaviour:
 * - On `job:updated`, the hook calls getJobEvents(jobId, cursor, limit) and
 *   applies events via the reducer.
 * - If the page reports snapshotRequired, the hook rebuilds state from
 *   getJobSnapshot and continues pulling from the new cursor.
 * - If the reducer detects a gap, the hook re-pulls from gapCursor.
 * - If hasMore is true, the hook paginates until caught up.
 * - A per-job pulling guard prevents concurrent pulls from stacking up
 *   when duplicate notifications arrive.
 */
import { useCallback, useEffect, useRef, useState } from "react";
import type { StudioClient } from "../../lib/studio-client";
import {
  applyEvents,
  initialState,
  rebuildFromSnapshot,
  type JobKind,
  type JobReplayState,
} from "./jobEventReducer";

export interface UseJobEventReplayOptions {
  client: StudioClient;
  /**
   * Called once when a job transitions to a terminal replay state. The
   * workbench uses this to refresh the canonical view (for changeSetId)
   * and surface a notice.
   */
  onJobTerminal?: (
    jobId: string,
    jobKind: JobKind,
    state: JobReplayState,
  ) => void;
  /**
   * Filter notifications by job kind. Default: handle both agent and
   * extraction jobs.
   */
  jobKindFilter?: ReadonlySet<JobKind>;
  /**
   * Page size for getJobEvents. Default: 50.
   */
  pageSize?: number;
  /**
   * Maximum pull iterations per notification to prevent infinite loops.
   * Default: 100.
   */
  maxIterations?: number;
}

export interface UseJobEventReplayResult {
  replayStates: Record<string, JobReplayState>;
  pullJobEvents: (jobId: string, jobKind: JobKind) => Promise<void>;
  resetJob: (jobId: string) => void;
}

const DEFAULT_PAGE_SIZE = 50;
const DEFAULT_MAX_ITERATIONS = 100;

export function useJobEventReplay({
  client,
  onJobTerminal,
  jobKindFilter,
  pageSize = DEFAULT_PAGE_SIZE,
  maxIterations = DEFAULT_MAX_ITERATIONS,
}: UseJobEventReplayOptions): UseJobEventReplayResult {
  const [replayStates, setReplayStates] = useState<
    Record<string, JobReplayState>
  >({});
  // Ref mirror so the async pull sees the latest state without re-subscribing
  // to onJobUpdated whenever replayStates changes.
  const replayStatesRef = useRef<Record<string, JobReplayState>>({});
  useEffect(() => {
    replayStatesRef.current = replayStates;
  }, [replayStates]);
  // Per-job pulling guard dedupes concurrent reads. pendingTargetSequence keeps
  // the greatest notification observed while that read is in flight, so a
  // best-effort Wails wake-up can never be discarded by the guard.
  const pullingRef = useRef<Set<string>>(new Set());
  const pendingTargetSequenceRef = useRef<Map<string, number>>(new Map());
  // Track terminal transitions so onJobTerminal fires once per job.
  const terminalNotifiedRef = useRef<Set<string>>(new Set());
  // Keep latest onJobTerminal in a ref so the onJobUpdated subscription
  // doesn't need to be re-created when the callback identity changes.
  const onJobTerminalRef = useRef(onJobTerminal);
  useEffect(() => {
    onJobTerminalRef.current = onJobTerminal;
  }, [onJobTerminal]);
  const clientRef = useRef(client);
  useEffect(() => {
    clientRef.current = client;
  }, [client]);

  const pullJobEvents = useCallback(
    async (
      jobId: string,
      jobKind: JobKind,
      latestSequence = 0,
    ): Promise<void> => {
      if (!jobId) return;
      const priorTarget = pendingTargetSequenceRef.current.get(jobId) ?? 0;
      pendingTargetSequenceRef.current.set(
        jobId,
        Math.max(priorTarget, latestSequence),
      );
      if (pullingRef.current.has(jobId)) return;
      pullingRef.current.add(jobId);
      let finalCursor = replayStatesRef.current[jobId]?.cursor ?? 0;
      try {
        let state = replayStatesRef.current[jobId] ?? initialState();
        let cursor = state.cursor;
        let iteration = 0;

        while (iteration < maxIterations) {
          iteration++;
          const page = await clientRef.current.getJobEvents(
            jobId,
            cursor,
            pageSize,
          );

          if (page.snapshotRequired) {
            const snapshot = await clientRef.current.getJobSnapshot(jobId);
            state = rebuildFromSnapshot(snapshot);
            cursor = state.cursor;
            // After snapshot rebuild, continue pulling from the new cursor
            // to catch any events appended after the snapshot was written.
            continue;
          }

          const result = applyEvents(state, page.events);
          state = result.state;
          cursor = result.state.cursor;
          finalCursor = cursor;

          if (result.needsPull) {
            // Gap detected: re-pull from the last applied sequence.
            cursor = result.gapCursor;
            continue;
          }

          if (page.hasMore) {
            continue;
          }

          // A later Wails notification may have arrived while this request
          // was pending. Do not settle until this replay reaches its reported
          // sequence; the next page is fetched from the cursor we just applied.
          if (cursor < (pendingTargetSequenceRef.current.get(jobId) ?? 0)) {
            continue;
          }

          break;
        }

        const wasTerminal = replayStatesRef.current[jobId]?.terminal ?? false;
        setReplayStates((current) => ({ ...current, [jobId]: state }));

        if (
          state.terminal &&
          !wasTerminal &&
          !terminalNotifiedRef.current.has(jobId)
        ) {
          terminalNotifiedRef.current.add(jobId);
          onJobTerminalRef.current?.(jobId, jobKind, state);
        } else if (!state.terminal) {
          // Reset the notification latch if the job somehow restarts.
          terminalNotifiedRef.current.delete(jobId);
        }
      } finally {
        pullingRef.current.delete(jobId);
        const target = pendingTargetSequenceRef.current.get(jobId) ?? 0;
        if (target <= finalCursor) {
          pendingTargetSequenceRef.current.delete(jobId);
        } else {
          // Cover the narrow window where a notification arrives after the
          // loop's target check but before the pulling guard is released.
          queueMicrotask(() => {
            void pullJobEvents(jobId, jobKind);
          });
        }
      }
    },
    [pageSize, maxIterations],
  );

  const resetJob = useCallback((jobId: string): void => {
    setReplayStates((current) => {
      if (!(jobId in current)) return current;
      const next = { ...current };
      delete next[jobId];
      return next;
    });
    terminalNotifiedRef.current.delete(jobId);
    pendingTargetSequenceRef.current.delete(jobId);
  }, []);

  // Subscribe to onJobUpdated once. The handler reads from refs so it does
  // not capture stale state.
  useEffect(() => {
    const unsubscribe = client.onJobUpdated(
      (jobId, jobKindStr, latestSequence) => {
        if (jobKindFilter && !jobKindFilter.has(jobKindStr as JobKind)) return;
        void pullJobEvents(jobId, jobKindStr as JobKind, latestSequence);
      },
    );
    return unsubscribe;
    // We intentionally only re-subscribe when the client identity changes.
    // pullJobEvents reads from refs internally.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client]);

  return { replayStates, pullJobEvents, resetJob };
}
