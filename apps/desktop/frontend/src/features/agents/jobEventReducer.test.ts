import { describe, expect, it } from "vitest";
import {
  applyEvents,
  initialState,
  rebuildFromSnapshot,
  type JobEventView,
  type JobSnapshotView,
} from "./jobEventReducer";

function event(
  sequence: number,
  eventType: JobEventView["eventType"],
  stepId = "",
  payload = "",
): JobEventView {
  return {
    jobId: "run-1",
    jobKind: "agent",
    projectId: "project-1",
    sequenceNumber: sequence,
    eventType,
    stepId,
    payload,
    createdAt: "2026-08-07T00:00:00Z",
  };
}

const COMPLETED_PAYLOAD = JSON.stringify({
  role: "writer",
  input_tokens: 10,
  output_tokens: 5,
});

describe("applyEvents", () => {
  it("applies events in ascending sequence order", () => {
    const state = initialState();
    const events = [
      event(1, "job_started"),
      event(2, "step_started", "step-1"),
      event(3, "step_completed", "step-1", COMPLETED_PAYLOAD),
      event(4, "job_completed"),
    ];
    const result = applyEvents(state, events);

    expect(result.appliedCount).toBe(4);
    expect(result.state.cursor).toBe(4);
    expect(result.state.status).toBe("completed");
    expect(result.state.terminal).toBe(true);
    expect(result.state.inputTokens).toBe(10);
    expect(result.state.outputTokens).toBe(5);
    expect(result.state.steps).toEqual([
      { stepId: "step-1", status: "completed" },
    ]);
    expect(result.needsPull).toBe(false);
  });

  it("skips already-applied events idempotently (duplicate sequence)", () => {
    const state = applyEvents(initialState(), [
      event(1, "job_started"),
      event(2, "step_started", "step-1"),
    ]).state;

    // Re-send event 2 plus new events 3-4. Event 2 must be skipped.
    const result = applyEvents(state, [
      event(2, "step_started", "step-1"),
      event(3, "step_completed", "step-1", COMPLETED_PAYLOAD),
      event(4, "job_completed"),
    ]);

    expect(result.appliedCount).toBe(2);
    expect(result.state.cursor).toBe(4);
    // step-1 should appear only once, not duplicated.
    expect(result.state.steps).toHaveLength(1);
    expect(result.state.steps[0].status).toBe("completed");
    // Tokens from the duplicate event 2 must NOT be double-counted.
    expect(result.state.inputTokens).toBe(10);
    expect(result.state.outputTokens).toBe(5);
  });

  it("detects a gap and stops applying (needsPull=true)", () => {
    const state = applyEvents(initialState(), [
      event(1, "job_started"),
      event(2, "step_started", "step-1"),
    ]).state;

    // Cursor is 2; next expected is 3, but we receive 4 → gap.
    const result = applyEvents(state, [
      event(4, "step_completed", "step-1"),
    ]);

    expect(result.appliedCount).toBe(0);
    expect(result.needsPull).toBe(true);
    expect(result.gapCursor).toBe(2);
    expect(result.state.cursor).toBe(2);
  });

  it("returns done=true when no events to apply and no gap", () => {
    const state = applyEvents(initialState(), [
      event(1, "job_started"),
      event(2, "job_completed"),
    ]).state;

    const result = applyEvents(state, []);
    expect(result.done).toBe(true);
    expect(result.appliedCount).toBe(0);
    expect(result.needsPull).toBe(false);
  });

  it("accumulates tokens only from step_completed payloads", () => {
    const events = [
      event(1, "job_started"),
      event(2, "step_started", "step-1"),
      event(3, "step_completed", "step-1", COMPLETED_PAYLOAD),
      event(4, "step_started", "step-2"),
      event(5, "step_completed", "step-2", JSON.stringify({
        role: "planner",
        input_tokens: 20,
        output_tokens: 10,
      })),
      event(6, "job_completed"),
    ];
    const result = applyEvents(initialState(), events);

    expect(result.state.inputTokens).toBe(30);
    expect(result.state.outputTokens).toBe(15);
  });

  it("handles step_failed and job_failed terminal events", () => {
    const events = [
      event(1, "job_started"),
      event(2, "step_started", "step-1"),
      event(3, "step_failed", "step-1"),
      event(4, "job_failed"),
    ];
    const result = applyEvents(initialState(), events);

    expect(result.state.status).toBe("failed");
    expect(result.state.terminal).toBe(true);
    expect(result.state.steps[0].status).toBe("failed");
  });

  it("handles job_cancelled terminal event", () => {
    const events = [
      event(1, "job_started"),
      event(2, "job_cancelled"),
    ];
    const result = applyEvents(initialState(), events);

    expect(result.state.status).toBe("cancelled");
    expect(result.state.terminal).toBe(true);
  });

  it("handles job_recovered event", () => {
    const events = [
      event(1, "job_started"),
      event(2, "job_recovered"),
    ];
    const result = applyEvents(initialState(), events);

    expect(result.state.status).toBe("recoverable");
    expect(result.state.terminal).toBe(false);
  });
});

describe("rebuildFromSnapshot", () => {
  it("resets cursor to maxSequence and copies authoritative fields", () => {
    const snapshot: JobSnapshotView = {
      jobId: "run-1",
      jobKind: "agent",
      projectId: "project-1",
      status: "completed",
      maxSequence: 8,
      earliestReplayableSequence: 5,
      inputTokens: 30,
      outputTokens: 15,
      updatedAt: "2026-08-07T00:00:00Z",
    };
    const state = rebuildFromSnapshot(snapshot);

    expect(state.cursor).toBe(8);
    expect(state.status).toBe("completed");
    expect(state.inputTokens).toBe(30);
    expect(state.outputTokens).toBe(15);
    expect(state.terminal).toBe(true);
  });

  it("marks non-terminal statuses as terminal=false", () => {
    const snapshot: JobSnapshotView = {
      jobId: "run-1",
      jobKind: "extraction",
      projectId: "project-1",
      status: "running",
      maxSequence: 3,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: "2026-08-07T00:00:00Z",
    };
    const state = rebuildFromSnapshot(snapshot);

    expect(state.terminal).toBe(false);
    expect(state.status).toBe("running");
  });
});
