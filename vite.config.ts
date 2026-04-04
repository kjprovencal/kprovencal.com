import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
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
