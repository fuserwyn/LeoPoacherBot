package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesHaveImpl(t *testing.T) {
	if filesHaveImpl([]string{".tracker/job-12.md"}) {
		t.Fatal("note is not impl")
	}
	if filesHaveImpl(nil) || filesHaveImpl([]string{"", ".tracker/job-1.md"}) {
		t.Fatal("empty")
	}
	if !filesHaveImpl([]string{".tracker/job-11.md", "miniapp/src/components/ProfileScreen.tsx"}) {
		t.Fatal("app file is impl")
	}
}

func TestIsAllowedImplPath(t *testing.T) {
	if !isAllowedImplPath("miniapp/src/App.tsx") || !isAllowedImplPath("ms_leo/internal/bot/x.go") {
		t.Fatal("allowed")
	}
	if isAllowedImplPath(".tracker/job-1.md") || isAllowedImplPath("../secret") || isAllowedImplPath("/etc/passwd") {
		t.Fatal("blocked")
	}
}

func TestParseImplReply(t *testing.T) {
	got := parseImplReply("```json\n{\"note\":\"убрал заголовок\",\"files\":[{\"path\":\"miniapp/src/a.tsx\",\"content\":\"x\"}]}\n```")
	if got.Note != "убрал заголовок" || len(got.Files) != 1 || got.Files[0].Path != "miniapp/src/a.tsx" {
		t.Fatalf("%+v", got)
	}
	plan := parseImplReply("Сначала найду кнопку, потом поправлю CSS.")
	if plan.Note == "" || len(plan.Files) != 0 {
		t.Fatalf("plan: %+v", plan)
	}
}

func TestApplyFileEdits(t *testing.T) {
	dir := t.TempDir()
	n, rejected, err := applyFileEdits(dir, []fileEdit{
		{Path: "miniapp/src/Hi.tsx", Content: "export const Hi = 1;\n"},
		{Path: ".tracker/job-1.md", Content: "nope"},
		{Path: "../escape", Content: "nope"},
	})
	if err != nil || n != 1 || len(rejected) != 0 {
		t.Fatalf("n=%d rejected=%v err=%v", n, rejected, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "miniapp", "src", "Hi.tsx"))
	if err != nil || string(raw) != "export const Hi = 1;\n" {
		t.Fatalf("file: %s %v", raw, err)
	}
}

func TestShipBlockReason(t *testing.T) {
	if got := ShipBlockReason(branchInfo{}, nil); got != "нет ветки задачи" {
		t.Fatalf("missing: %q", got)
	}
	if got := ShipBlockReason(branchInfo{Exists: true, HasImpl: false}, nil); got != "" {
		t.Fatalf("admin branch must ship: %q", got)
	}
	if got := ShipBlockReason(branchInfo{Exists: true, HasImpl: true}, nil); got != "" {
		t.Fatalf("ok: %q", got)
	}
}

func TestCollectContextFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "miniapp", "src", "components", "ProfileScreen.tsx")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("export function Profile(){ return <button>Взять больничный</button> }"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := collectContextFiles(dir, "кнопку взять больничный сделай праймари")
	if len(got) != 1 || got[0].Path != "miniapp/src/components/ProfileScreen.tsx" {
		t.Fatalf("%+v", got)
	}
}

// Огрызок вместо полного файла не должен затирать оригинал: так трекер
// вырезал ProfileScreen.tsx (#13) и index.css (#20).
func TestApplyFileEditsRejectsTruncatedRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "miniapp", "src", "index.css")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	full := strings.Repeat("a{color:red}\n", 500)
	if err := os.WriteFile(path, []byte(full), 0o644); err != nil {
		t.Fatal(err)
	}

	n, rejected, err := applyFileEdits(dir, []fileEdit{
		{Path: "miniapp/src/index.css", Content: "a{color:red}\n"},
	})
	if err != nil || n != 0 || len(rejected) != 1 {
		t.Fatalf("n=%d rejected=%v err=%v", n, rejected, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != full {
		t.Fatal("оригинал затёрт")
	}

	// Правка сопоставимого размера проходит.
	edited := strings.Repeat("a{color:blue}\n", 500)
	n, rejected, err = applyFileEdits(dir, []fileEdit{
		{Path: "miniapp/src/index.css", Content: edited},
	})
	if err != nil || n != 1 || len(rejected) != 0 {
		t.Fatalf("n=%d rejected=%v err=%v", n, rejected, err)
	}
}

// Большой файл идёт в промпт целиком либо не идёт: обрезанный контекст
// и требование «верни полный файл» — и были причиной вырезаний.
func TestCollectContextFilesNeverTruncates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "miniapp", "src", "index.css")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "/* таббар */\n" + strings.Repeat(".bottom-nav{color:red}\n", 2000)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := collectContextFiles(dir, "поправить таббар")
	if len(got) != 1 {
		t.Fatalf("файлов: %d", len(got))
	}
	if got[0].Content != body {
		t.Fatalf("контекст обрезан: %d из %d", len(got[0].Content), len(body))
	}
}
