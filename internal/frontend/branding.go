package frontend

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"strings"
	"time"
)

var timeZero time.Time

const DefaultIconColor = "#2563EB"

type Branding struct{ Name, Color string }
type iconRect struct {
	x, y, w, h, r float64
	outline       bool
}

// Shared geometry for SVG, PNG and ICO, on a 100×100 full-bleed canvas.
var iconGeometry = []iconRect{
	{25, 21, 14, 5, 2, true}, {43, 21, 14, 5, 2, true}, {61, 21, 14, 5, 2, true},
	{25, 29, 14, 42, 3, false}, {43, 29, 14, 30, 3, false}, {61, 29, 14, 20, 3, false},
}

// Compact geometry keeps hollow headers recognizable at 16–32 pixels.
func geometry(compact bool) []iconRect {
	if !compact {
		return iconGeometry
	}
	return []iconRect{
		{22, 18, 16, 16, 4, true}, {42, 18, 16, 16, 4, true}, {62, 18, 16, 16, 4, true},
		{22, 42, 16, 42, 3, false}, {42, 42, 16, 30, 3, false}, {62, 42, 16, 20, 3, false},
	}
}
func iconColor(value string) color.RGBA {
	raw, err := hex.DecodeString(strings.TrimPrefix(value, "#"))
	if err != nil || len(raw) != 3 {
		return color.RGBA{37, 99, 235, 255}
	}
	return color.RGBA{raw[0], raw[1], raw[2], 255}
}
func luminance(c color.RGBA) float64 {
	linear := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return .2126*linear(c.R) + .7152*linear(c.G) + .0722*linear(c.B)
}
func foreground(bg color.RGBA) color.RGBA {
	dark := color.RGBA{17, 24, 39, 255}
	l := luminance(bg)
	if (1.05 / (l + .05)) >= ((l + .05) / (luminance(dark) + .05)) {
		return color.RGBA{255, 255, 255, 255}
	}
	return dark
}
func inside(x, y float64, r iconRect) bool {
	cx := math.Max(r.x+r.r, math.Min(x, r.x+r.w-r.r))
	cy := math.Max(r.y+r.r, math.Min(y, r.y+r.h-r.r))
	return x >= r.x && x <= r.x+r.w && y >= r.y && y <= r.y+r.h && (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r.r*r.r
}
func iconPNG(size int, bg, fg color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			count := 0
			for sy := 0; sy < 3; sy++ {
				for sx := 0; sx < 3; sx++ {
					px := (float64(x) + (float64(sx)+.5)/3) * 100 / float64(size)
					py := (float64(y) + (float64(sy)+.5)/3) * 100 / float64(size)
					for _, r := range geometry(size <= 32) {
						if !inside(px, py, r) {
							continue
						}
						if r.outline {
							stroke := math.Max(.8, 100/float64(size))
							radius := math.Max(0, r.r-stroke)
							inner := iconRect{r.x + stroke, r.y + stroke, r.w - 2*stroke, r.h - 2*stroke, radius, false}
							if inside(px, py, inner) {
								continue
							}
						}
						count++
						break
					}
				}
			}
			blend := func(a, b uint8) uint8 { return uint8((int(a)*(9-count) + int(b)*count) / 9) }
			img.SetRGBA(x, y, color.RGBA{blend(bg.R, fg.R), blend(bg.G, fg.G), blend(bg.B, fg.B), 255})
		}
	}
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	return out.Bytes()
}
func iconSVG(bg, fg string) []byte {
	var out strings.Builder
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><style>.compact{display:none}@media(max-width:32px){.regular{display:none}.compact{display:inline}}</style><rect width="100" height="100" fill="%s"/>`, bg)
	for _, compact := range []bool{false, true} {
		class := "regular"
		if compact {
			class = "compact"
		}
		fmt.Fprintf(&out, `<g class="%s">`, class)
		for _, r := range geometry(compact) {
			fill, stroke := fg, "none"
			inset := 0.
			if r.outline {
				fill = "none"
				stroke = fg
				inset = .4
			}
			extra := ""
			width := "0.8"
			if compact && r.outline {
				extra = ` vector-effect="non-scaling-stroke"`
				width = "1"
				inset = 0
			}
			fmt.Fprintf(&out, `<rect x="%g" y="%g" width="%g" height="%g" rx="%g" fill="%s" stroke="%s" stroke-width="%s"%s/>`, r.x+inset, r.y+inset, r.w-2*inset, r.h-2*inset, r.r-inset, fill, stroke, width, extra)
		}
		out.WriteString(`</g>`)
	}
	out.WriteString(`</svg>`)
	return []byte(out.String())
}
func iconICO(bg, fg color.RGBA) []byte {
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, []uint16{0, 1, 3})
	chunks := [][]byte{iconPNG(16, bg, fg), iconPNG(32, bg, fg), iconPNG(48, bg, fg)}
	offset := uint32(6 + 3*16)
	for i, size := range []byte{16, 32, 48} {
		out.Write([]byte{size, size, 0, 0})
		_ = binary.Write(&out, binary.LittleEndian, []uint16{1, 32})
		_ = binary.Write(&out, binary.LittleEndian, []uint32{uint32(len(chunks[i])), offset})
		offset += uint32(len(chunks[i]))
	}
	for _, chunk := range chunks {
		out.Write(chunk)
	}
	return out.Bytes()
}
func brandingAssets(b Branding) (map[string][]byte, string) {
	if b.Name == "" {
		b.Name = "JKB"
	}
	if b.Color == "" {
		b.Color = DefaultIconColor
	}
	bg := iconColor(b.Color)
	fg := foreground(bg)
	fgHex := fmt.Sprintf("#%02X%02X%02X", fg.R, fg.G, fg.B)
	sum := sha256.Sum256([]byte("icon-v3:" + b.Name + ":" + b.Color))
	version := fmt.Sprintf("%x", sum[:8])
	suffix := "?v=" + version
	files := map[string][]byte{"/favicon.svg": iconSVG(b.Color, fgHex), "/favicon.ico": iconICO(bg, fg), "/apple-touch-icon.png": iconPNG(180, bg, fg), "/icon-192.png": iconPNG(192, bg, fg), "/icon-512.png": iconPNG(512, bg, fg), "/icon-maskable.png": iconPNG(512, bg, fg)}
	icons := []map[string]string{}
	for _, item := range []struct{ path, size, purpose string }{{"/icon-192.png", "192x192", "any"}, {"/icon-512.png", "512x512", "any"}, {"/icon-maskable.png", "512x512", "maskable"}} {
		icons = append(icons, map[string]string{"src": item.path + suffix, "sizes": item.size, "type": "image/png", "purpose": item.purpose})
	}
	files["/manifest.webmanifest"], _ = json.Marshal(map[string]any{"id": "/", "name": b.Name, "short_name": b.Name, "start_url": "/", "scope": "/", "display": "standalone", "theme_color": b.Color, "background_color": b.Color, "icons": icons})
	head := fmt.Sprintf(`<title>%s</title><meta name="apple-mobile-web-app-title" content="%s"><meta name="apple-mobile-web-app-capable" content="yes"><meta name="apple-mobile-web-app-status-bar-style" content="default"><meta name="theme-color" content="%s"><link rel="icon" href="/favicon.svg%s" type="image/svg+xml"><link rel="alternate icon" href="/favicon.ico%s"><link rel="apple-touch-icon" href="/apple-touch-icon.png%s"><link rel="manifest" href="/manifest.webmanifest%s">`, html.EscapeString(b.Name), html.EscapeString(b.Name), b.Color, suffix, suffix, suffix, suffix)
	return files, head
}
func serveBranding(w http.ResponseWriter, r *http.Request, files map[string][]byte) bool {
	data, ok := files[r.URL.Path]
	if !ok {
		return false
	}
	contentType := "image/png"
	if strings.HasSuffix(r.URL.Path, ".svg") {
		contentType = "image/svg+xml"
	}
	if strings.HasSuffix(r.URL.Path, ".ico") {
		contentType = "image/x-icon"
	}
	if strings.HasSuffix(r.URL.Path, ".webmanifest") {
		contentType = "application/manifest+json"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, r.URL.Path /* runtime files */, timeZero, bytes.NewReader(data))
	return true
}
