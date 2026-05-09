# GUI Bilingual i18n Plan

## Objective

Add a lightweight English / Simplified Chinese localization layer for the Wails GUI. The goal is for all user-facing interface copy to switch language without changing gateway behavior, config shape, model IDs, env var names, URLs, paths, logs, or provider error codes.

## Assumptions

- `i8n` means `i18n`.
- Scope is GUI-only first: `frontend/index.html`, `frontend/main.js`, and user-facing Wails GUI messages.
- Initial locales are `en` and `zh-CN`.
- Default language follows `localStorage`, then browser/system language, then English.
- No dependency is added for the first slice; use a small local dictionary module.

## Architecture Decisions

- Add `frontend/i18n.js` with message dictionaries, `t(key, params)`, locale detection, persistence, and a locale-change event.
- Replace static text in `index.html` with `data-i18n` attributes and apply translations on load.
- Replace dynamic hardcoded strings in `main.js` with translation keys.
- Keep protocol/config identifiers untranslated: `OPENROUTER_API_KEY`, `CLAUDE_GATEWAY_API_KEY`, model IDs, provider names, file paths, URLs, JSON fields, and issue codes.
- Keep RawBlock visual rules, but avoid relying on CSS `text-transform` as the only casing mechanism for Chinese text.

## Task List

### Task 1: Foundation Dictionary

**Description:** Add the locale registry and translation helper.

**Acceptance criteria:**
- [x] `en` and `zh-CN` dictionaries exist in one frontend module.
- [x] Missing keys fall back to English and are visible in tests.
- [x] Locale detection uses `localStorage` first, then `navigator.language`.

**Verification:**
- [x] `node --check frontend/i18n.js`
- [x] `npm test`

**Files likely touched:** `frontend/i18n.js`, `frontend/main.js`

### Task 2: Static Markup Translation

**Description:** Move headings, labels, table headers, buttons, and section aria labels out of raw HTML text.

**Acceptance criteria:**
- [x] `index.html` uses `data-i18n` / `data-i18n-attr` for user-facing copy.
- [x] English initial render remains usable before Wails data loads.
- [x] Chinese labels fit in existing RawBlock layout without overlap.

**Verification:**
- [x] Manual GUI check in English and Chinese.
- [x] `node --check frontend/main.js`

**Files likely touched:** `frontend/index.html`, `frontend/main.js`, `frontend/styles.css`

### Task 3: Dynamic Copy Translation

**Description:** Localize runtime strings produced by JavaScript.

**Acceptance criteria:**
- [x] Empty states, status labels, confirmations, validation errors, save/delete success messages, and timestamps use translation keys.
- [x] Plural route count renders correctly in both languages.
- [x] Technical values remain untranslated.

**Verification:**
- [x] Unit-style browserless test for `t()` interpolation and missing-key fallback.
- [ ] Manual route/key CRUD smoke check in both languages.

**Files likely touched:** `frontend/main.js`, `frontend/i18n.js`, optional `tests/`

### Task 4: Language Switcher

**Description:** Add a compact RawBlock language control in the top bar.

**Acceptance criteria:**
- [x] Users can switch between `EN` and `中文`.
- [x] Choice persists across app restarts through `localStorage`.
- [x] Switching language updates existing DOM without reloading or losing form data.

**Verification:**
- [x] Manual switch test in the running Wails app.
- [x] `GOCACHE=/private/tmp/go-build-cache /Users/c/go/bin/wails build`

**Files likely touched:** `frontend/index.html`, `frontend/main.js`, `frontend/styles.css`

### Task 5: Backend Message Boundary

**Description:** Decide which Wails/backend-originated messages should remain English diagnostics and which should become localizable frontend messages.

**Acceptance criteria:**
- [x] Backend errors keep stable English technical text for logs/tests.
- [x] Frontend wraps known validation failures with localized user guidance where practical.
- [x] No API response or config schema changes are introduced for i18n.

**Verification:**
- [x] `GOCACHE=/private/tmp/go-build-cache go test ./...`
- [x] Existing config/editor tests continue to pass unchanged.

**Files likely touched:** `frontend/main.js`, optional `internal/gui/` only if a typed error code boundary is needed.

## Checkpoints

### Checkpoint A: Foundation

- [x] Dictionary helper works.
- [x] Static UI can render both languages.
- [x] No layout regression in RawBlock desktop viewport.

### Checkpoint B: Complete GUI Pass

- [x] All visible GUI copy is localized except explicit technical identifiers.
- [x] Language switch persists.
- [x] Go tests, npm tests, TypeScript build, JS syntax check, and Wails build pass.

## Risks and Mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Chinese text overflows RawBlock labels | Medium | Use stable grid constraints and inspect Wails window manually. |
| Technical identifiers get translated accidentally | High | Keep a documented do-not-translate list and avoid translating values from backend data. |
| Backend errors become unstable if localized | Medium | Keep backend diagnostic strings stable; localize frontend guidance only. |
| Ad hoc translation keys sprawl | Medium | Use namespaced keys such as `nav.refresh`, `models.empty`, `editor.saved`. |

## Open Questions

- Should the default locale be system-following or always Chinese for this project?
- Should future CLI docs/errors be localized, or should i18n remain GUI-only?
