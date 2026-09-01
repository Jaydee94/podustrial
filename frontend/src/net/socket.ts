export type MachineStatus = "pending" | "running" | "failed";

export interface Machine {
  id: string;
  status: MachineStatus;
}

export type ClusterStatus = "ok" | "stromausfall";

export type EventType =
  | "machine_added"
  | "machine_updated"
  | "machine_removed"
  | "cluster_status";

export interface FactoryEvent {
  type: EventType;
  machine?: Machine;
  clusterStatus?: ClusterStatus;
}

export function connectFactorySocket(
  url: string,
  onEvent: (event: FactoryEvent) => void
): WebSocket {
  const socket = new WebSocket(url);
  socket.onmessage = (raw) => {
    try {
      const event = JSON.parse(raw.data) as FactoryEvent;
      onEvent(event);
    } catch {
      // ignore malformed messages
    }
  };
  return socket;
}
