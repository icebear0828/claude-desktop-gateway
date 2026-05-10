import {
  applyTranslations,
  detectLocale,
  storeLocale,
  translate,
} from "./i18n.js";

const SECRET_NAMES = ["OPENROUTER_API_KEY", "CLAUDE_GATEWAY_API_KEY"];

const elements = {
  refresh: document.getElementById("refresh"),
  localeButtons: Array.from(document.querySelectorAll("[data-locale]")),
  gatewayState: document.getElementById("gateway-state"),
  listenUrl: document.getElementById("listen-url"),
  healthUrl: document.getElementById("health-url"),
  gatewayPid: document.getElementById("gateway-pid"),
  gatewayLog: document.getElementById("gateway-log"),
  gatewayDetail: document.getElementById("gateway-detail"),
  configState: document.getElementById("config-state"),
  configPath: document.getElementById("config-path"),
  projectRoot: document.getElementById("project-root"),
  configError: document.getElementById("config-error"),
  editorState: document.getElementById("editor-state"),
  envPath: document.getElementById("env-path"),
  secretList: document.getElementById("secret-list"),
  editorMessage: document.getElementById("editor-message"),
  modelCount: document.getElementById("model-count"),
  routesBody: document.getElementById("routes-body"),
  routeForm: document.getElementById("route-form"),
  routeOriginalId: document.getElementById("route-original-id"),
  routeDesktopId: document.getElementById("route-desktop-id"),
  routeDisplayName: document.getElementById("route-display-name"),
  routeUpstreamModel: document.getElementById("route-upstream-model"),
  routeProvider: document.getElementById("route-provider"),
  routeSuggest: document.getElementById("route-suggest"),
  routeReset: document.getElementById("route-reset"),
  providerCount: document.getElementById("provider-count"),
  providersList: document.getElementById("providers-list"),
  desktopState: document.getElementById("desktop-state"),
  desktopApplied: document.getElementById("desktop-applied"),
  desktopProfile: document.getElementById("desktop-profile"),
  desktopIssues: document.getElementById("desktop-issues"),
  generatedAt: document.getElementById("generated-at"),
};

let currentEditor = null;
let currentDashboard = null;
let currentLocale = detectLocale();

function t(key, params = {}) {
  return translate(currentLocale, key, params);
}

function bridge(name) {
  const method = globalThis.go?.main?.App?.[name];
  return typeof method === "function" ? method : null;
}

function setText(node, value) {
  if (!node) return;
  const text = value === undefined || value === null || value === "" ? "-" : String(value);
  node.textContent = text;
}

function setStatus(node, state) {
  if (!node) return;
  const normalized = state || "unknown";
  node.removeAttribute("data-i18n");
  node.dataset.statusState = normalized;
  node.textContent = t(`status.${normalized}`);
  node.className = `status status-${normalized}`;
}

function clear(node) {
  if (!node) return;
  while (node.firstChild) {
    node.removeChild(node.firstChild);
  }
}

function appendCell(row, text) {
  const cell = document.createElement("td");
  cell.textContent = text || "-";
  row.appendChild(cell);
}

function appendButton(parent, key, className, onClick) {
  const button = document.createElement("button");
  button.className = className;
  button.type = "button";
  button.setAttribute("data-i18n", key);
  button.textContent = t(key);
  button.setAttribute("data-text-en", translate("en", key));
  button.setAttribute("data-text-zh", translate("zh-CN", key));
  button.addEventListener("click", onClick);
  parent.appendChild(button);
}

function showEditorMessage(message, state = "error") {
  setStatus(elements.editorState, state);
  setText(elements.editorMessage, message);
  elements.editorMessage?.classList.toggle("hidden", !message);
}

function updateLocaleControls() {
  const switcher = document.querySelector(".language-switcher");
  if (switcher) {
    switcher.setAttribute("data-active-locale", currentLocale);
  }
  for (const button of elements.localeButtons) {
    const isActive = button.getAttribute("data-locale") === currentLocale;
    button.classList.toggle("button-active", isActive);
    button.setAttribute("aria-pressed", String(isActive));
  }
}

function setLocale(locale) {
  currentLocale = storeLocale(locale);
  applyTranslations(document, currentLocale);
  updateLocaleControls();
  rerenderLocalizedState();
}

