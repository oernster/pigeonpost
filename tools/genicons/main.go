// Command genicons derives every platform icon asset from a single master PNG at the repo root.
// It pads the master to a square, transparent canvas and emits:
//
//	build/appicon.png              (512, used by Wails)
//	build/eml.png                  (512, the .eml file-association icon Wails packages)
//	build/windows/icon.ico         (multi-size PNG-in-ICO for the Windows exe and installer)
//	build/linux/icons/pigeonpost_<size>.png (the hicolor set installed by build_flatpak.sh)
//	frontend/src/assets/pigeonpost.png (256, used by the in-app About dialog)
//	frontend/src/assets/icons/*.png       (the title-bar and folder-list glyphs)
//
// The glyph set is derived from the artwork in assets/ at the repo root, one output per master. Each is
// cropped to its visible pixels, centred on a transparent square and scaled down to one common size, so
// every glyph carries the same visual weight whatever the master's own framing was. Without that crop a
// letterboxed master (the inbox tray fills a little over half its canvas) renders half the height of a
// master drawn edge to edge beside it.
//
// It also derives the donate button's artwork from its own master, donate.png. That one is not an icon
// and is handled separately: it is a wide pair of glasses drawn at a button's height, so squaring it
// would spend half the height on empty canvas. It is cropped to its opaque artwork and scaled by height
// alone, into frontend/src/assets/donate.png for the app and docs/donate.png for the landing page, which
// puts the same mark on its own donate button.
//
// Run from the repo root: go run ./tools/genicons
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const masterFile = "pigeonpost.png"

// The donate button's own master and the two copies drawn from it: the one the front end bundles and
// the one the landing page serves. donateHeight is four times the tray's glyph height
// (--titlebar-icon-size, 29px), so the button stays crisp under display scaling without carrying the
// master's 1.7MB into the binary; the site's button draws it smaller still, so the one size covers both.
const donateMasterFile = "donate.png"

var donateOutputs = []string{
	filepath.Join("frontend", "src", "assets", "donate.png"),
	filepath.Join("docs", "donate.png"),
}

const donateHeight = 116

// The title-bar and folder-list glyphs: every PNG in glyphMastersDir yields one square PNG of the same
// name in glyphOutputDir.
const (
	glyphMastersDir = "assets"
	glyphOutputDir  = "frontend/src/assets/icons"
)

// glyphSide is the pixel side of each generated glyph. The largest surface that draws one is the title
// bar at --titlebar-glyph-size (61px), so this is four times that, on the same reasoning as donateHeight:
// crisp under display scaling without carrying a megabyte-scale master into the binary. Raising that CSS
// token means raising this with it; the two are kept in step by hand, since neither language can read
// the other's constant.
const glyphSide = 244

// glyphAlphaFloor is the alpha, on the usual 0 to 255 scale, at or below which a pixel is treated as
// empty canvas when cropping. Artwork of this kind carries a soft glow fading to alpha 1 far outside the
// drawing, which a bare "alpha is zero" test reads as content: measured across this set, that test put
// the visible box at 96 to 99 per cent of the canvas on every master, hiding the very differences the
// crop exists to remove.
const glyphAlphaFloor = 8

// fileAssocIconName is the base name (no extension) of the icon Wails bundles
// for the .eml file association. It MUST match the iconName on that
// fileAssociation in wails.json: the darwin packager reads
// build/<fileAssocIconName>.png; an empty iconName makes it open ".png"
// and abort packaging.
const fileAssocIconName = "eml"

var icoSizes = []int{16, 24, 32, 48, 64, 128, 256}

var hicolorSizes = []int{16, 24, 32, 48, 64, 128, 256, 512}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "genicons:", err)
		os.Exit(1)
	}
}

func run() error {
	master, err := loadPNG(masterFile)
	if err != nil {
		return err
	}
	square := toSquare(master)

	if err := writePNG(filepath.Join("build", "appicon.png"), resize(square, 512)); err != nil {
		return err
	}
	if err := writePNG(filepath.Join("build", fileAssocIconName+".png"), resize(square, 512)); err != nil {
		return err
	}
	if err := writePNG(filepath.Join("frontend", "src", "assets", "pigeonpost.png"), resize(square, 256)); err != nil {
		return err
	}

	images := make([]*image.RGBA, 0, len(icoSizes))
	for _, size := range icoSizes {
		images = append(images, resize(square, size))
	}
	if err := writeICO(filepath.Join("build", "windows", "icon.ico"), images); err != nil {
		return err
	}

	for _, size := range hicolorSizes {
		name := fmt.Sprintf("pigeonpost_%d.png", size)
		if err := writePNG(filepath.Join("build", "linux", "icons", name), resize(square, size)); err != nil {
			return err
		}
	}
	donate, err := loadPNG(donateMasterFile)
	if err != nil {
		return err
	}
	donateMark := scaleToHeight(cropToArtwork(donate), donateHeight)
	for _, path := range donateOutputs {
		if err := writePNG(path, donateMark); err != nil {
			return err
		}
	}

	glyphs, err := writeGlyphs()
	if err != nil {
		return err
	}

	fmt.Println("genicons: wrote appicon.png, eml.png, icon.ico, the hicolor set, the About asset, the donate artwork and", glyphs, "glyphs")
	return nil
}

