// jsdom does not implement a real <canvas> 2D rendering context (see
// https://github.com/jsdom/jsdom/issues/1782), but Phaser's device/feature
// detection (Phaser.Device.CanvasFeatures) unconditionally probes one at
// module-load time. This stub only needs to satisfy that probe well enough
// not to throw — it is never used to assert on actual pixel output, since
// FactoryScene's tests only cover data-flow (machines map), not rendering.
function createStub2DContext(width: number, height: number): Partial<CanvasRenderingContext2D> {
  return {
    fillStyle: "",
    globalCompositeOperation: "source-over",
    imageSmoothingEnabled: true,
    fillRect: () => {},
    drawImage: () => {},
    putImageData: () => {},
    getImageData: (_x, _y, w, h) => ({
      data: new Uint8ClampedArray((w ?? width) * (h ?? height) * 4),
      width: w ?? width,
      height: h ?? height,
      colorSpace: "srgb",
    }) as ImageData,
  };
}

// Phaser also feature-detects canvas support via `!!window.CanvasRenderingContext2D`
// (a constructor reference, not an actual context instance) — jsdom doesn't
// define this global either without the native "canvas" package.
if (typeof globalThis.CanvasRenderingContext2D === "undefined") {
  // @ts-expect-error minimal stand-in, only its existence is checked
  globalThis.CanvasRenderingContext2D = class CanvasRenderingContext2D {};
}

const originalGetContext = HTMLCanvasElement.prototype.getContext;
// @ts-expect-error overriding with a narrower, test-only signature
HTMLCanvasElement.prototype.getContext = function (this: HTMLCanvasElement, type: string, ...args: unknown[]) {
  if (type === "2d") {
    return createStub2DContext(this.width, this.height);
  }
  return originalGetContext.call(this, type as never, ...(args as []));
};