function rerenderLocalizedState() {
  const routeFormState = captureRouteFormState();
  const secretValues = captureSecretInputValues();
  if (currentDashboard) {
    renderDashboard(currentDashboard);
  }
  if (currentEditor) {
    setStatus(elements.editorState, elements.editorState?.dataset.statusState || "ok");
    renderSecrets(currentEditor.secrets || []);
    renderRoutes(currentEditor.config?.routes || []);
    restoreSecretInputValues(secretValues);
  }
  restoreRouteFormState(routeFormState);
}

function capValue(caps, key) {
  return Boolean(caps?.[key] ?? caps?.[key.charAt(0).toLowerCase() + key.slice(1)]);
}

function capabilityText(name, enabled) {
  return t("providers.capability", { name, state: enabled ? "on" : "off" });
}

function renderRoutes(routes) {
  clear(elements.routesBody);
  setText(elements.modelCount, t("models.count", { count: routes.length }));
  if (routes.length === 0) {
    const row = document.createElement("tr");
    const cell = document.createElement("td");
    cell.colSpan = 5;
    cell.textContent = t("models.noRoutes");
    row.appendChild(cell);
    elements.routesBody.appendChild(row);
    return;
  }
  for (const route of routes) {
    const row = document.createElement("tr");
    appendCell(row, route.desktopID);
    appendCell(row, route.displayName);
    appendCell(row, route.provider);
    appendCell(row, route.upstreamModel);

    const actionCell = document.createElement("td");
    actionCell.className = "table-actions";
    appendButton(actionCell, "models.edit", "button button-small button-secondary", () => populateRouteForm(route));
    appendButton(actionCell, "models.delete", "button button-small button-destructive", () => deleteRoute(route.desktopID));
    row.appendChild(actionCell);
    elements.routesBody.appendChild(row);
  }
}

function renderProviders(providers) {
  clear(elements.providersList);
  setText(elements.providerCount, String(providers.length));
  if (providers.length === 0) {
    const empty = document.createElement("p");
    empty.className = "mono-line";
    empty.textContent = t("providers.empty");
    elements.providersList.appendChild(empty);
    return;
  }
  for (const provider of providers) {
    const item = document.createElement("section");
    item.className = "list-item";
    const title = document.createElement("h3");
    title.textContent = provider.name;
    const meta = document.createElement("p");
    meta.className = "mono-line";
    const caps = provider.capabilities || {};
    meta.textContent = [
      provider.profile,
      provider.baseUrl,
      `key=${provider.apiKeyEnv || "-"}`,
      capabilityText("stream", capValue(caps, "Streaming")),
      capabilityText("tools", capValue(caps, "Tools")),
      capabilityText("json", capValue(caps, "JSONMode")),
    ].join(" / ");
    item.appendChild(title);
    item.appendChild(meta);
    elements.providersList.appendChild(item);
  }
}

function renderIssues(issues) {
  clear(elements.desktopIssues);
  if (issues.length === 0) {
    const ok = document.createElement("p");
    ok.className = "mono-line";
    ok.textContent = t("desktop.noIssues");
    elements.desktopIssues.appendChild(ok);
    return;
  }
  for (const issue of issues) {
    const item = document.createElement("section");
    item.className = `issue issue-${issue.severity || "warning"}`;
    const title = document.createElement("h3");
    title.textContent = `${t(`status.${issue.severity || "warning"}`)} / ${issue.code || "issue"}`;
    const message = document.createElement("p");
    message.textContent = issue.message || "-";
    const path = document.createElement("p");
    path.className = "mono-line";
    path.textContent = issue.path || "-";
    item.appendChild(title);
    item.appendChild(message);
    item.appendChild(path);
    elements.desktopIssues.appendChild(item);
  }
}

function fallbackDashboard() {
  return {
    projectRoot: "-",
    configPath: "gateway.local.json",
    configError: t("editor.bridgeLiveUnavailable"),
    listenUrl: "http://127.0.0.1:8787",
    gateway: {
      state: "stopped",
      managed: false,
      pid: "",
      healthUrl: "http://127.0.0.1:8787/health",
      logPath: ".local-gateway/gateway.log",
      detail: t("editor.offlinePreview"),
    },
    providers: [],
    routes: [],
    claudeDesktop: {
      state: "warning",
      appliedId: "",
      activeProfilePath: "",
      issues: [],
    },
    generatedAtIso: new Date().toISOString(),
  };
}

