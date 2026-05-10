import { describe, expect, it } from "vitest";

import { mergeEditedRoute } from "../frontend/routes.js";

describe("route editor helpers", () => {
  it("preserves dynamic free model and cache metadata when editing an existing route", () => {
    const routes = [
      {
        desktopID: "claude-free-agent",
        provider: "openrouter",
        upstreamModel: "openrouter/free",
        displayName: "OpenRouter Free Agent Auto",
        dynamicFreeModels: {
          enabled: true,
          requiredParameters: ["tools", "tool_choice"],
          minContextLength: 32768,
          maxModels: 4,
          catalogCacheTTLSeconds: 900,
          fallback: ["inclusionai/ring-2.6-1t:free", "openrouter/free"],
        },
        cache: {
          enabled: true,
          ttlSeconds: 300,
        },
      },
    ];

    const next = mergeEditedRoute(routes, "claude-free-agent", {
      desktopID: "claude-free-agent",
      provider: "openrouter",
      upstreamModel: "openrouter/free",
      displayName: "Agent",
    });

    expect(next).toEqual([
      {
        desktopID: "claude-free-agent",
        provider: "openrouter",
        upstreamModel: "openrouter/free",
        displayName: "Agent",
        dynamicFreeModels: {
          enabled: true,
          requiredParameters: ["tools", "tool_choice"],
          minContextLength: 32768,
          maxModels: 4,
          catalogCacheTTLSeconds: 900,
          fallback: ["inclusionai/ring-2.6-1t:free", "openrouter/free"],
        },
        cache: {
          enabled: true,
          ttlSeconds: 300,
        },
      },
    ]);
  });
});
