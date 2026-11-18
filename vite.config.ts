import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// Production CSP (markdown-render-pipeline skill §5): no inline scripts, no
// remote images except the kbimg protocol, no frames or objects.
const CSP =
  "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
  "img-src 'self' data: http://kbimg.localhost; font-src 'self'; connect-src 'self' http://kbimg.localhost; " +
  "object-src 'none'; frame-src 'none'; base-uri 'none'; form-action 'none'";

const cspPlugin = {
  name: "mkb-csp",
  apply: "build" as const,
  transformIndexHtml(html: string) {
    return html.replace("</head>", `    <meta http-equiv="Content-Security-Policy" content="${CSP}" />
  </head>`);
  },
};

export default defineConfig({
  plugins: [react(), tailwindcss(), cspPlugin],
  server: {
    // The app loads this address during `mygo dev` (devUrl in mygo.config.ts).
    port: 5274,
    strictPort: true,
    // The development app and the packaged builds are not the page's.
    watch: { ignored: ["**/.mygo/**", "**/build/**"] },
  },
});
