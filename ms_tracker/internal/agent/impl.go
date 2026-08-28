package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type fileEdit struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type implReply struct {
	Note  string     `json:"note"`
	Files []fileEdit `json:"files"`
}

type fileSnippet struct {
	Path    string
	Content string
}

var implRoots = []string{"miniapp/", "ms_leo/", "ms_tracker/"}

// contextBudget — сколько символов исходников влезает в один промпт агента.
const contextBudget = 200_000

func filesHaveImpl(names []string) bool {
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || strings.HasPrefix(name, ".tracker/") {
			continue
		}
		return true
	}
	return false
}

func isAllowedImplPath(p string) bool {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if p == "" || strings.Contains(p, "..") || strings.HasPrefix(p, "/") || strings.HasPrefix(p, ".tracker/") {
		return false
	}
	for _, root := range implRoots {
		if strings.HasPrefix(p, root) {
			return true
		}
	}
	return false
}

func parseImplReply(raw string) implReply {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			var out implReply
			if json.Unmarshal([]byte(raw[i:j+1]), &out) == nil {
				out.Note = strings.TrimSpace(out.Note)
				if out.Note == "" {
					out.Note = "Правки в репозитории."
				}
				return out
			}
		}
	}
	note := clip(raw, 3500)
	if note == "" {
		note = "Агент не вернул правки файлов."
	}
	return implReply{Note: note}
}

// shrinkFloor — какую долю файла правка обязана сохранить. Модель отдаёт
// «полный новый текст файла»; если вернулся огрызок, это не правка, а потеря
// кода: на #13 так вырезало 1456 строк ProfileScreen.tsx, на #20 — 1006 строк
// index.css, и оба раза ленивое ревью пропустило это в main.
const shrinkFloor = 0.6

func applyFileEdits(repoDir string, edits []fileEdit) (int, []string, error) {
	n := 0
	var rejected []string
	for _, edit := range edits {
		path := filepath.ToSlash(strings.TrimSpace(edit.Path))
		if !isAllowedImplPath(path) || strings.TrimSpace(edit.Content) == "" {
			continue
		}
		if len(edit.Content) > 400_000 {
			continue
		}
		full := filepath.Join(repoDir, filepath.FromSlash(path))
		if reason := shrinkReason(full, edit.Content); reason != "" {
			rejected = append(rejected, path+": "+reason)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return n, rejected, err
		}
		if err := os.WriteFile(full, []byte(edit.Content), 0o644); err != nil {
			return n, rejected, err
		}
		n++
	}
	return n, rejected, nil
}

// shrinkReason — почему правку нельзя писать поверх существующего файла.
// Пустая строка = писать можно (новый файл или правка нормального размера).
func shrinkReason(full, content string) string {
	prev, err := os.ReadFile(full)
	if err != nil || len(prev) == 0 {
		return ""
	}
	if len(content) >= int(float64(len(prev))*shrinkFloor) {
		return ""
	}
	was := strings.Count(string(prev), "\n") + 1
	now := strings.Count(content, "\n") + 1
	return fmt.Sprintf("агент вернул %d строк вместо %d — похоже на обрезанный файл, правка отклонена", now, was)
}

func collectContextFiles(repoDir, prompt string) []fileSnippet {
	words := implKeywords(prompt)
	if len(words) == 0 {
		return nil
	}
	type scored struct {
		fileSnippet
		score int
	}
	var found []scored
	for _, root := range implRoots {
		_ = filepath.Walk(filepath.Join(repoDir, filepath.FromSlash(strings.TrimSuffix(root, "/"))), func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(repoDir, path)
			if err != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if !isAllowedImplPath(rel) || !implSourceFile(rel) || info.Size() > 120_000 {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			content := string(raw)
			score := implFileScore(rel, content, words)
			if score <= 0 {
				return nil
			}
			found = append(found, scored{fileSnippet{Path: rel, Content: content}, score})
			return nil
		})
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].score == found[j].score {
			return found[i].Path < found[j].Path
		}
		return found[i].score > found[j].score
	})
	// Файл идёт в промпт целиком или не идёт вовсе. Раньше большие файлы
	// обрезались на 18k рун, а промпт при этом требовал «полный новый текст
	// файла» — модель дописывала огрызок, и он затирал оригинал.
	out := make([]fileSnippet, 0, 4)
	budget := contextBudget
	for _, item := range found {
		if len(out) >= 4 {
			break
		}
		if len(item.Content) > budget {
			continue
		}
		budget -= len(item.Content)
		out = append(out, item.fileSnippet)
	}
	return out
}

func implSourceFile(path string) bool {
	low := strings.ToLower(path)
	if strings.Contains(low, "_test.") || strings.Contains(low, "/vendor/") || strings.Contains(low, "/node_modules/") {
		return false
	}
	switch filepath.Ext(low) {
	case ".tsx", ".ts", ".css", ".go", ".js":
		return true
	default:
		return false
	}
}

func implKeywords(prompt string) []string {
	skip := map[string]bool{
		"задача": true, "сделать": true, "сделай": true, "убери": true, "убрать": true,
		"кнопку": true, "кнопка": true, "текст": true, "нужно": true, "чтобы": true,
		"этот": true, "этой": true, "просто": true, "можно": true, "надо": true,
		"после": true, "перед": true, "под": true, "над": true, "или": true,
	}
	seen := map[string]bool{}
	var out []string
	var buf strings.Builder
	flush := func() {
		w := strings.ToLower(strings.TrimSpace(buf.String()))
		buf.Reset()
		if len([]rune(w)) < 4 || skip[w] || seen[w] {
			return
		}
		seen[w] = true
		out = append(out, w)
	}
	for _, r := range strings.ToLower(prompt) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func implFileScore(path, content string, words []string) int {
	lowPath := strings.ToLower(path)
	low := strings.ToLower(content)
	score := 0
	for _, w := range words {
		if strings.Contains(lowPath, w) {
			score += 3
		}
		if strings.Contains(low, w) {
			score++
		}
	}
	return score
}

func ShipBlockReason(info branchInfo, err error) string {
	if err != nil {
		return err.Error()
	}
	if !info.Exists {
		return "нет ветки задачи"
	}
	return ""
}
