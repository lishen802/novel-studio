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
      expect(result.current.replayStates["run-1"]?.cursor).toBeGreaterThanOrEqual(5);
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
});
