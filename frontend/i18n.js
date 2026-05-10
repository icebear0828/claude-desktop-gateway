export const STORAGE_KEY = "claudeGatewayLocale";
export const SUPPORTED_LOCALES = ["en", "zh-CN"];

export const messages = {
  en: {
    "app.title": "Claude Desktop Gateway",
    "nav.kicker": "LOCAL CONSOLE",
    "nav.refresh": "REFRESH",
    "nav.language": "LANGUAGE",
    "nav.english": "EN",
    "nav.chinese": "中文",
    "actions.save": "SAVE",
    "actions.delete": "DELETE",
    "sections.gatewayOverview": "Gateway overview",
    "sections.providersAndDesktop": "Providers and Claude Desktop",
    "gateway.title": "GATEWAY",
    "gateway.listenUrl": "LISTEN URL",
    "gateway.health": "HEALTH",
    "gateway.pid": "PID",
    "gateway.log": "LOG",
    "gateway.unmanaged": "UNMANAGED",
    "gateway.healthFailed": "health check failed",
    "gateway.healthOk": "health check OK",
    "gateway.healthReturned": "health returned HTTP {status}",
    "config.title": "CONFIG",
    "config.file": "FILE",
    "config.project": "PROJECT",
    "keys.title": "KEYS",
    "keys.set": "SET IN ENV",
    "keys.unset": "NOT SET",
    "keys.enterNew": "ENTER NEW VALUE TO REPLACE",
    "keys.enterValue": "ENTER VALUE",
    "keys.valueRequired": "{name} value is required",
    "keys.deleteConfirm": "Delete {name} from .env.local?",
    "keys.saved": "{name} SAVED",
    "keys.deleted": "{name} DELETED",
    "models.title": "MODELS",
    "models.count": ({ count }) => `${count} ${Number(count) === 1 ? "ROUTE" : "ROUTES"}`,
    "models.noRoutes": "NO ROUTES",
    "models.desktopId": "DESKTOP ID",
    "models.displayName": "DISPLAY NAME",
    "models.upstreamModel": "UPSTREAM MODEL",
    "models.upstream": "UPSTREAM",
    "models.provider": "PROVIDER",
    "models.action": "ACTION",
    "models.save": "SAVE MODEL",
    "models.suggest": "SUGGEST ID",
    "models.reset": "RESET",
    "models.edit": "EDIT",
    "models.delete": "DELETE",
    "models.upstreamRequired": "UPSTREAM MODEL is required",
    "models.desktopRequired": "DESKTOP ID is required",
    "models.duplicate": "MODEL {id} already exists",
    "models.deleteConfirm": "Delete model {id}?",
    "providers.title": "PROVIDERS",
    "providers.empty": "NO PROVIDERS",
    "providers.capability": ({ name, state }) => `${name}=${state}`,
    "providers.on": "on",
    "providers.off": "off",
    "desktop.title": "CLAUDE DESKTOP",
    "desktop.appliedId": "APPLIED ID",
    "desktop.activeProfile": "ACTIVE PROFILE",
    "desktop.noIssues": "NO CONFIG ISSUES FOUND",
    "status.unknown": "UNKNOWN",
    "status.ok": "OK",
    "status.error": "ERROR",
    "status.warning": "WARNING",
    "status.info": "INFO",
    "status.stopped": "STOPPED",
    "status.running": "RUNNING",
    "editor.configSaved": "CONFIG SAVED",
    "editor.bridgeConfigUnavailable": "Wails bridge unavailable. Launch the desktop app to save config.",
    "editor.bridgeKeysSaveUnavailable": "Wails bridge unavailable. Launch the desktop app to save keys.",
    "editor.bridgeKeysDeleteUnavailable": "Wails bridge unavailable. Launch the desktop app to delete keys.",
    "editor.bridgeLiveUnavailable": "Wails bridge unavailable. Launch the desktop app with Wails to load live state.",
    "editor.offlinePreview": "offline preview",
    "footer.updated": "UPDATED {time}",
  },
  "zh-CN": {
    "app.title": "Claude Desktop Gateway",
    "nav.kicker": "本地控制台",
    "nav.refresh": "刷新",
    "nav.language": "语言",
    "nav.english": "EN",
    "nav.chinese": "中文",
    "actions.save": "保存",
    "actions.delete": "删除",
    "sections.gatewayOverview": "网关概览",
    "sections.providersAndDesktop": "供应商与 Claude Desktop",
    "gateway.title": "网关",
    "gateway.listenUrl": "监听地址",
    "gateway.health": "健康检查",
    "gateway.pid": "进程 ID",
    "gateway.log": "日志",
    "gateway.unmanaged": "未托管",
    "gateway.healthFailed": "健康检查失败",
    "gateway.healthOk": "健康检查正常",
    "gateway.healthReturned": "健康检查返回 HTTP {status}",
    "config.title": "配置",
    "config.file": "文件",
    "config.project": "项目",
    "keys.title": "密钥",
    "keys.set": "已写入环境",
    "keys.unset": "未设置",
    "keys.enterNew": "输入新值以替换",
    "keys.enterValue": "输入密钥",
    "keys.valueRequired": "{name} 不能为空",
    "keys.deleteConfirm": "从 .env.local 删除 {name}？",
    "keys.saved": "{name} 已保存",
    "keys.deleted": "{name} 已删除",
    "models.title": "模型",
    "models.count": ({ count }) => `${count} 个路由`,
    "models.noRoutes": "暂无路由",
    "models.desktopId": "桌面端 ID",
    "models.displayName": "显示名称",
    "models.upstreamModel": "上游模型",
    "models.upstream": "上游",
    "models.provider": "供应商",
    "models.action": "操作",
    "models.save": "保存模型",
    "models.suggest": "建议 ID",
    "models.reset": "重置",
    "models.edit": "编辑",
    "models.delete": "删除",
    "models.upstreamRequired": "上游模型不能为空",
    "models.desktopRequired": "桌面端 ID 不能为空",
    "models.duplicate": "模型 {id} 已存在",
    "models.deleteConfirm": "删除模型 {id}？",
    "providers.title": "供应商",
    "providers.empty": "暂无供应商",
    "providers.capability": ({ name, state }) => `${name}=${state === "on" ? "开" : "关"}`,
    "providers.on": "开",
    "providers.off": "关",
    "desktop.title": "Claude Desktop",
    "desktop.appliedId": "当前配置 ID",
    "desktop.activeProfile": "当前配置文件",
    "desktop.noIssues": "未发现配置问题",
    "status.unknown": "未知",
    "status.ok": "正常",
    "status.error": "错误",
    "status.warning": "警告",
    "status.info": "信息",
    "status.stopped": "已停止",
    "status.running": "运行中",
    "editor.configSaved": "配置已保存",
    "editor.bridgeConfigUnavailable": "Wails 桥接不可用。请通过桌面应用保存配置。",
    "editor.bridgeKeysSaveUnavailable": "Wails 桥接不可用。请通过桌面应用保存密钥。",
    "editor.bridgeKeysDeleteUnavailable": "Wails 桥接不可用。请通过桌面应用删除密钥。",
    "editor.bridgeLiveUnavailable": "Wails 桥接不可用。请通过 Wails 桌面应用加载实时状态。",
    "editor.offlinePreview": "离线预览",
    "footer.updated": "已更新 {time}",
  },
};