function fallbackEditor() {
  return {
    envPath: ".env.local",
    secrets: SECRET_NAMES.map((name) => ({ name, present: false, value: "" })),
    config: {
      path: "gateway.local.json",
      host: "127.0.0.1",
      port: 8787,
      gatewayApiKeyEnv: "CLAUDE_GATEWAY_API_KEY",
      tlsCertFile: "",
      tlsKeyFile: "",
      providers: [
        {
          name: "openrouter",
          profile: "openai-chat",
          baseUrl: "https://openrouter.ai/api/v1",
          apiKeyEnv: "OPENROUTER_API_KEY",
          referrer: "",
          title: "Claude Gateway",
          capabilities: { Streaming: true, Tools: true, JSONMode: false },
        },
      ],
      routes: [],
    },
  };
}

async function fetchDashboard() {
  const method = bridge("Dashboard");
  if (method) {
    return method();
  }
  return fallbackDashboard();
}

async function fetchEditor() {
  const method = bridge("Editor");
  if (method) {
    return method();
  }
  return fallbackEditor();
}

function localizeGatewayDetail(detail) {
  if (detail === "health check failed") {
    return t("gateway.healthFailed");
  }
  if (detail === "health check OK") {
    return t("gateway.healthOk");
  }
  const match = String(detail || "").match(/^health returned HTTP (\d+)$/);
  if (match) {
    return t("gateway.healthReturned", { status: match[1] });
  }
  return detail;
}

function renderDashboard(dashboard) {
  setText(elements.projectRoot, dashboard.projectRoot);
  setText(elements.configPath, dashboard.configPath);
  setText(elements.listenUrl, dashboard.listenUrl);
  setText(elements.healthUrl, dashboard.gateway?.healthUrl);
  setText(elements.gatewayPid, dashboard.gateway?.managed ? dashboard.gateway.pid : t("gateway.unmanaged"));
  setText(elements.gatewayLog, dashboard.gateway?.logPath);
  setText(elements.gatewayDetail, localizeGatewayDetail(dashboard.gateway?.detail));
  setStatus(elements.gatewayState, dashboard.gateway?.state);

  const hasConfigError = Boolean(dashboard.configError);
  setStatus(elements.configState, hasConfigError ? "error" : "ok");
  setText(elements.configError, dashboard.configError);
  elements.configError?.classList.toggle("hidden", !hasConfigError);

  renderProviders(dashboard.providers || []);

  const desktop = dashboard.claudeDesktop || { state: "unknown", issues: [] };
  setStatus(elements.desktopState, desktop.state);
  setText(elements.desktopApplied, desktop.appliedId);
  setText(elements.desktopProfile, desktop.activeProfilePath);
  renderIssues(desktop.issues || []);

  setText(elements.generatedAt, t("footer.updated", { time: dashboard.generatedAtIso || "-" }));
}

function renderEditor(editor) {
  currentEditor = editor;
  setStatus(elements.editorState, "ok");
  setText(elements.envPath, editor.envPath);
  elements.editorMessage?.classList.add("hidden");
  renderSecrets(editor.secrets || []);
  renderProviderOptions(editor.config?.providers || []);
  renderRoutes(editor.config?.routes || []);
  if (!elements.routeProvider?.value) {
    resetRouteForm();
  }
}

