// Package taskbar reflects the unread-message total onto the Windows taskbar button as a small red
// overlay badge showing the count. The badge rendering (a red circle with a centred number) is pure
// and cross-platform so it can be unit tested; the COM and GDI plumbing that turns it into a shell
// overlay icon lives in the windows-only file. On platforms without a taskbar badge the overlay is a
// no-op.
package taskbar

import (
	"image"
	"image/color"
	"math"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	// iconSize is the square pixel size of the rendered overlay icon. The shell scales it down to the
	// overlay slot, so a value a little above the 16px slot keeps the number crisp.
	iconSize = 20
	// badgeCap is the largest count shown as a plain number. Above it the badge shows "9+" so the label
	// never exceeds two glyphs and stays legible in the small overlay.
	badgeCap = 9
)

var (
	// badgeFill is the red disc colour, matching the Windows notification accent.
	badgeFill = color.RGBA{R: 0xE8, G: 0x11, B: 0x23, A: 0xFF}
	// badgeText is the colour of the count drawn on the disc.
	badgeText = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
)

// BadgeLabel formats an unread total for the overlay. Zero or negative yields "" (no badge); a total
// above the cap yields the cap followed by "+".
func BadgeLabel(total int) string {
	if total <= 0 {
		return ""
	}
	if total > badgeCap {
		return strconv.Itoa(badgeCap) + "+"
	}
	return strconv.Itoa(total)
}

// renderBadge draws a filled red circle with the label centred in white and returns a square RGBA
// icon. An empty label yields nil, meaning there is nothing to show.
func renderBadge(label string) *image.RGBA {
	if label == "" {
		return nil
	}
	img := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))
	drawDisc(img, badgeFill)
	drawLabel(img, label)
	return img
}

// drawDisc fills an antialiased circle covering the icon, writing premultiplied RGBA (the Go image
// convention) so the one-pixel edge band blends cleanly.
func drawDisc(img *image.RGBA, fill color.RGBA) {
	const centre = float64(iconSize) / 2
	radius := centre - 0.5
	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			dx := float64(x) + 0.5 - centre
			dy := float64(y) + 0.5 - centre
			coverage := radius + 0.5 - math.Sqrt(dx*dx+dy*dy)
			if coverage <= 0 {
				continue
			}
			if coverage > 1 {
				coverage = 1
			}
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(float64(fill.R) * coverage),
				G: uint8(float64(fill.G) * coverage),
				B: uint8(float64(fill.B) * coverage),
				A: uint8(coverage * float64(0xFF)),
			})
		}
	}
}

// drawLabel centres the count on the disc in white, drawn twice with a one-pixel horizontal offset to
// fake a bold weight so the thin bitmap font reads clearly at overlay size.
func drawLabel(img *image.RGBA, label string) {
	face := basicfont.Face7x13
	width := font.MeasureString(face, label).Ceil()
	metrics := face.Metrics()
	x := (iconSize - width) / 2
	baseline := (iconSize + metrics.Ascent.Ceil() - metrics.Descent.Ceil()) / 2
	drawer := &font.Drawer{Dst: img, Src: image.NewUniform(badgeText), Face: face}
	for _, offset := range []int{0, 1} {
		drawer.Dot = fixed.P(x+offset, baseline)
		drawer.DrawString(label)
	}
}
