package taskbar

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestDecodeScaledIconResizes(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.Set(x, y, color.NRGBA{R: 0xFF, A: 0xFF})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	img, err := decodeScaledIcon(buf.Bytes(), 8)
	if err != nil {
		t.Fatalf("decodeScaledIcon: %v", err)
	}
	if img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 {
		t.Fatalf("scaled to %dx%d, want 8x8", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestDecodeScaledIconRejectsBadData(t *testing.T) {
	if _, err := decodeScaledIcon([]byte("not a png"), 8); err == nil {
		t.Error("expected an error decoding non-PNG data")
	}
}

func TestCompositeBadgePlacesBadgeInCorner(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, trayIconPx, trayIconPx))
	for i := 0; i < len(base.Pix); i += 4 {
		base.Pix[i], base.Pix[i+1], base.Pix[i+2], base.Pix[i+3] = 0xFF, 0xFF, 0xFF, 0xFF
	}
	badge := renderBadge(BadgeLabel(3))
	if badge == nil {
		t.Fatal("expected a badge image for a non-zero count")
	}
	out := compositeBadge(base, badge)
	if out.Bounds() != base.Bounds() {
		t.Fatalf("composite bounds %v, want %v", out.Bounds(), base.Bounds())
	}
	if tl := out.RGBAAt(0, 0); tl.R < 200 || tl.G < 200 || tl.B < 200 {
		t.Errorf("top-left = %v, want the untouched white base", tl)
	}
	var red bool
	for y := trayIconPx - badge.Bounds().Dy(); y < trayIconPx; y++ {
		for x := trayIconPx - badge.Bounds().Dx(); x < trayIconPx; x++ {
			if c := out.RGBAAt(x, y); c.R > 150 && c.G < 90 && c.B < 90 {
				red = true
			}
		}
	}
	if !red {
		t.Error("expected red badge pixels in the bottom-right corner")
	}
}
