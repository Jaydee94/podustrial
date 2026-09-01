import { connectFactorySocket } from "./net/socket";
import { createFactoryScene } from "./scene/FactoryScene";

const wsUrl = `ws://${window.location.host}/ws`;
const scene = createFactoryScene(document.getElementById("app")!);
connectFactorySocket(wsUrl, (event) => scene.applyEvent(event));