export function isSupportedLocale(locale) {
  return SUPPORTED_LOCALES.includes(locale);
}

export function normalizeLocale(locale) {
  const value = String(locale || "").trim();
  if (isSupportedLocale(value)) {
    return value;
  }
  const lower = value.toLowerCase();
  if (lower.startsWith("zh")) {
    return "zh-CN";
  }
  return "en";
}

export function detectLocale(options = {}) {
  const storage = options.storage ?? safeLocalStorage();
  const stored = storage?.getItem(STORAGE_KEY);
  if (stored && isSupportedLocale(stored)) {
    return stored;
  }
  return normalizeLocale(options.navigatorLanguage ?? globalThis.navigator?.language);
}

export function storeLocale(locale, storage = safeLocalStorage()) {
  const normalized = normalizeLocale(locale);
  storage?.setItem(STORAGE_KEY, normalized);
  return normalized;
}

export function translate(locale, key, params = {}) {
  const normalized = normalizeLocale(locale);
  const message = messages[normalized]?.[key] ?? messages.en[key] ?? key;
  if (typeof message === "function") {
    return String(message(params));
  }
  return interpolate(String(message), params);
}

export function formatMessage(locale, key, params = {}) {
  return translate(locale, key, params);
}

export function applyTranslations(root, locale) {
  const normalized = normalizeLocale(locale);
  const scope = root ?? globalThis.document;
  if (!scope) {
    return normalized;
  }
  const documentElement = scope.documentElement ?? globalThis.document?.documentElement;
  if (documentElement) {
    documentElement.lang = normalized;
  }
  for (const node of scope.querySelectorAll("[data-i18n]")) {
    const key = node.getAttribute("data-i18n");
    node.textContent = translate(normalized, key);
    if (node.classList?.contains("button")) {
      node.setAttribute("data-text-en", translate("en", key));
      node.setAttribute("data-text-zh", translate("zh-CN", key));
    }
  }
  for (const node of scope.querySelectorAll("[data-i18n-attr]")) {
    const raw = node.getAttribute("data-i18n-attr") || "";
    for (const pair of raw.split(",")) {
      const [attribute, key] = pair.split(":").map((part) => part.trim());
      if (attribute && key) {
        node.setAttribute(attribute, translate(normalized, key));
      }
    }
  }
  return normalized;
}

function interpolate(template, params) {
  return template.replace(/\{([a-zA-Z0-9_]+)\}/g, (match, key) => {
    const value = params[key];
    if (value === undefined || value === null) {
      return match;
    }
    return String(value);
  });
}

function safeLocalStorage() {
  try {
    return globalThis.localStorage;
  } catch {
    return null;
  }
}
