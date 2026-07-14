// Command genicons derives every platform icon asset from a single master PNG at the repo root.
// It pads the master to a square, transparent canvas and emits:
//
//	build/appicon.png              (512, used by Wails)
//	build/windows/icon.ico         (multi-size PNG-in-ICO for the Windows exe and installer)
//	build/linux/icons/pigeonpost_<size>.png (the hicolor set installed by build_flatpak.sh)
//	frontend/src/assets/pigeonpost.png (256, used by the in-app About dialog)
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

	xdraw "golang.org/x/image/draw"
)

const masterFile = "pigeonpost.png"

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
	fmt.Println("genicons: wrote appicon.png, icon.ico, the hicolor set and the About asset")
	return nil
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
