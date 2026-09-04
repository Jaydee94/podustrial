import type { ClusterStatus, FactoryEvent, Machine, MachineStatus } from "../net/socket";

export interface FactoryScene {
  applyEvent(event: FactoryEvent): void;
  getMachineCount(): number;
  getMachineStatus(id: string): MachineStatus | undefined;
  /** Tears down the rendered scene and removes it from its container. Safe to call once. */
  destroy(): void;
}

const STATUS_LABEL: Record<MachineStatus, string> = {
  pending: "läuft an",
  running: "läuft",
  failed: "ausgefallen",
};

const STATUS_COLOR: Record<MachineStatus, string> = {
  pending: "#B58A2B",
  running: "#3F6478",
  failed: "#B5451C",
};

const STATUS_BORDER: Record<MachineStatus, string> = {
  pending: "#E0D3B0",
  running: "#C6D2D9",
  failed: "#E8C4B4",
};

function buildLogoIcon(sizePx: number, cellBg: string, accentBg: string, frameBg: string): HTMLElement {
  const icon = document.createElement("div");
  icon.className = "werkbank-logo-icon";
  icon.style.width = `${sizePx}px`;
  icon.style.height = `${sizePx}px`;
  icon.style.background = frameBg;
  // Anti-diagonal (top-right to bottom-left) picked out in the accent color —
  // the same 3x3 pattern used across every logo variant in the source design.
  const accentCells = new Set([2, 4, 6]);
  for (let i = 0; i < 9; i++) {
    const cell = document.createElement("div");
    cell.style.background = accentCells.has(i) ? accentBg : cellBg;
    icon.appendChild(cell);
  }
  return icon;
}

function buildShell(container: HTMLElement): {
  root: HTMLElement;
  grid: HTMLElement;
  countEl: HTMLElement;
  powerDot: HTMLElement;
  powerLabel: HTMLElement;
} {
  const root = document.createElement("div");
  root.className = "werkbank";

  const topbar = document.createElement("div");
  topbar.className = "werkbank-topbar";

  const brand = document.createElement("div");
  brand.className = "werkbank-brand";
  brand.appendChild(buildLogoIcon(28, "#6B7C85", "#B5451C", "#21201C"));
  const wordmark = document.createElement("span");
  wordmark.className = "werkbank-wordmark";
  wordmark.textContent = "podustrial";
  brand.appendChild(wordmark);
  topbar.appendChild(brand);

  const power = document.createElement("div");
  power.className = "werkbank-power";
  const powerDot = document.createElement("span");
  powerDot.className = "werkbank-power-dot";
  powerDot.dataset.role = "power-dot";
  const powerLabel = document.createElement("span");
  powerLabel.dataset.role = "power-label";
  power.append(powerDot, powerLabel);
  topbar.appendChild(power);

  root.appendChild(topbar);

  const panel = document.createElement("section");
  panel.className = "werkbank-panel";

  const panelHead = document.createElement("div");
  panelHead.className = "werkbank-panel-head";
  const heading = document.createElement("h2");
  heading.textContent = "Werk";
  const countEl = document.createElement("span");
  countEl.className = "werkbank-count";
  countEl.dataset.role = "count";
  panelHead.append(heading, countEl);
  panel.appendChild(panelHead);

  const grid = document.createElement("div");
  grid.className = "werkbank-grid";
  panel.appendChild(grid);

  root.appendChild(panel);
  container.appendChild(root);

  return { root, grid, countEl, powerDot, powerLabel };
}

function buildMachineCard(id: string): { card: HTMLElement; chip: HTMLElement; statusLine: HTMLElement } {
  const card = document.createElement("div");
  card.className = "werkbank-machine";
  card.dataset.machineId = id;

  const chip = document.createElement("div");
  chip.className = "werkbank-machine-chip";

  const label = document.createElement("div");
  label.className = "werkbank-machine-label";
  label.textContent = id;

  const statusLine = document.createElement("div");
  statusLine.className = "werkbank-machine-status";

  card.append(chip, label, statusLine);
  return { card, chip, statusLine };
}

class WerkbankScene implements FactoryScene {
  private machines = new Map<string, Machine>();
  private cards = new Map<string, { card: HTMLElement; chip: HTMLElement; statusLine: HTMLElement }>();
  private root: HTMLElement;
  private grid: HTMLElement;
  private countEl: HTMLElement;
  private powerDot: HTMLElement;
  private powerLabel: HTMLElement;

  constructor(container: HTMLElement) {
    const shell = buildShell(container);
    this.root = shell.root;
    this.grid = shell.grid;
    this.countEl = shell.countEl;
    this.powerDot = shell.powerDot;
    this.powerLabel = shell.powerLabel;
    this.setClusterStatus("ok");
    this.updateCount();
  }

  applyEvent(event: FactoryEvent): void {
    switch (event.type) {
      case "machine_added":
      case "machine_updated":
        if (event.machine) {
          this.machines.set(event.machine.id, event.machine);
          this.renderMachine(event.machine);
          this.updateCount();
        }
        break;
      case "machine_removed":
        if (event.machine) {
          this.machines.delete(event.machine.id);
          this.cards.get(event.machine.id)?.card.remove();
          this.cards.delete(event.machine.id);
          this.updateCount();
        }
        break;
      case "cluster_status":
        this.setClusterStatus(event.clusterStatus ?? "ok");
        break;
    }
  }

  getMachineCount(): number {
    return this.machines.size;
  }

  getMachineStatus(id: string): MachineStatus | undefined {
    return this.machines.get(id)?.status;
  }

  destroy(): void {
    this.root.remove();
  }

  private renderMachine(machine: Machine): void {
    let entry = this.cards.get(machine.id);
    if (!entry) {
      entry = buildMachineCard(machine.id);
      this.cards.set(machine.id, entry);
      this.grid.appendChild(entry.card);
    }
    entry.card.dataset.status = machine.status;
    entry.chip.style.background = STATUS_COLOR[machine.status];
    entry.chip.style.animation = machine.status === "pending" ? "werkbankPodPulse 1.1s ease-in-out infinite" : "none";
    entry.card.style.borderColor = STATUS_BORDER[machine.status];
    entry.statusLine.textContent = STATUS_LABEL[machine.status];
    entry.statusLine.style.color = STATUS_COLOR[machine.status];
  }

  private updateCount(): void {
    const n = this.machines.size;
    this.countEl.textContent = `${n} Maschine${n === 1 ? "" : "n"}`;
  }

  private setClusterStatus(status: ClusterStatus): void {
    const failure = status === "stromausfall";
    this.powerDot.style.background = failure ? "#B5451C" : "#3F6478";
    this.powerLabel.textContent = failure ? "Werk: Störung" : "Werk: läuft";
  }
}

export function createFactoryScene(container: HTMLElement): FactoryScene {
  return new WerkbankScene(container);
}
