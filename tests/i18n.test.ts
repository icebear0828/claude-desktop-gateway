import { describe, expect, it } from "vitest";

import {
  detectLocale,
  formatMessage,
  isSupportedLocale,
  normalizeLocale,
  storeLocale,
  translate,
} from "../frontend/i18n.js";

describe("frontend i18n", () => {
  it("normalizes supported locales and falls back to English", () => {
    expect(normalizeLocale("zh-CN")).toBe("zh-CN");
    expect(normalizeLocale("zh-Hans")).toBe("zh-CN");
    expect(normalizeLocale("en-US")).toBe("en");
    expect(normalizeLocale("fr-FR")).toBe("en");
    expect(isSupportedLocale("zh-CN")).toBe(true);
    expect(isSupportedLocale("zh-TW")).toBe(false);
  });

  it("detects locale from storage before browser language", () => {
    const storage = createMemoryStorage({ claudeGatewayLocale: "zh-CN" });

    expect(detectLocale({ storage, navigatorLanguage: "en-US" })).toBe("zh-CN");
    expect(detectLocale({ storage: createMemoryStorage(), navigatorLanguage: "zh-Hans" })).toBe("zh-CN");
    expect(detectLocale({ storage: createMemoryStorage(), navigatorLanguage: "de-DE" })).toBe("en");
  });

  it("translates with interpolation and English fallback", () => {
    expect(translate("zh-CN", "nav.refresh")).toBe("刷新");
    expect(translate("zh-CN", "models.duplicate", { id: "claude-test" })).toBe("模型 claude-test 已存在");
    expect(translate("zh-CN", "missing.key")).toBe("missing.key");
    expect(translate("zh-CN", "providers.capability", { name: "stream", state: "on" })).toBe("stream=开");
  });

  it("formats route counts in both locales", () => {
    expect(formatMessage("en", "models.count", { count: 1 })).toBe("1 ROUTE");
    expect(formatMessage("en", "models.count", { count: 2 })).toBe("2 ROUTES");
    expect(formatMessage("zh-CN", "models.count", { count: 2 })).toBe("2 个路由");
  });

  it("translates Claude Desktop repair actions", () => {
    expect(translate("en", "desktop.repair")).toBe("REPAIR CONFIG");
    expect(translate("zh-CN", "desktop.repair")).toBe("修复配置");
    expect(translate("zh-CN", "desktop.repairDone")).toContain("重启 Claude Desktop");
  });

  it("stores only supported locale values", () => {
    const storage = createMemoryStorage();

    storeLocale("zh-CN", storage);
    expect(storage.getItem("claudeGatewayLocale")).toBe("zh-CN");

    storeLocale("fr-FR", storage);
    expect(storage.getItem("claudeGatewayLocale")).toBe("en");
  });
});

type MemoryStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

function createMemoryStorage(initial: Record<string, string> = {}): MemoryStorage {
  const values = new Map<string, string>(Object.entries(initial));
  return {
    getItem(key: string): string | null {
      return values.get(key) ?? null;
    },
    setItem(key: string, value: string): void {
      values.set(key, value);
    },
    removeItem(key: string): void {
      values.delete(key);
    },
  };
}
