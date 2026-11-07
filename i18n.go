// Package i18n provides lightweight .po-based internationalization for Go programs.
//
// Usage:
//
//	import "github.com/nanjj/i18n"
//	import "embed"
//
//	//go:embed locales/*/*.po
//	var localesFS embed.FS
//
//	var loc = i18n.New(localesFS, i18n.WithPOFile("app.po"))
//
//	func G(msgid string) string { return loc.G(msgid) }
package i18n

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
)

// Locales is a translation engine backed by embedded .po files.
type Locales struct {
	fsys   fs.FS
	dir    string
	poFile string

	// sentinel is the locale where msgstr == msgid (e.g. "en_US").
	sentinel string

	translations map[string]map[string]string
	detectedLang string
}

// Option configures a Locales.
type Option func(*Locales)

// WithDir sets the directory within the FS to scan for locale subdirectories.
// Default: "locales".
func WithDir(dir string) Option {
	return func(l *Locales) {
		if dir != "" {
			l.dir = dir
		}
	}
}

// WithPOFile sets the .po filename to look for in each locale directory.
// Default: "messages.po".
func WithPOFile(name string) Option {
	return func(l *Locales) {
		if name != "" {
			l.poFile = name
		}
	}
}

// WithSentinel sets the sentinel locale (where msgstr equals msgid).
// The sentinel locale is used as fallback when no translation is found and
// as the default language when no environment variable is set.
// Default: "en_US".
func WithSentinel(locale string) Option {
	return func(l *Locales) {
		if locale != "" {
			l.sentinel = locale
		}
	}
}

// New creates a Locales from an fs.FS.
//
// The FS is expected to contain directories named by locale code
// (e.g. "en_US", "zh_CN", "zh"), each containing a .po file.
//
// Default directory: "locales"
// Default .po file:  "messages.po"
// Default sentinel:  "en_US"
//
// Example with //go:embed:
//
//	//go:embed locales/*/*.po
//	var localesFS embed.FS
//
//	loc := i18n.New(localesFS, i18n.WithPOFile("app.po"))
func New(fsys fs.FS, opts ...Option) *Locales {
	l := &Locales{
		fsys:         fsys,
		dir:          "locales",
		poFile:       "messages.po",
		sentinel:     "en_US",
		translations: make(map[string]map[string]string),
	}
	for _, opt := range opts {
		opt(l)
	}
	l.loadAllTranslations()
	l.detectedLang = l.detectLang()
	return l
}

// poLineRE matches msgid/msgstr lines in .po files.
var poLineRE = regexp.MustCompile(`^(msgid|msgstr)\s+"(.*)"\s*$`)

// loadAllTranslations loads all .po files from the embedded FS.
func (l *Locales) loadAllTranslations() {
	entries, err := fs.ReadDir(l.fsys, l.dir)
	if err != nil {
		// No embedded translation data.
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		lang := entry.Name()
		data, err := fs.ReadFile(l.fsys, l.dir+"/"+lang+"/"+l.poFile)
		if err != nil {
			continue
		}

		l.translations[lang] = parsePO(string(data))
	}
}

// detectLang detects the user's language from environment variables.
//
// Priority: LANGUAGE > LC_ALL > LANG
//
// Returns the sentinel locale if none is set or all are "C"/"POSIX".
func (l *Locales) detectLang() string {
	for _, env := range []string{"LANGUAGE", "LC_ALL", "LANG"} {
		val := os.Getenv(env)
		if val == "" || val == "C" || val == "POSIX" {
			continue
		}

		// LANGUAGE may contain colon-separated list; take first.
		lang, _, _ := strings.Cut(val, ":")

		// Strip .UTF-8, .utf8, etc.
		if idx := strings.IndexByte(lang, '.'); idx >= 0 {
			lang = lang[:idx]
		}

		if lang != "" {
			return lang
		}
	}
	return l.sentinel
}

// parsePO parses .po file content into a msgid→msgstr map.
func parsePO(data string) map[string]string {
	table := make(map[string]string)

	var currentID string
	var currentStr string
	var inID bool
	var inStr bool

	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			// Save the previous pair.
			if currentID != "" {
				table[currentID] = currentStr
				currentID = ""
				currentStr = ""
			}
			inID = false
			inStr = false
			continue
		}

		if matches := poLineRE.FindStringSubmatch(trimmed); len(matches) == 3 {
			key := matches[1]
			val := unescapePO(matches[2])

			switch key {
			case "msgid":
				// Save the previous pair.
				if currentID != "" {
					table[currentID] = currentStr
				}
				currentID = val
				currentStr = ""
				inID = true
				inStr = false
			case "msgstr":
				currentStr = val
				inID = false
				inStr = true
			}
		} else if inID {
			// Multi-line msgid (uncommon but supported).
			cont := strings.TrimSpace(line)
			cont = strings.Trim(cont, `"`)
			cont = unescapePO(cont)
			currentID += cont
		} else if inStr {
			// Multi-line msgstr.
			cont := strings.TrimSpace(line)
			cont = strings.Trim(cont, `"`)
			cont = unescapePO(cont)
			currentStr += cont
		}
	}

	// Save the last pair.
	if currentID != "" {
		table[currentID] = currentStr
	}

	return table
}

// unescapePO handles C-style escape sequences in PO strings.
//
// Must handle \\ before \" and \n to avoid double-unescaping:
//
//	\\  →  literal backslash
//	\"  →  literal double-quote
//	\n  →  newline
func unescapePO(s string) string {
	s = strings.ReplaceAll(s, `\\`, "\x00")
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, "\x00", `\`)
	return s
}

// G returns the translation of msgid for the current locale.
//
// Lookup order matches gettext: the current locale, then a shorter language
// code (zh_CN → zh), then the msgid itself. A msgid that is absent from the
// table, or present with an empty msgstr, is untranslated and yields the
// msgid unchanged. Missing entries never panic: a runtime that dies because a
// .po file lags one commit behind the source is worse than an untranslated
// string, and gettext has always behaved the other way.
//
// Use "slingshot i18n verify" to catch missing entries at build time, where
// a failure costs a CI run rather than a user's session.
func (l *Locales) G(msgid string) string {
	if translated, ok := l.lookup(l.detectedLang, msgid); ok {
		return translated
	}

	// Try falling back to a shorter language code (e.g. "zh" from "zh_CN").
	if idx := strings.IndexByte(l.detectedLang, '_'); idx >= 0 {
		if translated, ok := l.lookup(l.detectedLang[:idx], msgid); ok {
			return translated
		}
	}

	return msgid
}

// lookup returns the non-empty translation of msgid in locale, if any.
func (l *Locales) lookup(locale, msgid string) (string, bool) {
	table, ok := l.translations[locale]
	if !ok {
		return "", false
	}

	translated, ok := table[msgid]
	if !ok || translated == "" {
		return "", false
	}

	return translated, true
}

// SetLocale forces a specific locale. Useful for testing.
func (l *Locales) SetLocale(lang string) {
	l.detectedLang = lang
}

// CurrentLocale returns the currently active locale code.
func (l *Locales) CurrentLocale() string {
	return l.detectedLang
}

// Dump returns a human-readable summary of loaded translations.
func (l *Locales) Dump() string {
	var b strings.Builder
	fmt.Fprintf(&b, "detected lang: %s\n", l.detectedLang)
	for lang, table := range l.translations {
		fmt.Fprintf(&b, "  %s: %d entries\n", lang, len(table))
	}
	return b.String()
}