// writeGlyphs derives one square glyph per master in glyphMastersDir, returning how many it wrote. The
// directory is read rather than listed here, so adding a master is enough to ship it; the front end names
// the file it wants, so a master that goes missing fails the frontend build rather than passing silently.
func writeGlyphs() (int, error) {
	entries, err := os.ReadDir(glyphMastersDir)
	if err != nil {
		return 0, fmt.Errorf("read glyph masters %q: %w", glyphMastersDir, err)
	}
	written := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
			continue
		}
		master, err := loadPNG(filepath.Join(glyphMastersDir, entry.Name()))
		if err != nil {
			return 0, err
		}
		name := strings.ToLower(entry.Name())
		if err := writePNG(filepath.Join(glyphOutputDir, name), resize(toSquare(cropToArtwork(master)), glyphSide)); err != nil {
			return 0, err
		}
		written++
	}
	return written, nil
}

// cropToArtwork returns the tight box of src's visible pixels, leaving its aspect ratio alone. Visible
// means alpha above glyphAlphaFloor; see that constant for why a bare zero test does not do. A fully
// transparent image has no artwork to crop to, so it comes back unchanged.
func cropToArtwork(src image.Image) image.Image {
	// At() reports alpha on the 16-bit scale, so the 8-bit floor is scaled to match.
	const floor = glyphAlphaFloor * 0x101

	b := src.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := src.At(x, y).RGBA(); a <= floor {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x >= maxX {
				maxX = x + 1
			}
			if y >= maxY {
				maxY = y + 1
			}
		}
	}
	if minX >= maxX || minY >= maxY {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, maxX-minX, maxY-minY))
	xdraw.Draw(dst, dst.Bounds(), src, image.Pt(minX, minY), xdraw.Src)
	return dst
}

// scaleToHeight scales src to height, keeping its aspect ratio.
func scaleToHeight(src image.Image, height int) *image.RGBA {
	b := src.Bounds()
	width := b.Dx() * height / b.Dy()
	if width < 1 {
		width = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open master %q: %w", path, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode master %q: %w", path, err)
	}
	return img, nil
}

// toSquare centres the source on a transparent square canvas sized to its larger dimension.
func toSquare(src image.Image) *image.RGBA {
	b := src.Bounds()
	side := b.Dx()
	if b.Dy() > side {
		side = b.Dy()
	}
	dst := image.NewRGBA(image.Rect(0, 0, side, side))
	offset := image.Pt((side-b.Dx())/2, (side-b.Dy())/2)
	xdraw.Draw(dst, image.Rectangle{Min: offset, Max: offset.Add(image.Pt(b.Dx(), b.Dy()))}, src, b.Min, xdraw.Src)
	return dst
}

func resize(src image.Image, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %q: %w", path, err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %q: %w", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode %q: %w", path, err)
	}
	return nil
}

// writeICO writes a Vista-style ICO whose entries hold PNG-compressed images.
func writeICO(path string, images []*image.RGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %q: %w", path, err)
	}

	type entry struct {
		size int
		data []byte
	}
	entries := make([]entry, 0, len(images))
	for _, img := range images {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return fmt.Errorf("encode ico frame: %w", err)
		}
		entries = append(entries, entry{size: img.Bounds().Dx(), data: buf.Bytes()})
	}

	const headerSize = 6
	const dirEntrySize = 16
	var out bytes.Buffer
	writeLE := func(v any) error { return binary.Write(&out, binary.LittleEndian, v) }

	// ICONDIR header.
	if err := writeLE(uint16(0)); err != nil { // reserved
		return err
	}
	if err := writeLE(uint16(1)); err != nil { // type: 1 = icon
		return err
	}
	if err := writeLE(uint16(len(entries))); err != nil {
		return err
	}

	offset := headerSize + dirEntrySize*len(entries)
	for _, e := range entries {
		dim := byte(e.size)
		if e.size >= 256 {
			dim = 0 // 0 means 256 in the ICO format
		}
		out.WriteByte(dim)                                      // width
		out.WriteByte(dim)                                      // height
		out.WriteByte(0)                                        // colour count
		out.WriteByte(0)                                        // reserved
		_ = binary.Write(&out, binary.LittleEndian, uint16(1))  // colour planes
		_ = binary.Write(&out, binary.LittleEndian, uint16(32)) // bits per pixel
		_ = binary.Write(&out, binary.LittleEndian, uint32(len(e.data)))
		_ = binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(e.data)
	}
	for _, e := range entries {
		out.Write(e.data)
	}

	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write ico %q: %w", path, err)
	}
	return nil
}
