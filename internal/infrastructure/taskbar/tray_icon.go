package taskbar

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"

	xdraw "golang.org/x/image/draw"
)

// trayIconPx is the pixel size of the composited tray icon before the shell scales it to the small
// notification slot. Rendering above the 16px slot keeps the app icon and unread badge crisp on
// high-DPI displays.
const trayIconPx = 32

// decodeScaledIcon decodes a PNG and fits it, preserving aspect ratio, onto a square premultiplied-alpha
// image of the given size, ready to be turned into a tray icon. A non-square source is centred with
// transparent margins rather than stretched.
func decodeScaledIcon(data []byte, size int) (*image.RGBA, error) {
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	b := src.Bounds()
	w, h := size, size
	if b.Dx() > b.Dy() {
		h = size * b.Dy() / b.Dx()
	} else if b.Dy() > b.Dx() {
		w = size * b.Dx() / b.Dy()
	}
	ox, oy := (size-w)/2, (size-h)/2
	xdraw.CatmullRom.Scale(dst, image.Rect(ox, oy, ox+w, oy+h), src, b, xdraw.Src, nil)
	return dst, nil
}

// compositeBadge returns a copy of the base icon with the unread badge drawn in its bottom-right corner.
// The badge carries its own transparent surround, so it is alpha-composited over the base.
func compositeBadge(base, badge *image.RGBA) *image.RGBA {
	out := image.NewRGBA(base.Bounds())
	draw.Draw(out, out.Bounds(), base, base.Bounds().Min, draw.Src)
	size := badge.Bounds().Size()
	offset := image.Pt(out.Bounds().Dx()-size.X, out.Bounds().Dy()-size.Y)
	target := image.Rectangle{Min: offset, Max: offset.Add(size)}
	draw.Draw(out, target, badge, badge.Bounds().Min, draw.Over)
	return out
}
