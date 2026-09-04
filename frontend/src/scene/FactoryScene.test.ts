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

  it("destroy() removes the rendered scene from the container without throwing", () => {
    const scene = createFactoryScene(container);
    scene.applyEvent({ type: "machine_added", machine: { id: "m1", status: "running" } });

    expect(() => scene.destroy()).not.toThrow();
    expect(container.children).toHaveLength(0);
  });

  it("renders one machine card per machine, colored by status", () => {
    const scene = createFactoryScene(container);
    scene.applyEvent({ type: "machine_added", machine: { id: "m1", status: "pending" } });
    scene.applyEvent({ type: "machine_added", machine: { id: "m2", status: "running" } });
    scene.applyEvent({ type: "machine_added", machine: { id: "m3", status: "failed" } });

    const cards = container.querySelectorAll<HTMLElement>("[data-machine-id]");
    expect(cards).toHaveLength(3);

    const byId = (id: string) =>
      container.querySelector<HTMLElement>(`[data-machine-id="${id}"]`)!;

    expect(byId("m1").dataset.status).toBe("pending");
    expect(byId("m2").dataset.status).toBe("running");
    expect(byId("m3").dataset.status).toBe("failed");
  });

  it("updates a machine card in place on machine_updated instead of duplicating it", () => {
    const scene = createFactoryScene(container);
    scene.applyEvent({ type: "machine_added", machine: { id: "m1", status: "pending" } });
    scene.applyEvent({ type: "machine_updated", machine: { id: "m1", status: "running" } });

    expect(container.querySelectorAll("[data-machine-id]")).toHaveLength(1);
    expect(container.querySelector<HTMLElement>('[data-machine-id="m1"]')!.dataset.status).toBe(
      "running"
    );
  });

  it("removes a machine's card from the DOM on machine_removed", () => {
    const scene = createFactoryScene(container);
    scene.applyEvent({ type: "machine_added", machine: { id: "m1", status: "running" } });
    scene.applyEvent({ type: "machine_removed", machine: { id: "m1", status: "running" } });

    expect(container.querySelectorAll("[data-machine-id]")).toHaveLength(0);
  });

  it("never reuses or reorders a machine's card position, even after other ids are removed and re-added", () => {
    const scene = createFactoryScene(container);
    scene.applyEvent({ type: "machine_added", machine: { id: "m1", status: "running" } });
    scene.applyEvent({ type: "machine_added", machine: { id: "m2", status: "running" } });
    scene.applyEvent({ type: "machine_removed", machine: { id: "m1", status: "running" } });
    scene.applyEvent({ type: "machine_added", machine: { id: "m3", status: "pending" } });

    const ids = Array.from(container.querySelectorAll<HTMLElement>("[data-machine-id]")).map(
      (el) => el.dataset.machineId
    );
    expect(ids).toEqual(["m2", "m3"]);
  });

  it("reflects cluster_status ok/stromausfall in the power indicator", () => {
    const scene = createFactoryScene(container);
    const dot = () => container.querySelector<HTMLElement>("[data-role='power-dot']")!;
    const label = () => container.querySelector<HTMLElement>("[data-role='power-label']")!;

    expect(label().textContent).toContain("läuft");

    scene.applyEvent({ type: "cluster_status", clusterStatus: "stromausfall" });
    expect(label().textContent).toContain("Störung");
    expect(dot().style.background).not.toBe("");

    scene.applyEvent({ type: "cluster_status", clusterStatus: "ok" });
    expect(label().textContent).not.toContain("Störung");
  });

  it("shows the current machine count", () => {
    const scene = createFactoryScene(container);
    const count = () => container.querySelector<HTMLElement>("[data-role='count']")!.textContent;

    scene.applyEvent({ type: "machine_added", machine: { id: "m1", status: "running" } });
    scene.applyEvent({ type: "machine_added", machine: { id: "m2", status: "running" } });

    expect(count()).toContain("2");
  });
});
