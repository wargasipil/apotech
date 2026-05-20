import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // The backend now serves Connect handlers UNDER /api (it strips the
      // prefix internally), matching how the embedded SPA is served in
      // production. So forward /api/* unchanged — no rewrite.
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
