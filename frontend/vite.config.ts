import { defineConfig } from "vite";

export default defineConfig({
  resolve: {
    // Vitest runs under Node and picks up Phaser's CommonJS "main" entry
    // (raw src/phaser.js), which unconditionally requires the dev-only
    // "phaser3spectorjs" debug package. Real (browser) Vite builds never
    // hit this because they resolve the "module" field instead — point
    // Vitest at the same prebuilt ESM bundle so tests exercise the same
    // code real users get.
    alias: {
      phaser: "phaser/dist/phaser.esm.js",
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./vitest.setup.ts"],
  },
});
