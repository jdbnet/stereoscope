import { defineConfig } from "vite";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [
    wails("./bindings"),
    {
      name: "android-webview-scripts",
      transformIndexHtml: {
        order: "post",
        handler(html) {
          // WebView2 and the Android WebView both drop a module script
          // marked crossorigin when the app itself serves the file.
          return html.replaceAll(" crossorigin", "");
        },
      },
    },
  ],
});
