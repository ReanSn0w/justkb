package frontend

import (
	"bytes"
	"encoding/json"
	"image/png"
	"io/fs"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrandingAssets(t *testing.T) {
	for _, bg := range []string{DefaultIconColor, "#FFFFFF", "#111111"} {
		t.Run(bg, func(t *testing.T) {
			files, head := brandingAssets(Branding{Name: `Name "<script>"`, Color: bg})
			if strings.Contains(head, "<script>") || !strings.Contains(head, "&lt;script&gt;") {
				t.Fatal("unescaped metadata")
			}
			var manifest struct {
				Name  string
				ID    string
				Icons []struct{ Src string }
			}
			if err := json.Unmarshal(files["/manifest.webmanifest"], &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.Name != `Name "<script>"` || manifest.ID != "/" || len(manifest.Icons) != 3 {
				t.Fatalf("manifest: %+v", manifest)
			}
			img, err := png.Decode(bytes.NewReader(files["/icon-512.png"]))
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Dx() != 512 {
				t.Fatal("size")
			}
			for _, point := range [][2]int{{0, 0}, {164, 120}} {
				r, g, b, a := img.At(point[0], point[1]).RGBA()
				want := iconColor(bg)
				if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B || a != 65535 {
					t.Fatal("background or hollow header mismatch")
				}
			}
			if !bytes.Contains(files["/favicon.svg"], []byte(`fill="none"`)) {
				t.Fatal("missing outlined headers")
			}
		})
	}
	a, _ := brandingAssets(Branding{Name: "One"})
	b, _ := brandingAssets(Branding{Name: "One", Color: "#FF0000"})
	if bytes.Equal(a["/manifest.webmanifest"], b["/manifest.webmanifest"]) {
		t.Fatal("branding fingerprint unchanged")
	}
}
func TestBrandingHTTP(t *testing.T) {
	h := Handler(Branding{Name: `A < B`, Color: "#FF0000"})
	for _, test := range []struct{ path, content string }{{"/manifest.webmanifest", "application/manifest+json"}, {"/favicon.svg", "image/svg+xml"}, {"/apple-touch-icon.png", "image/png"}, {"/favicon.ico", "image/x-icon"}} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", test.path, nil))
		if r.Code != 200 || r.Header().Get("Content-Type") != test.content {
			t.Fatalf("%s: %d %s", test.path, r.Code, r.Header().Get("Content-Type"))
		}
	}
	if _, err := fs.Stat(assets, "dist/index.html"); err == nil {
		for _, path := range []string{"/cards/example", "/index.html", "/"} {
			r := httptest.NewRecorder()
			h.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
			if !strings.Contains(r.Body.String(), "<title>A &lt; B</title>") {
				t.Fatalf("runtime title missing at %s", path)
			}
		}
	}
}
