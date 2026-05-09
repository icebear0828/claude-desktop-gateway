# RawBlock GUI Direction

RawBlock is the required visual direction for the desktop GUI. Keep the console
structural, high-contrast, and intentionally plain.

## Rules

- Use black `#000000` and white `#ffffff` as the dominant palette.
- Use blue `#0000ff` only for links.
- Use green, orange, and red only for explicit success, warning, and error states.
- Use thick square borders, usually `3px` or `5px`.
- Do not use border radius, shadows, gradients, decorative images, or icons.
- Use uppercase labels and buttons with tracked text.
- Prefer tables, lists, and bordered sections over decorative cards.
- Use monospace text for paths, URLs, model IDs, and diagnostics.

## Typography

Use these font families when available, with local fallbacks:

- Headline: `Archivo Black`
- Body: `Work Sans`
- Mono: `Space Mono`

## Current GUI Scope

The MVP dashboard shows gateway health, listen URL, config file state,
providers, route aliases, and Claude Desktop doctor output. Route editing,
provider testing, and start/stop controls remain follow-up tasks.
