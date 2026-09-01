import { describe, it, expect, beforeEach } from "vitest";
import { createFactoryScene } from "./FactoryScene";
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
});
