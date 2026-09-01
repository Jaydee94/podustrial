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

class PhaserFactoryScene extends Phaser.Scene implements FactoryScene {
  private machines = new Map<string, Machine>();
  private sprites = new Map<string, Phaser.GameObjects.Rectangle>();

  constructor() {
    super("factory");
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
      return; // headless test environment without a running Phaser game loop
    }
    const index = this.sprites.size;
    let rect = this.sprites.get(machine.id);
    if (!rect) {
      rect = this.add.rectangle(60 + index * 70, 60, 50, 50, STATUS_COLOR[machine.status]);
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
