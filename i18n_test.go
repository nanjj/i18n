package i18n

import (
	"os"
	"testing"
	"testing/fstest"
)

func TestNew_LoadsTranslations(t *testing.T) {
	loc := New(os.DirFS("testdata"))

	// Must have loaded some locales.
	if len(loc.translations) == 0 {
		t.Fatal("expected translations to be loaded")
	}

	// en_US should have 3 entries (Hello, World, Goodbye).
	if n := len(loc.translations["en_US"]); n != 3 {
		t.Fatalf("en_US: expected 3 entries, got %d", n)
	}

	// zh_CN should have 2 entries.
	if n := len(loc.translations["zh_CN"]); n != 2 {
		t.Fatalf("zh_CN: expected 2 entries, got %d", n)
	}
}

func TestG_ReturnsTranslation(t *testing.T) {
	loc := New(os.DirFS("testdata"))
	loc.SetLocale("zh_CN")

	if got := loc.G("Hello"); got != "你好" {
		t.Fatalf("G('Hello') = %q, want '你好'", got)
	}
}

func TestG_FallbacksToMsgid(t *testing.T) {
	loc := New(os.DirFS("testdata"))
	loc.SetLocale("de_DE")

	// "World" is translated: "Welt"
	if got := loc.G("World"); got != "Welt" {
		t.Fatalf("G('World') = %q, want 'Welt'", got)
	}

	// "Hello" has empty msgstr → untranslated, fallback to msgid.
	if got := loc.G("Hello"); got != "Hello" {
		t.Fatalf("G('Hello') = %q, want 'Hello' (fallback)", got)
	}
}

func TestG_MissingKeyPanics(t *testing.T) {
	loc := New(os.DirFS("testdata"))
	loc.SetLocale("zh_CN")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for missing translation")
		}
	}()

	loc.G("Goodbye") // zh_CN has no "Goodbye" entry
}

func TestG_NoPOFileForLocale(t *testing.T) {
	loc := New(os.DirFS("testdata"))
	loc.SetLocale("fr_FR") // no .po file for this locale

	if got := loc.G("Hello"); got != "Hello" {
		t.Fatalf("G('Hello') = %q, want 'Hello' (fallback to msgid)", got)
	}
}

func TestG_ShortCodeFallback(t *testing.T) {
	// zh_CN detected but no zh_CN.po; fallback to zh.po short code.
	fsys := fstest.MapFS{
		"locales/zh/messages.po": &fstest.MapFile{
			Data: []byte("msgid \"Hello\"\nmsgstr \"你好\"\n"),
		},
	}

	loc := New(fsys)
	loc.SetLocale("zh_CN") // zh_CN not loaded, but "zh" is

	if got := loc.G("Hello"); got != "你好" {
		t.Fatalf("G('Hello') = %q, want '你好' (from zh fallback)", got)
	}
}

func TestDetectLang_NoEnv(t *testing.T) {
	// Save and clear locale env vars.
	type pair struct{ k, v string }
	var saved []pair
	for _, env := range []string{"LANGUAGE", "LC_ALL", "LANG"} {
		saved = append(saved, pair{env, os.Getenv(env)})
		os.Unsetenv(env)
	}
	defer func() {
		for _, p := range saved {
			os.Setenv(p.k, p.v)
		}
	}()

	loc := New(os.DirFS("testdata"))
	if got := loc.CurrentLocale(); got != "en_US" {
		t.Fatalf("CurrentLocale() = %q, want 'en_US' (sentinel)", got)
	}
}

func TestSetLocale(t *testing.T) {
	loc := New(os.DirFS("testdata"))
	loc.SetLocale("de_DE")

	if got := loc.CurrentLocale(); got != "de_DE" {
		t.Fatalf("CurrentLocale() = %q, want 'de_DE'", got)
	}
}

func TestDump(t *testing.T) {
	loc := New(os.DirFS("testdata"))
	dump := loc.Dump()

	if dump == "" {
		t.Fatal("Dump() returned empty string")
	}
	t.Logf("Dump:\n%s", dump)
}

func TestCustomDir(t *testing.T) {
	// Use a custom directory structure where locale dirs are under "translations/".
	fsys := fstest.MapFS{
		"translations/en_US/custom.po": &fstest.MapFile{
			Data: []byte("msgid \"yes\"\nmsgstr \"yes\"\n"),
		},
		"translations/zh_CN/custom.po": &fstest.MapFile{
			Data: []byte("msgid \"yes\"\nmsgstr \"是\"\n"),
		},
	}

	loc := New(fsys, WithDir("translations"), WithPOFile("custom.po"))
	loc.SetLocale("zh_CN")

	if got := loc.G("yes"); got != "是" {
		t.Fatalf("G('yes') = %q, want '是'", got)
	}
}

func TestEmptyFS(t *testing.T) {
	// An empty FS should not panic.
	fsys := fstest.MapFS{}
	loc := New(fsys)

	loc.SetLocale("en_US")
	if got := loc.G("anything"); got != "anything" {
		t.Fatalf("G('anything') = %q, want 'anything' (fallback)", got)
	}
}

func TestParsePO_EscapeSequences(t *testing.T) {
	data := `msgid "hello\nworld"
msgstr "你好\n世界"
`
	table := parsePO(data)
	if got := table["hello\nworld"]; got != "你好\n世界" {
		t.Fatalf("escaped newline: got %q, want %q", got, "你好\n世界")
	}
}

func TestParsePO_QuotedString(t *testing.T) {
	data := `msgid "He said \"Hi\""
msgstr "他说\"你好\""
`
	table := parsePO(data)
	if got := table[`He said "Hi"`]; got != `他说"你好"` {
		t.Fatalf("escaped quotes: got %q, want %q", got, `他说"你好"`)
	}
}

func TestParsePO_Empty(t *testing.T) {
	table := parsePO("")
	if len(table) != 0 {
		t.Fatalf("expected empty table, got %d entries", len(table))
	}
}

func TestParsePO_HeaderOnly(t *testing.T) {
	data := `msgid ""
msgstr "Project-Id-Version: test\n"
`
	table := parsePO(data)
	if len(table) != 0 {
		t.Fatalf("expected empty table (header only), got %d entries", len(table))
	}
}

func TestCustomSentinel(t *testing.T) {
	// Save and clear locale env vars.
	type pair struct{ k, v string }
	var saved []pair
	for _, env := range []string{"LANGUAGE", "LC_ALL", "LANG"} {
		saved = append(saved, pair{env, os.Getenv(env)})
		os.Unsetenv(env)
	}
	defer func() {
		for _, p := range saved {
			os.Setenv(p.k, p.v)
		}
	}()

	fsys := fstest.MapFS{
		"locales/ja_JP/messages.po": &fstest.MapFile{
			Data: []byte("msgid \"hello\"\nmsgstr \"こんにちは\"\n"),
		},
	}

	loc := New(fsys, WithSentinel("ja_JP"))
	if got := loc.CurrentLocale(); got != "ja_JP" {
		t.Fatalf("CurrentLocale() = %q, want 'ja_JP' (custom sentinel)", got)
	}
}
