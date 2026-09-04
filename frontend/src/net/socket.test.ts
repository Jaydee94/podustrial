import { describe, it, expect } from "vitest";
import { connectFactorySocket, type FactoryEvent } from "./socket";

class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  onmessage: ((ev: { data: string }) => void) | null = null;
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  url: string;
  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }
  emit(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) });
  }
}

describe("connectFactorySocket", () => {
  it("forwards parsed FactoryEvents to the callback", () => {
    // @ts-expect-error test double replacing the global WebSocket
    globalThis.WebSocket = FakeWebSocket;
    const received: FactoryEvent[] = [];
    connectFactorySocket("ws://localhost:8080/ws", (ev) => received.push(ev));

    const socket = FakeWebSocket.instances.at(-1)!;
    socket.emit({ type: "machine_added", machine: { id: "m1", status: "running" } });

    expect(received).toHaveLength(1);
    expect(received[0]).toEqual({
      type: "machine_added",
      machine: { id: "m1", status: "running" },
    });
  });

  it("ignores messages it cannot parse as JSON", () => {
    // @ts-expect-error test double replacing the global WebSocket
    globalThis.WebSocket = FakeWebSocket;
    const received: FactoryEvent[] = [];
    connectFactorySocket("ws://localhost:8080/ws", (ev) => received.push(ev));

    const socket = FakeWebSocket.instances.at(-1)!;
    socket.onmessage?.({ data: "not json" });

    expect(received).toHaveLength(0);
  });
});
