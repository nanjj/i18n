# i18n — Lightweight .po-based Internationalization for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/nanjj/i18n.svg)](https://pkg.go.dev/github.com/nanjj/i18n)
[![Go Version](https://img.shields.io/github/go-mod/go-version/nanjj/i18n)](https://golang.org/dl/)
[![License](https://img.shields.io/github/license/nanjj/i18n)](LICENSE)

**i18n** is a minimal, zero-dependency internationalization library for Go
that uses standard **GNU gettext `.po` files** as its translation source.
Designed for embedded use via `//go:embed`, it fits naturally into Go's
toolchain without external tools or runtime code generation.

## Features

- **Pure Go** — no CGO, no external dependencies
- **Embed-friendly** — works with `//go:embed` out of the box
- **Standard .po format** — use Poedit, Lokalize, or any gettext toolchain
- **Locale auto-detection** — respects `LANGUAGE`, `LC_ALL`, `LANG`
- **Sensible fallbacks** — sentinel locale, short code fallback, msgid fallback
- **Panic on missing entries** — catches incomplete translations at runtime
- **Multi-line msgid/msgstr support** — handles escaped strings and multiline
- **Fully tested** — comprehensive unit test coverage

## Installation

```bash
go get github.com/nanjj/i18n
```

Requirements: **Go 1.26** or later.

## Quick Start

### 1. Prepare locale files

Create a `locales/` directory with subdirectories named by locale code,
each containing a `.po` file:

```
locales/
├── en_US/
│   └── messages.po
├── zh_CN/
│   └── messages.po
└── de_DE/
    └── messages.po
```

Example `locales/zh_CN/messages.po`:

```po
msgid "Hello"
msgstr "你好"

msgid "World"
msgstr "世界"
```

### 2. Embed and use in code

```go
package main

import (
    "embed"

    "github.com/nanjj/i18n"
)

//go:embed locales/*/*.po
var localesFS embed.FS

var loc = i18n.New(localesFS)

func G(msgid string) string { return loc.G(msgid) }

func main() {
    // Language is auto-detected from environment.
    // You can also set it explicitly:
    loc.SetLocale("zh_CN")

    println(G("Hello")) // 你好
    println(G("World")) // 世界
}
```

## API

### `New(fsys fs.FS, opts ...Option) *Locales`

Creates a new translation engine from an `fs.FS`. The filesystem is expected
to contain locale subdirectories, each with a `.po` file.

```go
loc := i18n.New(embeddedFS)
// or with options:
loc := i18n.New(embeddedFS,
    i18n.WithDir("translations"),
    i18n.WithPOFile("app.po"),
    i18n.WithSentinel("zh_CN"),
)
```

### `G(msgid string) string`

Returns the translation of `msgid` for the current locale.

- If the current locale has the key but the translation is empty → falls back
  to `msgid` (treated as untranslated).
- If the current locale has a `.po` file but the key is missing → **panics**,
  alerting the developer to add the missing entry.
- If the current locale has no `.po` file → falls back to `msgid`.
- If the detected locale contains a region code (e.g. `zh_CN`) and no exact
  match is found, falls back to the short code `zh` if available.

### `SetLocale(lang string)`

Forces a specific locale. Useful for testing or user-preference overrides.

```go
loc.SetLocale("de_DE")
```

### `CurrentLocale() string`

Returns the currently active locale code.

### `Dump() string`

Returns a human-readable summary of all loaded translations, useful for
debugging.

## Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithDir(dir)` | Directory within the FS to scan for locale subdirectories | `"locales"` |
| `WithPOFile(name)` | `.po` filename to look for in each locale directory | `"messages.po"` |
| `WithSentinel(locale)` | Sentinel locale used as fallback and default language | `"en_US"` |

## Locale Detection

Priority order:

1. `LANGUAGE` environment variable (colon-separated list; first value used)
2. `LC_ALL` environment variable
3. `LANG` environment variable
4. Sentinel locale (default: `"en_US"`)

Values `"C"` and `"POSIX"` are treated as unset. Encoding suffixes
(e.g. `.UTF-8`, `.utf8`) are automatically stripped.

## Fallback Behavior

When `G()` is called, the library tries in order:

1. **Exact locale match** — look up the key in the detected locale's table
   - Key found with non-empty translation → return it
   - Key found with empty translation → return `msgid` (untranslated)
   - Key missing → **panic** (developer forgot to add the entry)
2. **Short code fallback** — if locale contains `_` (e.g. `zh_CN`), try `zh`
   - Non-empty translation found → return it
   - Otherwise → return `msgid`
3. **No .po file** for this locale → return `msgid`

> **Design rationale**: The panic on missing entries is intentional. In a
> well-maintained i18n project, every `G()` call should have a corresponding
> entry. The panic catches omissions immediately rather than silently returning
> the English string, which could go unnoticed through QA.

## PO File Format

Standard GNU gettext `.po` format is supported:

```po
# Comments are ignored
msgid "Hello"
msgstr "你好"

# Multi-line strings are supported
msgid "hello\nworld"
msgstr "你好\n世界"

# Escaped quotes
msgid "He said \"Hi\""
msgstr "他说\"你好\""

# Empty msgid/msgstr (header block) is ignored
msgid ""
msgstr "Project-Id-Version: 1.0\n"
```

## Testing

Run the test suite:

```bash
go test ./... -v
```

The project includes test data with three locales:

| Locale | Purpose |
|--------|---------|
| `en_US` | Sentinel locale — all 3 entries translated |
| `zh_CN` | Real translations — 2 entries |
| `de_DE` | Mixed — 1 translated, 1 intentionally empty (untranslated) |

## License

Copyright © 2026 JUN JIE NAN <nanjunjie@gmail.com>

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file
for details.
