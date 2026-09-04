import { connectFactorySocket } from "./net/socket";
import { createFactoryScene } from "./scene/FactoryScene";

const wsProtocol = window.location.protocol === "https:" ? "wss:" : "ws:";
const wsUrl = `${wsProtocol}//${window.location.host}/ws`;
const scene = createFactoryScene(document.getElementById("app")!);
connectFactorySocket(wsUrl, (event) => scene.applyEvent(event));
