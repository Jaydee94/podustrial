// Phaser's ESM build (dist/phaser.esm.js — see the "phaser" alias in
// vite.config.ts) has no default export, only named ones; a namespace
// import is what actually resolves against it.
import * as Phaser from "phaser";
import type { FactoryEvent, Machine, MachineStatus } from "../net/socket";

export interface FactoryScene {
  applyEvent(event: FactoryEvent): void;
  getMachineCount(): number;
  getMachineStatus(id: string): MachineStatus | undefined;
}

const STATUS_COLOR: Record<MachineStatus, number> = {
  pending: 0x999999,
  running: 0x4caf50,
  failed: 0xe53935,
};

const SLOT_START_X = 60;
const SLOT_Y = 60;
const SLOT_SPACING = 70;

/**
 * Assigns each machine id a permanent, unique horizontal slot the first time
 * it's seen. Slots are never reused across different ids — even once a
 * machine is removed — so rectangles positioned by slot index can never end
 * up overlapping after add/remove/add cycles. An id that reappears reclaims
 * its original slot instead of jumping to a new one.
 */
export class SlotAllocator {
  private slots = new Map<string, number>();
  private next = 0;

  slotFor(id: string): number {
    let slot = this.slots.get(id);
    if (slot === undefined) {
      slot = this.next++;
      this.slots.set(id, slot);
    }
    return slot;
  }
}

class PhaserFactoryScene extends Phaser.Scene implements FactoryScene {
  private machines = new Map<string, Machine>();
  private sprites = new Map<string, Phaser.GameObjects.Rectangle>();
  private slots = new SlotAllocator();

  constructor() {
    super("factory");
  }

  // Phaser boots asynchronously, so applyEvent() can run — and in practice
  // does run — before `this.add` exists (see renderMachine's guard below).
  // create() fires once the scene is actually ready, and replays every
  // machine received so far so none of them are silently left unrendered.
  create(): void {
    for (const machine of this.machines.values()) {
      this.renderMachine(machine);
    }
  }

  applyEvent(event: FactoryEvent): void {
    if (!event.machine) {
      return;
    }
    const machine = event.machine;
    switch (event.type) {
      case "machine_added":
      case "machine_updated":
        this.machines.set(machine.id, machine);
        this.renderMachine(machine);
        break;
      case "machine_removed":
        this.machines.delete(machine.id);
        this.sprites.get(machine.id)?.destroy();
        this.sprites.delete(machine.id);
        break;
    }
  }

  getMachineCount(): number {
    return this.machines.size;
  }

  getMachineStatus(id: string): MachineStatus | undefined {
    return this.machines.get(id)?.status;
  }

  private renderMachine(machine: Machine): void {
    if (!this.add) {
      return; // headless test environment / scene not started yet — see create()
    }
    let rect = this.sprites.get(machine.id);
    if (!rect) {
      const slot = this.slots.slotFor(machine.id);
      const x = SLOT_START_X + slot * SLOT_SPACING;
      rect = this.add.rectangle(x, SLOT_Y, 50, 50, STATUS_COLOR[machine.status]);
      this.sprites.set(machine.id, rect);
    } else {
      rect.setFillStyle(STATUS_COLOR[machine.status]);
    }
  }
}

export function createFactoryScene(container: HTMLElement): FactoryScene {
  const scene = new PhaserFactoryScene();
  new Phaser.Game({
    type: Phaser.AUTO,
    width: 800,
    height: 400,
    parent: container,
    scene,
  });
  return scene;
}
