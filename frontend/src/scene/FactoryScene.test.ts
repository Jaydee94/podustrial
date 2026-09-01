import { describe, it, expect, beforeEach, vi } from "vitest";
import { createFactoryScene, SlotAllocator } from "./FactoryScene";
import type { FactoryEvent } from "../net/socket";

describe("FactoryScene", () => {
  let container: HTMLElement;

  beforeEach(() => {
    container = document.createElement("div");
  });

  it("adds a machine on machine_added", () => {
    const scene = createFactoryScene(container);
    const event: FactoryEvent = {
      type: "machine_added",
      machine: { id: "m1", status: "pending" },
    };
    scene.applyEvent(event);

    expect(scene.getMachineCount()).toBe(1);
    expect(scene.getMachineStatus("m1")).toBe("pending");
  });

  it("updates machine status on machine_updated", () => {
    const scene = createFactoryScene(container);
    scene.applyEvent({ type: "machine_added", machine: { id: "m1", status: "pending" } });
    scene.applyEvent({ type: "machine_updated", machine: { id: "m1", status: "running" } });

    expect(scene.getMachineCount()).toBe(1);
    expect(scene.getMachineStatus("m1")).toBe("running");
  });

  it("removes a machine on machine_removed", () => {
    const scene = createFactoryScene(container);
    scene.applyEvent({ type: "machine_added", machine: { id: "m1", status: "running" } });
    scene.applyEvent({ type: "machine_removed", machine: { id: "m1", status: "running" } });

    expect(scene.getMachineCount()).toBe(0);
    expect(scene.getMachineStatus("m1")).toBeUndefined();
  });

  it("ignores cluster_status events for machine count", () => {
    const scene = createFactoryScene(container);
    scene.applyEvent({ type: "cluster_status", clusterStatus: "stromausfall" });

    expect(scene.getMachineCount()).toBe(0);
  });

  it("replays every tracked machine through renderMachine once create() fires", () => {
    // Phaser's game boot is async — applyEvent() routinely runs before
    // this.add exists, so renderMachine() no-ops (see its guard). create()
    // is Phaser's normal "scene is ready" lifecycle hook; it must replay
    // whatever was already tracked or those machines are never drawn.
    const scene = createFactoryScene(container);
    const renderSpy = vi.spyOn(scene as unknown as { renderMachine: (m: unknown) => void }, "renderMachine");

    scene.applyEvent({ type: "machine_added", machine: { id: "m1", status: "pending" } });
    scene.applyEvent({ type: "machine_added", machine: { id: "m2", status: "running" } });
    renderSpy.mockClear(); // applyEvent() itself already called renderMachine once per event

    (scene as unknown as { create: () => void }).create();

    expect(renderSpy).toHaveBeenCalledTimes(2);
    expect(renderSpy).toHaveBeenCalledWith({ id: "m1", status: "pending" });
    expect(renderSpy).toHaveBeenCalledWith({ id: "m2", status: "running" });
  });

  it("does not replay a machine that was removed before create() fires", () => {
    const scene = createFactoryScene(container);
    const renderSpy = vi.spyOn(scene as unknown as { renderMachine: (m: unknown) => void }, "renderMachine");

    scene.applyEvent({ type: "machine_added", machine: { id: "m1", status: "pending" } });
    scene.applyEvent({ type: "machine_removed", machine: { id: "m1", status: "pending" } });
    renderSpy.mockClear();

    (scene as unknown as { create: () => void }).create();

    expect(renderSpy).not.toHaveBeenCalled();
  });
});

describe("SlotAllocator", () => {
  it("assigns increasing slots to new ids", () => {
    const slots = new SlotAllocator();

    expect(slots.slotFor("m1")).toBe(0);
    expect(slots.slotFor("m2")).toBe(1);
    expect(slots.slotFor("m3")).toBe(2);
  });

  it("returns the same slot for an id queried again", () => {
    const slots = new SlotAllocator();

    expect(slots.slotFor("m1")).toBe(0);
    slots.slotFor("m2");
    expect(slots.slotFor("m1")).toBe(0);
  });

  it("never reuses a slot for a different id, even after the original id is gone", () => {
    // Regression: renderMachine used to derive a new sprite's slot from
    // sprites.size, so removing a machine and then adding a new one could
    // hand out an already-occupied slot, overlapping an active rectangle.
    const slots = new SlotAllocator();

    slots.slotFor("m1"); // 0
    const m2Slot = slots.slotFor("m2"); // 1
    const m3Slot = slots.slotFor("m3"); // 2
    // m2 "removed" here — nothing to do on the allocator, it never forgets.
    const m4Slot = slots.slotFor("m4");

    expect(m4Slot).not.toBe(m2Slot);
    expect(m4Slot).not.toBe(m3Slot);
    expect(m4Slot).toBe(3);
  });

  it("gives a reappearing id back its original slot", () => {
    const slots = new SlotAllocator();

    const first = slots.slotFor("m1");
    slots.slotFor("m2");
    const again = slots.slotFor("m1");

    expect(again).toBe(first);
  });
});