function renderSecrets(secrets) {
  clear(elements.secretList);
  const byName = new Map(secrets.map((secret) => [secret.name, secret]));
  for (const name of SECRET_NAMES) {
    const secret = byName.get(name) || { name, present: false };
    const row = document.createElement("section");
    row.className = "secret-row";

    const label = document.createElement("div");
    const title = document.createElement("h3");
    title.textContent = name;
    const state = document.createElement("p");
    state.className = "mono-line";
    state.textContent = secret.present ? t("keys.set") : t("keys.unset");
    label.appendChild(title);
    label.appendChild(state);

    const input = document.createElement("input");
    input.className = "input";
    input.type = "password";
    input.autocomplete = "off";
    input.placeholder = secret.present ? t("keys.enterNew") : t("keys.enterValue");
    input.setAttribute("aria-label", name);
    input.setAttribute("data-secret-name", name);

    const actions = document.createElement("div");
    actions.className = "actions";
    appendButton(actions, "actions.save", "button button-small button-primary", () => saveSecret(name, input.value));
    appendButton(actions, "actions.delete", "button button-small button-destructive", () => deleteSecret(name));

    row.appendChild(label);
    row.appendChild(input);
    row.appendChild(actions);
    elements.secretList.appendChild(row);
  }
}

function renderProviderOptions(providers) {
  clear(elements.routeProvider);
  for (const provider of providers) {
    const option = document.createElement("option");
    option.value = provider.name;
    option.textContent = provider.name;
    elements.routeProvider.appendChild(option);
  }
}

function captureRouteFormState() {
  return {
    originalID: elements.routeOriginalId?.value || "",
    desktopID: elements.routeDesktopId?.value || "",
    displayName: elements.routeDisplayName?.value || "",
    upstreamModel: elements.routeUpstreamModel?.value || "",
    provider: elements.routeProvider?.value || "",
  };
}

function restoreRouteFormState(state) {
  if (!state || !elements.routeForm) return;
  elements.routeOriginalId.value = state.originalID;
  elements.routeDesktopId.value = state.desktopID;
  elements.routeDisplayName.value = state.displayName;
  elements.routeUpstreamModel.value = state.upstreamModel;
  if (state.provider) {
    elements.routeProvider.value = state.provider;
  }
}

function captureSecretInputValues() {
  const values = new Map();
  for (const input of document.querySelectorAll("[data-secret-name]")) {
    values.set(input.getAttribute("data-secret-name"), input.value);
  }
  return values;
}

function restoreSecretInputValues(values) {
  for (const input of document.querySelectorAll("[data-secret-name]")) {
    const name = input.getAttribute("data-secret-name");
    if (values.has(name)) {
      input.value = values.get(name);
    }
  }
}

function resetRouteForm() {
  if (!elements.routeForm) return;
  elements.routeOriginalId.value = "";
  elements.routeDesktopId.value = "";
  elements.routeDisplayName.value = "";
  elements.routeUpstreamModel.value = "";
  const providers = currentEditor?.config?.providers || [];
  elements.routeProvider.value = providers[0]?.name || "openrouter";
}

function populateRouteForm(route) {
  elements.routeOriginalId.value = route.desktopID || "";
  elements.routeDesktopId.value = route.desktopID || "";
  elements.routeDisplayName.value = route.displayName || "";
  elements.routeUpstreamModel.value = route.upstreamModel || "";
  elements.routeProvider.value = route.provider || "openrouter";
  elements.routeDesktopId.focus();
}

function normalizedEditorConfig() {
  const base = currentEditor?.config || fallbackEditor().config;
  return {
    path: base.path,
    host: base.host || "127.0.0.1",
    port: Number(base.port) || 8787,
    gatewayApiKeyEnv: base.gatewayApiKeyEnv || "CLAUDE_GATEWAY_API_KEY",
    tlsCertFile: base.tlsCertFile || "",
    tlsKeyFile: base.tlsKeyFile || "",
    providers: base.providers || [],
    routes: base.routes || [],
  };
}

async function saveEditorConfig(nextConfig) {
  const method = bridge("SaveConfig");
  if (!method) {
    showEditorMessage(t("editor.bridgeConfigUnavailable"));
    return;
  }
  try {
    const editor = await method(nextConfig);
    renderEditor(editor);
    await loadDashboardOnly();
    showEditorMessage(t("editor.configSaved"), "ok");
  } catch (error) {
    showEditorMessage(error instanceof Error ? error.message : String(error));
  }
}

async function saveSecret(name, value) {
  if (!value) {
    showEditorMessage(t("keys.valueRequired", { name }));
    return;
  }
  const method = bridge("SaveSecret");
  if (!method) {
    showEditorMessage(t("editor.bridgeKeysSaveUnavailable"));
    return;
  }
  try {
    const editor = await method({ name, value });
    renderEditor(editor);
    showEditorMessage(t("keys.saved", { name }), "ok");
  } catch (error) {
    showEditorMessage(error instanceof Error ? error.message : String(error));
  }
}

