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
      name: "inline-app-script",
      apply: "build",
      enforce: "post",
      generateBundle(_options, bundle) {
        const page = bundle["index.html"];
        if (!page || page.type !== "asset") return;
        let html = page.source.toString();
        const tag = html.match(/<script\b[^>]*\bsrc="([^"]+)"[^>]*><\/script>/);
        if (!tag) return;
        const name = tag[1].replace(/^\//, "");
        const script = bundle[name];
        if (!script || script.type !== "chunk") return;
        const code = script.code.replaceAll("</script", "<\\/script");
        // Run after #app exists. A module script in the head is deferred,
        // but both WebViews have been dropping that request.
        html = html.replace(tag[0], "");
        html = html.replace("</body>", `<script>${code}</script></body>`);
        page.source = html;
        delete bundle[name];
      },
    },
  ],
});
