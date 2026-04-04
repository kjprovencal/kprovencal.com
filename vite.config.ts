import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    coverage: {
      provider: "v8",
      reportsDirectory: "./coverage",
      reporter: ["text", "text-summary", "lcov", "json-summary"],
      include: ["src/**/*.ts"],
      exclude: [
        "**/node_modules/**",
        "src/**/*.test.ts",
        "src/vite-env.d.ts",
        // App shell / DOM wiring — covered by manual or future E2E tests, not Vitest.
        "src/main.ts",
        "src/mount-admin.ts",
        "src/mount-wedding-rsvp.ts",
        "src/theme.ts",
      ],
    },
  },
  server: {
    port: 5173,
    // Dev only: forward /api and /admin to Go. Prod: set VITE_PUBLIC_API_URL on the static build.
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
      },
      "/admin": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
        // Let the SPA route `GET /admin` (Accept: text/html); proxy API/admin fetches.
        bypass(req) {
          const raw = req.url?.split("?")[0] ?? "";
          if (req.method !== "GET" || raw !== "/admin") return;
          const accept = req.headers.accept ?? "";
          if (accept.includes("text/html")) {
            return "/index.html";
          }
        },
      },
    },
  },
  build: {
    rollupOptions: {
      input: "index.html",
    },
  },
});
