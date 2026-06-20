import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const repoRoot = dirname(dirname(fileURLToPath(import.meta.url)));

async function readFrontendFile(path: string): Promise<string> {
  return readFile(join(repoRoot, "frontend", path), "utf8");
}

function ruleBody(css: string, selector: string): string {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = new RegExp(`${escapedSelector}\\s*\\{(?<body>[^}]*)\\}`, "m").exec(css);
  expect(match, `missing CSS rule for ${selector}`).not.toBeNull();
  return match?.groups?.body ?? "";
}

function declarations(body: string): Map<string, string> {
  const result = new Map<string, string>();
  for (const declaration of body.split(";")) {
    const separator = declaration.indexOf(":");
    if (separator === -1) continue;
    const property = declaration.slice(0, separator).trim();
    const value = declaration.slice(separator + 1).trim();
    if (property && value) {
      result.set(property, value);
    }
  }
  return result;
}

function px(value: string): number {
  const match = /^(\d+(?:\.\d+)?)px$/.exec(value.trim());
  expect(match, `expected pixel value, got ${value}`).not.toBeNull();
  return Number(match?.[1] ?? 0);
}

function firstPx(value: string): number {
  const match = /(\d+(?:\.\d+)?)px/.exec(value);
  expect(match, `expected declaration with pixel value, got ${value}`).not.toBeNull();
  return Number(match?.[1] ?? 0);
}

function lineHeightPx(value: string, fontSize: number): number {
  const trimmed = value.trim();
  if (trimmed.endsWith("px")) {
    return px(trimmed);
  }
  const numeric = Number(trimmed);
  expect(Number.isFinite(numeric), `expected numeric line-height, got ${value}`).toBe(true);
  return numeric * fontSize;
}

function verticalPaddingPx(value: string): number {
  const parts = value.trim().split(/\s+/);
  expect(parts.length, `expected padding declaration, got ${value}`).toBeGreaterThan(0);
  if (parts.length === 1) {
    return px(parts[0]) * 2;
  }
  return px(parts[0]) + px(parts[2] ?? parts[0]);
}

describe("frontend visual regressions", () => {
  it("keeps the top bar in normal page flow", async () => {
    const css = await readFrontendFile("styles.css");
    const topbar = ruleBody(css, ".topbar");

    expect(topbar).not.toMatch(/position\s*:\s*sticky/);
    expect(topbar).not.toMatch(/top\s*:/);
  });

  it("uses distinct notice styles for success and error feedback", async () => {
    const css = await readFrontendFile("styles.css");
    const main = await readFrontendFile("main.js");
    const baseNotice = ruleBody(css, ".notice");

    expect(baseNotice).not.toMatch(/color\s*:\s*var\(--error\)/);
    expect(css).toContain("--success-foreground");
    expect(ruleBody(css, ".notice-success")).toMatch(/var\(--success-foreground\)/);
    expect(ruleBody(css, ".notice-error")).toMatch(/var\(--error\)/);
    expect(main).toContain("notice-success");
    expect(main).toContain("notice-error");
  });

  it("renders provider metadata as wrapping structured details", async () => {
    const css = await readFrontendFile("styles.css");
    const main = await readFrontendFile("main.js");

    expect(main).toContain("provider-details");
    expect(main).toContain("provider-detail-value");
    expect(ruleBody(css, ".provider-detail-value")).toMatch(/min-width\s*:\s*0/);
    expect(ruleBody(css, ".provider-detail-value")).toMatch(/overflow-wrap\s*:\s*anywhere/);
  });

  it("keeps route table provider labels readable", async () => {
    const css = await readFrontendFile("styles.css");
    const main = await readFrontendFile("main.js");
    const providerCell = ruleBody(css, ".route-provider-cell");

    expect(main).toContain("route-provider-cell");
    expect(providerCell).toMatch(/white-space\s*:\s*nowrap/);
    expect(providerCell).toMatch(/min-width\s*:\s*120px/);
  });

  it("keeps form control text vertically unclipped", async () => {
    const css = await readFrontendFile("styles.css");
    const input = declarations(ruleBody(css, ".input"));
    const height = px(input.get("height") ?? "");
    const fontSize = px(input.get("font-size") ?? "");
    const contentHeight =
      lineHeightPx(input.get("line-height") ?? "", fontSize) +
      verticalPaddingPx(input.get("padding") ?? "") +
      firstPx(input.get("border") ?? "") * 2;

    expect(contentHeight).toBeLessThanOrEqual(height);
  });

  it("avoids dark dashboard panels and negative letter spacing", async () => {
    const css = await readFrontendFile("styles.css");
    const heavyBlock = ruleBody(css, ".block-heavy");

    expect(heavyBlock).not.toMatch(/surface-tile|#000|#272729|#2a2a2c|#252527/i);
    expect(css).not.toMatch(/letter-spacing\s*:\s*-/);
  });
});
