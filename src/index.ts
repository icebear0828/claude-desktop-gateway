import { serve } from "@hono/node-server";
import { createGatewayApp } from "./app.js";
import { loadConfigFromEnv } from "./config.js";

export { createGatewayApp } from "./app.js";
export { loadConfigFromEnv, parseAliasEnv, resolveModelAlias } from "./config.js";

const isMain = import.meta.url === `file://${process.argv[1]}`;

if (isMain) {
  const config = loadConfigFromEnv();
  const app = createGatewayApp(config);

  serve(
    {
      fetch: app.fetch,
      hostname: config.host,
      port: config.port,
    },
    () => {
      console.log(`Claude Desktop gateway listening on http://${config.host}:${config.port}`);
      console.log(`Default OpenRouter model: ${config.defaultModel}`);
    },
  );
}
