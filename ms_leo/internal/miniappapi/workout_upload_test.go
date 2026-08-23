package miniappapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSniffImageExt(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
		ok   bool
	}{
		{name: "jpeg", in: []byte{0xff, 0xd8, 0xff, 0xe0}, want: ".jpg", ok: true},
		{name: "png", in: []byte("\x89PNG\r\n\x1a\npayload"), want: ".png", ok: true},
		{name: "webp", in: []byte("RIFFxxxxWEBPpayload"), want: ".webp", ok: true},
		{name: "gif87", in: []byte("GIF87apayload"), want: ".gif", ok: true},
		{name: "gif89", in: []byte("GIF89apayload"), want: ".gif", ok: true},
		{name: "text", in: []byte("not an image"), ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := sniffImageExt(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("sniffImageExt() = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestSaveWorkoutPhotoFilePreservesBytes(t *testing.T) {
	dir := t.TempDir()
	original := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x42}, 128)...)

	baseName, errCode := saveWorkoutPhotoFile(dir, bytes.NewReader(original))
	if errCode != "" {
		t.Fatalf("saveWorkoutPhotoFile errCode = %q", errCode)
	}
	if !strings.HasSuffix(baseName, ".png") {
		t.Fatalf("baseName = %q, want .png suffix", baseName)
	}

	saved, err := os.ReadFile(filepath.Join(dir, baseName))
	if err != nil {
		t.Fatalf("read saved photo: %v", err)
	}
	if !bytes.Equal(saved, original) {
		t.Fatalf("saved photo bytes changed: got %d bytes, want %d", len(saved), len(original))
	}
}

func TestSaveWorkoutPhotoFileRejectsUnsupportedAndTooLarge(t *testing.T) {
	t.Run("unsupported", func(t *testing.T) {
		dir := t.TempDir()
		baseName, errCode := saveWorkoutPhotoFile(dir, bytes.NewReader([]byte("plain text")))
		if errCode != "unsupported_image" || baseName != "" {
			t.Fatalf("saveWorkoutPhotoFile() = %q, %q; want unsupported_image", baseName, errCode)
		}
	})

	t.Run("too large", func(t *testing.T) {
		dir := t.TempDir()
		tooLarge := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0x42}, maxWorkoutPhotoBytes+1)...)
		baseName, errCode := saveWorkoutPhotoFile(dir, bytes.NewReader(tooLarge))
		if errCode != "photo_too_large" || baseName != "" {
			t.Fatalf("saveWorkoutPhotoFile() = %q, %q; want photo_too_large", baseName, errCode)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read temp dir: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("too-large upload left %d file(s) behind", len(entries))
		}
	})
}

func TestAbsolutePublicBaseFromRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://internal.local/api/miniapp/workout", nil)
	req.Host = "internal.local"
	req.Header.Set("X-Forwarded-Host", "leopard.example")
	req.Header.Set("X-Forwarded-Proto", "https")

	got := absolutePublicBaseFromRequest(req)
	if got != "https://leopard.example" {
		t.Fatalf("absolutePublicBaseFromRequest() = %q", got)
	}
}

func TestHandleGetMiniappMediaServesRootFileOnly(t *testing.T) {
	dir := t.TempDir()
	body := []byte("\x89PNG\r\n\x1a\nok")
	if err := os.WriteFile(filepath.Join(dir, "ok.png"), body, 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "secret.png"), []byte("secret"), 0600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}

	s := &Server{mediaDirAbsolute: dir}
	okReq := httptest.NewRequest(http.MethodGet, "/api/miniapp/media/ok.png", nil)
	okResp := httptest.NewRecorder()
	s.handleGetMiniappMedia(okResp, okReq)
	if okResp.Code != http.StatusOK {
		t.Fatalf("media status = %d, want 200", okResp.Code)
	}
	if !bytes.Equal(okResp.Body.Bytes(), body) {
		t.Fatalf("media body = %q, want %q", okResp.Body.Bytes(), body)
	}

	badReq := httptest.NewRequest(http.MethodGet, "/api/miniapp/media/../secret.png", nil)
	badResp := httptest.NewRecorder()
	s.handleGetMiniappMedia(badResp, badReq)
	if badResp.Code != http.StatusNotFound {
		t.Fatalf("traversal status = %d, want 404", badResp.Code)
	}
}
