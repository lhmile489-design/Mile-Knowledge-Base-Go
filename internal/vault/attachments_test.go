package vault

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pngBytes() []byte {
	return append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, bytes.Repeat([]byte{0x00}, 32)...)
}

func TestSniffImageFormats(t *testing.T) {
	cases := []struct {
		name string
		body []byte
		ext  string
		ok   bool
	}{
		{"png", pngBytes(), "png", true},
		{"jpg", append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0}, 8)...), "jpg", true},
		{"gif", []byte("GIF89a......"), "gif", true},
		{"webp", append(append([]byte("RIFF"), []byte{0, 0, 0, 0}...), []byte("WEBPVP8 ")...), "webp", true},
		{"bmp", []byte("BMxxxxxx"), "bmp", true},
		{"avif", append(append([]byte{0, 0, 0, 0x1C}, []byte("ftypavif")...), []byte{0, 0, 0, 0}...), "avif", true},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), "", false},
		{"ico", []byte{0, 0, 1, 0, 1, 0}, "", false},
		{"empty", nil, "", false},
	}
	for _, c := range cases {
		ext, mime, ok := SniffImage(c.body)
		if ok != c.ok || ext != c.ext {
			t.Errorf("%s: ext=%q mime=%q ok=%v", c.name, ext, mime, ok)
		}
		if ok && mime == "" {
			t.Errorf("%s: missing mime", c.name)
		}
	}
}

func TestSaveAttachmentDedupesByName(t *testing.T) {
	root := t.TempDir()
	body := pngBytes()
	rel, existed, f := SaveAttachment(root, body)
	if f != nil {
		t.Fatal(f)
	}
	if !strings.HasPrefix(rel, AttachmentDir+"/pasted-") || !strings.HasSuffix(rel, ".png") {
		t.Errorf("rel = %s", rel)
	}
	if existed {
		t.Error("first save reported existed")
	}
	again, existed, f := SaveAttachment(root, body)
	if f != nil || again != rel || !existed {
		t.Fatalf("second save: %s %v %+v", again, existed, f)
	}
	entries, _ := os.ReadDir(filepath.Join(root, "raws", "attachments"))
	if len(entries) != 1 {
		t.Errorf("entries = %d", len(entries))
	}
}

func TestSaveAttachmentRejects(t *testing.T) {
	root := t.TempDir()
	if _, _, f := SaveAttachment(root, nil); f == nil {
		t.Error("empty accepted")
	}
	if _, _, f := SaveAttachment(root, []byte("not an image")); f == nil {
		t.Error("non-image accepted")
	}
	big := make([]byte, MaxAttachmentBytes+1)
	copy(big, pngBytes())
	if _, _, f := SaveAttachment(root, big); f == nil {
		t.Error("oversize accepted")
	}
}