async function deleteSecret(name) {
  if (!window.confirm(t("keys.deleteConfirm", { name }))) {
    return;
  }
  const method = bridge("DeleteSecret");
  if (!method) {
    showEditorMessage(t("editor.bridgeKeysDeleteUnavailable"));
    return;
  }
  try {
    const editor = await method({ name });
    renderEditor(editor);
    showEditorMessage(t("keys.deleted", { name }), "ok");
  } catch (error) {
    showEditorMessage(error instanceof Error ? error.message : String(error));
  }
}

async function suggestDesktopID() {
  const upstream = elements.routeUpstreamModel.value.trim();
  if (!upstream) {
    showEditorMessage(t("models.upstreamRequired"));
    return;
  }
  const method = bridge("SuggestedDesktopID");
  elements.routeDesktopId.value = method ? await method(upstream) : localSuggestedDesktopID(upstream);
}

function localSuggestedDesktopID(upstream) {
  if (upstream.startsWith("claude-") || upstream.startsWith("anthropic/claude-")) {
    return upstream;
  }
  return `claude-${upstream}`;
}

async function saveRoute(event) {
  event.preventDefault();
  const config = normalizedEditorConfig();
  const originalID = elements.routeOriginalId.value.trim();
  const upstreamModel = elements.routeUpstreamModel.value.trim();
  const desktopID = elements.routeDesktopId.value.trim() || localSuggestedDesktopID(upstreamModel);
  const provider = elements.routeProvider.value.trim() || "openrouter";
  const displayName = elements.routeDisplayName.value.trim() || desktopID;

  if (!upstreamModel) {
    showEditorMessage(t("models.upstreamRequired"));
    return;
  }
  if (!desktopID) {
    showEditorMessage(t("models.desktopRequired"));
    return;
  }
  if (config.routes.some((route) => route.desktopID === desktopID && route.desktopID !== originalID)) {
    showEditorMessage(t("models.duplicate", { id: desktopID }));
    return;
  }

  const nextRoute = { desktopID, provider, upstreamModel, displayName };
  const nextRoutes = config.routes.filter((route) => route.desktopID !== originalID && route.desktopID !== desktopID);
  nextRoutes.push(nextRoute);
  nextRoutes.sort((left, right) => left.desktopID.localeCompare(right.desktopID));
  await saveEditorConfig({ ...config, routes: nextRoutes });
  resetRouteForm();
}

async function deleteRoute(desktopID) {
  if (!window.confirm(t("models.deleteConfirm", { id: desktopID }))) {
    return;
  }
  const config = normalizedEditorConfig();
  const nextRoutes = config.routes.filter((route) => route.desktopID !== desktopID);
  await saveEditorConfig({ ...config, routes: nextRoutes });
}

async function loadDashboardOnly() {
  try {
    const dashboard = await fetchDashboard();
    currentDashboard = dashboard;
    renderDashboard(dashboard);
  } catch (error) {
    currentDashboard = {
      ...fallbackDashboard(),
      configError: error instanceof Error ? error.message : String(error),
      generatedAtIso: new Date().toISOString(),
    };
    renderDashboard(currentDashboard);
  }
}

async function loadAll() {
  elements.refresh?.setAttribute("disabled", "true");
  try {
    await loadDashboardOnly();
    const editor = await fetchEditor();
    renderEditor(editor);
  } catch (error) {
    showEditorMessage(error instanceof Error ? error.message : String(error));
  } finally {
    elements.refresh?.removeAttribute("disabled");
  }
}

elements.refresh?.addEventListener("click", loadAll);
for (const button of elements.localeButtons) {
  button.addEventListener("click", () => setLocale(button.getAttribute("data-locale")));
}
elements.routeForm?.addEventListener("submit", saveRoute);
elements.routeSuggest?.addEventListener("click", suggestDesktopID);
elements.routeReset?.addEventListener("click", resetRouteForm);

applyTranslations(document, currentLocale);
updateLocaleControls();
void loadAll();
