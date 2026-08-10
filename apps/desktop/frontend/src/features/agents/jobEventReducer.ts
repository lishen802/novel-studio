/**
 * jobEventReducer provides the pure cursor-based replay logic for JobEvent
 * feeds. The frontend uses it to rebuild job state incrementally: each time a
 * "job:updated" notification arrives or the component re-mounts, the caller
 * pulls events via getJobEvents(jobID, cursor, limit) and applies them here.
 *
 * Invariants enforced by the reducer:
 * 1. Idempotent — events with sequence <= cursor are skipped, so duplicate
 *    notifications or re-sent pages never double-count tokens or steps.
 * 2. Gap detection — if an event's sequence > cursor + 1, the reducer stops
 *    and returns needsPull=true with gapCursor so the caller can re-pull from
 *    the gap.
 * 3. Authoritative accumulation — tokens only accumulate from step_completed
 *    event payloads (or from a snapshot rebuild), never from event arrival
 *    counts.
 */

export type JobEventType =
  | "job_queued"
  | "job_started"
  | "step_started"
  | "step_delta"
  | "step_completed"
  | "step_failed"
  | "job_completed"
  | "job_failed"
  | "job_cancelled"
  | "job_recovered";

export type JobKind = "agent" | "extraction";

export interface JobEventView {
  jobId: string;
  jobKind: JobKind;
  projectId: string;
  sequenceNumber: number;
  eventType: JobEventType;
  stepId?: string;
  payload?: string;
  createdAt: string;
}

export interface JobSnapshotView {
  jobId: string;
  jobKind: JobKind;
  projectId: string;
  status: string;
  maxSequence: number;
  earliestReplayableSequence: number;
  inputTokens: number;
  outputTokens: number;
  pendingAction?: string;
  terminalInfo?: string;
  steps?: JobStepState[];
  updatedAt: string;
}

export interface JobEventsPage {
  events: JobEventView[];
  nextCursor: number;
  hasMore: boolean;
  snapshotRequired: boolean;
}

export interface JobSummaryView {
  jobId: string;
  jobKind: JobKind;
  projectId: string;
  status: string;
  maxSequence: number;
  updatedAt: string;
}

export type StepStatus =
  "queued" | "running" | "started" | "completed" | "failed" | "cancelled";

export interface JobStepState {
  stepId: string;
  status: StepStatus;
}

export interface JobReplayState {
  /** The highest sequence number applied so far (0 = nothing applied). */
  cursor: number;
  status: string;
  inputTokens: number;
  outputTokens: number;
  steps: JobStepState[];
  terminal: boolean;
}

export interface ApplyEventsResult {
  state: JobReplayState;
  /** How many events were actually applied (duplicates excluded). */
  appliedCount: number;
  /** True when a gap was detected; the caller should re-pull from gapCursor. */
  needsPull: boolean;
  /** The cursor to re-pull from after a gap (the last applied sequence). */
  gapCursor: number;
  /** True when all events were applied and no more pages are needed. */
  done: boolean;
}

const TERMINAL_STATUSES = new Set(["completed", "failed", "cancelled"]);

/**
 * initialState returns a fresh replay state for a job that hasn't received any
 * events yet. The caller typically follows up with getJobEvents(jobID, 0, limit).
 */
export function initialState(): JobReplayState {
  return {
    cursor: 0,
    status: "queued",
    inputTokens: 0,
    outputTokens: 0,
    steps: [],
    terminal: false,
  };
}

/**
 * applyEvents applies a page of events to the replay state in ascending
 * sequence order. It is a pure function: it does not mutate the input state.
 *
 * - Events with sequence <= cursor are skipped (idempotent).
 * - If an event's sequence > cursor + 1, the reducer stops and returns
 *   needsPull=true so the caller can re-pull from the gap.
 * - step_completed payloads accumulate input/output tokens.
 */
export function applyEvents(
  state: JobReplayState,
  events: JobEventView[],
): ApplyEventsResult {
  let cursor = state.cursor;
  let status = state.status;
  let inputTokens = state.inputTokens;
  let outputTokens = state.outputTokens;
  let steps = state.steps;
  let terminal = state.terminal;
  let appliedCount = 0;
  let needsPull = false;
  let gapCursor = cursor;

  for (const evt of events) {
    // Idempotent: skip already-applied events.
    if (evt.sequenceNumber <= cursor) continue;

    // Gap detection: if the next event is beyond cursor + 1, stop and ask
    // the caller to re-pull from the gap.
    if (evt.sequenceNumber > cursor + 1) {
      needsPull = true;
      gapCursor = cursor;
      break;
    }

    cursor = evt.sequenceNumber;
    appliedCount++;

    switch (evt.eventType) {
      case "job_queued":
        status = "queued";
        break;
      case "job_started":
        status = "running";
        break;
      case "step_started":
        steps = [...steps, { stepId: evt.stepId ?? "", status: "started" }];
        break;
      case "step_completed":
        steps = steps.map((s) =>
          s.stepId === evt.stepId
            ? { ...s, status: "completed" as StepStatus }
            : s,
        );
        {
          const tokens = parseTokenPayload(evt.payload);
          inputTokens += tokens.inputTokens;
          outputTokens += tokens.outputTokens;
        }
        break;
      case "step_failed":
        steps = steps.map((s) =>
          s.stepId === evt.stepId
            ? { ...s, status: "failed" as StepStatus }
            : s,
        );
        break;
      case "job_completed":
        status = "completed";
        terminal = true;
        break;
      case "job_failed":
        status = "failed";
        terminal = true;
        break;
      case "job_cancelled":
        status = "cancelled";
        terminal = true;
        break;
      case "job_recovered":
        status = "recoverable";
        break;
      // step_delta is a lightweight progress pointer; no state change needed.
      case "step_delta":
        break;
    }
  }

  return {
    state: { cursor, status, inputTokens, outputTokens, steps, terminal },
    appliedCount,
    needsPull,
    gapCursor,
    done: !needsPull && appliedCount === 0,
  };
}

/**
 * rebuildFromSnapshot resets the replay state from an authoritative JobSnapshot.
 * The cursor is set to maxSequence so subsequent pulls start after the
 * snapshot. Token counts come from the snapshot, not from replayed events.
 */
export function rebuildFromSnapshot(snapshot: JobSnapshotView): JobReplayState {
  return {
    cursor: snapshot.maxSequence,
    status: snapshot.status,
    inputTokens: snapshot.inputTokens,
    outputTokens: snapshot.outputTokens,
    steps: snapshot.steps ?? [],
    terminal: TERMINAL_STATUSES.has(snapshot.status),
  };
}

interface TokenDelta {
  inputTokens: number;
  outputTokens: number;
}

function parseTokenPayload(payload: string | undefined): TokenDelta {
  if (!payload) return { inputTokens: 0, outputTokens: 0 };
  try {
    const parsed = JSON.parse(payload) as Record<string, unknown>;
    const inputTokens =
      typeof parsed.input_tokens === "number" ? parsed.input_tokens : 0;
    const outputTokens =
      typeof parsed.output_tokens === "number" ? parsed.output_tokens : 0;
    return { inputTokens, outputTokens };
  } catch {
    return { inputTokens: 0, outputTokens: 0 };
  }
}
