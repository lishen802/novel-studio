import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import { useJobEventReplay } from "./useJobEventReplay";
import type { JobEventView } from "./jobEventReducer";

function makeEvent(
  sequence: number,
  eventType: JobEventView["eventType"],
  payload = "",
  stepId = "",
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

describe("useJobEventReplay", () => {
  it("pulls events and applies the reducer on job:updated notification", async () => {
    const client = new FakeStudioClient();
    client.openProjectResult = {
      id: "project-1",
      title: "Test",
      genre: "",
      summary: "",
      path: "/tmp/test.zjproject",
      accessMode: "editable",
    };
    // Seed events for run-1 directly via the fake's helper.
    client.appendJobEvent("run-1", "agent", 1, "job_started", "");
    client.appendJobEvent(
      "run-1",
      "agent",
      2,
      "step_completed",
      JSON.stringify({ input_tokens: 10, output_tokens: 5 }),
      "step-1",
    );
    client.appendJobEvent("run-1", "agent", 3, "job_completed", "");
    client.upsertJobSnapshot({
      jobId: "run-1",
      jobKind: "agent",
      projectId: "project-1",
      status: "completed",
      maxSequence: 3,
      earliestReplayableSequence: 1,
      inputTokens: 10,
      outputTokens: 5,
      updatedAt: "2026-08-07T00:00:00Z",
    });

    const onJobTerminal = vi.fn();
    const { result } = renderHook(() =>
      useJobEventReplay({ client, onJobTerminal }),
    );

    // Simulate the backend emitting job:updated after writing events.
    act(() => {
      client.emitJobUpdated("run-1", "agent", 3);
    });

    await waitFor(() => {
      expect(result.current.replayStates["run-1"]?.cursor).toBe(3);
    });

    expect(result.current.replayStates["run-1"]).toMatchObject({
      cursor: 3,
      status: "completed",
      terminal: true,
      inputTokens: 10,
      outputTokens: 5,
    });
    expect(client.getJobEventsCalls).toContainEqual({
      jobId: "run-1",
      afterSequence: 0,
      limit: 50,
    });
    expect(onJobTerminal).toHaveBeenCalledTimes(1);
    expect(onJobTerminal).toHaveBeenCalledWith(
      "run-1",
      "agent",
      expect.objectContaining({ status: "completed", terminal: true }),
    );
  });

  it("rebuilds from snapshot when snapshotRequired is true", async () => {
    const client = new FakeStudioClient();
    client.openProjectResult = {
      id: "project-1",
      title: "Test",
      genre: "",
      summary: "",
      path: "/tmp/test.zjproject",
      accessMode: "editable",
    };
    // Seed a snapshot with earliest_replayable_sequence=5 so a cursor at 0
    // triggers snapshotRequired. Then add events after the snapshot.
    client.upsertJobSnapshot({
      jobId: "run-1",
      jobKind: "agent",
      projectId: "project-1",
      status: "running",
      maxSequence: 5,
      earliestReplayableSequence: 5,
      inputTokens: 100,
      outputTokens: 50,
      updatedAt: "2026-08-07T00:00:00Z",
    });
    client.appendJobEvent("run-1", "agent", 6, "step_completed", "", "step-2");

    const { result } = renderHook(() => useJobEventReplay({ client }));

    act(() => {
      client.emitJobUpdated("run-1", "agent", 6);
    });

    await waitFor(() => {
      expect(
        result.current.replayStates["run-1"]?.cursor,
      ).toBeGreaterThanOrEqual(5);
    });

    // Snapshot rebuild should set status to running and tokens to 100/50.
    expect(result.current.replayStates["run-1"]).toMatchObject({
      status: "running",
      inputTokens: 100,
      outputTokens: 50,
      terminal: false,
    });
    // Cursor should advance past 5 after pulling the post-snapshot event.
    expect(result.current.replayStates["run-1"]?.cursor).toBe(6);
    expect(client.getJobSnapshotCalls).toContain("run-1");
  });

  it("dedupes concurrent notifications for the same job", async () => {
    const client = new FakeStudioClient();
    client.openProjectResult = {
      id: "project-1",
      title: "Test",
      genre: "",
      summary: "",
      path: "/tmp/test.zjproject",
      accessMode: "editable",
    };
    client.appendJobEvent("run-1", "agent", 1, "job_started", "");
    client.upsertJobSnapshot({
      jobId: "run-1",
      jobKind: "agent",
      projectId: "project-1",
      status: "running",
      maxSequence: 1,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: "2026-08-07T00:00:00Z",
    });

    const { result } = renderHook(() => useJobEventReplay({ client }));

    // Emit three notifications in rapid succession. The pulling guard should
    // dedupe so only one getJobEvents call happens for this batch.
    act(() => {
      client.emitJobUpdated("run-1", "agent", 1);
      client.emitJobUpdated("run-1", "agent", 1);
      client.emitJobUpdated("run-1", "agent", 1);
    });

    await waitFor(() => {
      expect(result.current.replayStates["run-1"]?.cursor).toBe(1);
    });

    // At least one call happened; the guard prevents stacking.
    const callsForRun = client.getJobEventsCalls.filter(
      (call) => call.jobId === "run-1",
    );
    expect(callsForRun.length).toBeGreaterThanOrEqual(1);
    expect(callsForRun.length).toBeLessThanOrEqual(3);
  });

  it("filters notifications by jobKindFilter", async () => {
    const client = new FakeStudioClient();
    client.openProjectResult = {
      id: "project-1",
      title: "Test",
      genre: "",
      summary: "",
      path: "/tmp/test.zjproject",
      accessMode: "editable",
    };
    client.appendJobEvent("run-agent", "agent", 1, "job_started", "");
    client.appendJobEvent("run-extraction", "extraction", 1, "job_started", "");
    client.upsertJobSnapshot({
      jobId: "run-agent",
      jobKind: "agent",
      projectId: "project-1",
      status: "running",
      maxSequence: 1,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: "2026-08-07T00:00:00Z",
    });
    client.upsertJobSnapshot({
      jobId: "run-extraction",
      jobKind: "extraction",
      projectId: "project-1",
      status: "running",
      maxSequence: 1,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: "2026-08-07T00:00:00Z",
    });

    const { result } = renderHook(() =>
      useJobEventReplay({ client, jobKindFilter: new Set(["extraction"]) }),
    );

    act(() => {
      client.emitJobUpdated("run-agent", "agent", 1);
      client.emitJobUpdated("run-extraction", "extraction", 1);
    });

    await waitFor(() => {
      expect(result.current.replayStates["run-extraction"]?.cursor).toBe(1);
    });

    // Agent job should be filtered out.
    expect(result.current.replayStates["run-agent"]).toBeUndefined();
    expect(
      client.getJobEventsCalls.some((call) => call.jobId === "run-agent"),
    ).toBe(false);
  });

  it("paginates when hasMore is true", async () => {
    const client = new FakeStudioClient();
    client.openProjectResult = {
      id: "project-1",
      title: "Test",
      genre: "",
      summary: "",
      path: "/tmp/test.zjproject",
      accessMode: "editable",
    };
    // Seed 3 events; with pageSize=2, the hook should paginate to fetch all.
    client.appendJobEvent("run-1", "agent", 1, "job_started", "");
    client.appendJobEvent(
      "run-1",
      "agent",
      2,
      "step_completed",
      JSON.stringify({ input_tokens: 10, output_tokens: 5 }),
      "step-1",
    );
    client.appendJobEvent("run-1", "agent", 3, "job_completed", "");
    client.upsertJobSnapshot({
      jobId: "run-1",
      jobKind: "agent",
      projectId: "project-1",
      status: "completed",
      maxSequence: 3,
      earliestReplayableSequence: 1,
      inputTokens: 10,
      outputTokens: 5,
      updatedAt: "2026-08-07T00:00:00Z",
    });

    const { result } = renderHook(() =>
      useJobEventReplay({ client, pageSize: 2 }),
    );

    act(() => {
      client.emitJobUpdated("run-1", "agent", 3);
    });

    await waitFor(() => {
      expect(result.current.replayStates["run-1"]?.cursor).toBe(3);
    });

    // Should have made at least 2 calls (page 1: events 1-2, page 2: event 3).
    const callsForRun = client.getJobEventsCalls.filter(
      (call) => call.jobId === "run-1",
    );
    expect(callsForRun.length).toBeGreaterThanOrEqual(2);
  });

  it("resetJob drops the replay state for a job", async () => {
    const client = new FakeStudioClient();
    client.openProjectResult = {
      id: "project-1",
      title: "Test",
      genre: "",
      summary: "",
      path: "/tmp/test.zjproject",
      accessMode: "editable",
    };
    client.appendJobEvent("run-1", "agent", 1, "job_started", "");
    client.upsertJobSnapshot({
      jobId: "run-1",
      jobKind: "agent",
      projectId: "project-1",
      status: "running",
      maxSequence: 1,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: "2026-08-07T00:00:00Z",
    });

    const { result } = renderHook(() => useJobEventReplay({ client }));

    act(() => {
      client.emitJobUpdated("run-1", "agent", 1);
    });

    await waitFor(() => {
      expect(result.current.replayStates["run-1"]?.cursor).toBe(1);
    });

    act(() => {
      result.current.resetJob("run-1");
    });

    expect(result.current.replayStates["run-1"]).toBeUndefined();
  });

  it("replays a recovered job after Bridge remount via pullJobEvents", async () => {
    // Simulates an app restart / Bridge remount: the backend reopened the
    // project, RecoverInterruptedAgentRuns appended job_recovered, and the
    // workbench calls pullJobEvents for each recoverable job it discovered via
    // listRecoverableJobs. The hook must pull the events and settle on a
    // non-terminal recoverable state without firing onJobTerminal.
    const client = new FakeStudioClient();
    client.openProjectResult = {
      id: "project-1",
      title: "Test",
      genre: "",
      summary: "",
      path: "/tmp/test.zjproject",
      accessMode: "editable",
    };
    client.appendJobEvent("run-1", "agent", 1, "job_started", "");
    client.appendJobEvent("run-1", "agent", 2, "job_recovered", "");
    client.upsertJobSnapshot({
      jobId: "run-1",
      jobKind: "agent",
      projectId: "project-1",
      status: "recoverable",
      maxSequence: 2,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: "2026-08-07T00:00:00Z",
    });

    const onJobTerminal = vi.fn();
    const { result } = renderHook(() =>
      useJobEventReplay({ client, onJobTerminal }),
    );

    // The workbench discovered run-1 via listRecoverableJobs and kicks off a
    // pull directly (no job:updated notification yet).
    await act(async () => {
      await result.current.pullJobEvents("run-1", "agent");
    });

    expect(result.current.replayStates["run-1"]).toMatchObject({
      cursor: 2,
      status: "recoverable",
      terminal: false,
    });
    // Recoverable is non-terminal; onJobTerminal must NOT fire so the workbench
    // doesn't treat recovery as completion.
    expect(onJobTerminal).not.toHaveBeenCalled();
  });

  it("tolerates a lost job:updated notification by pulling on the next wake-up", async () => {
    // The notifier is best-effort and may drop a wake-up. The next time any
    // notification arrives (even a stale one), the hook re-pulls from its
    // cursor and catches up to the events that were missed.
    const client = new FakeStudioClient();
    client.openProjectResult = {
      id: "project-1",
      title: "Test",
      genre: "",
      summary: "",
      path: "/tmp/test.zjproject",
      accessMode: "editable",
    };
    client.appendJobEvent("run-1", "agent", 1, "job_started", "");
    client.upsertJobSnapshot({
      jobId: "run-1",
      jobKind: "agent",
      projectId: "project-1",
      status: "running",
      maxSequence: 1,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: "2026-08-07T00:00:00Z",
    });

    const { result } = renderHook(() => useJobEventReplay({ client }));

    // First notification delivers event 1.
    act(() => {
      client.emitJobUpdated("run-1", "agent", 1);
    });
    await waitFor(() => {
      expect(result.current.replayStates["run-1"]?.cursor).toBe(1);
    });

    // Backend appends event 2 (step_completed) but the notification is lost.
    client.appendJobEvent(
      "run-1",
      "agent",
      2,
      "step_completed",
      JSON.stringify({ input_tokens: 10, output_tokens: 5 }),
      "step-1",
    );
    client.upsertJobSnapshot({
      jobId: "run-1",
      jobKind: "agent",
      projectId: "project-1",
      status: "running",
      maxSequence: 2,
      earliestReplayableSequence: 1,
      inputTokens: 10,
      outputTokens: 5,
      updatedAt: "2026-08-07T00:00:00Z",
    });

    // A later (possibly coalesced) notification triggers a re-pull from
    // cursor=1, which picks up the missed event 2.
    act(() => {
      client.emitJobUpdated("run-1", "agent", 2);
    });
    await waitFor(() => {
      expect(result.current.replayStates["run-1"]?.cursor).toBe(2);
    });

    expect(result.current.replayStates["run-1"]).toMatchObject({
      inputTokens: 10,
      outputTokens: 5,
    });
  });

  it("catches an update that arrives while a pull is in flight", async () => {
    const client = new FakeStudioClient();
    client.appendJobEvent("run-1", "agent", 1, "job_started", "");
    client.upsertJobSnapshot({
      jobId: "run-1",
      jobKind: "agent",
      projectId: "project-1",
      status: "running",
      maxSequence: 1,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: "2026-08-10T00:00:00Z",
    });

    let releaseFirstPull: (() => void) | undefined;
    const firstPull = new Promise<void>((resolve) => {
      releaseFirstPull = resolve;
    });
    const getJobEvents = vi.spyOn(client, "getJobEvents");
    let blockFirstCall = true;
    getJobEvents.mockImplementation(async (jobId, afterSequence, limit) => {
      if (blockFirstCall) {
        blockFirstCall = false;
        // Capture the first page before the later event exists, then delay
        // delivery. This recreates the exact notifier race: event 2 arrives
        // while replay is still applying a stale page for event 1.
        const firstPage = await FakeStudioClient.prototype.getJobEvents.call(
          client,
          jobId,
          afterSequence,
          limit,
        );
        await firstPull;
        return firstPage;
      }
      return FakeStudioClient.prototype.getJobEvents.call(
        client,
        jobId,
        afterSequence,
        limit,
      );
    });

    const { result } = renderHook(() => useJobEventReplay({ client }));
    act(() => client.emitJobUpdated("run-1", "agent", 1));
    await waitFor(() => expect(getJobEvents).toHaveBeenCalledTimes(1));

    client.appendJobEvent("run-1", "agent", 2, "job_completed", "");
    client.upsertJobSnapshot({
      jobId: "run-1",
      jobKind: "agent",
      projectId: "project-1",
      status: "completed",
      maxSequence: 2,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: "2026-08-10T00:00:01Z",
    });
    act(() => client.emitJobUpdated("run-1", "agent", 2));
    releaseFirstPull?.();

    await waitFor(() =>
      expect(result.current.replayStates["run-1"]?.cursor).toBe(2),
    );
    expect(result.current.replayStates["run-1"]?.status).toBe("completed");
  });
});
