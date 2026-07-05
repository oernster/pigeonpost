#!/usr/bin/env python3
"""Regenerate every PigeonPost icon from the master pigeonpost.png.

The master already has a transparent surround. This crops that transparent margin down to the artwork,
centres the artwork on a square transparent canvas with a small even margin so the pigeon fills most of
the icon, writes the square master back, then produces every derived asset from it. Run after editing
the artwork:

    python generate_icons.py
"""

from __future__ import annotations

from pathlib import Path

from PIL import Image

# The master lives at the repository root and is the single source for every icon.
MASTER = Path("pigeonpost.png")

# Fraction of the square canvas kept as an even transparent margin around the artwork. Kept small so the
# pigeon fills most of the square; the artwork is taller than wide, so this sets the height fill and the
# narrower width keeps its own side margins.
MARGIN_FRAC = 0.02

# Copies that use the square master as-is (shown as <img> in the app and on GitHub Pages).
SAME_MASTER_COPIES = [
    Path("frontend/src/assets/pigeonpost.png"),
    Path("docs/pigeonpost.png"),
]

# Square PNGs and their sizes.
SQUARE_PNGS = [
    (Path("build/appicon.png"), 512),
    (Path("installer/build/appicon.png"), 512),
    (Path("installer/frontend/dist/icon.png"), 256),
]

# Multi-resolution Windows icons.
ICO_TARGETS = [
    Path("build/windows/icon.ico"),
    Path("installer/build/windows/icon.ico"),
]
ICO_SIZES = [(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]


def square_master(image: Image.Image, margin_frac: float) -> Image.Image:
    """Crop image to its opaque artwork and centre it on a square transparent canvas with a margin."""
    image = image.convert("RGBA")
    bbox = image.getchannel("A").getbbox()  # tight box of the non-transparent artwork
    if bbox is None:
        raise SystemExit("master icon has no opaque artwork to crop to")
    art = image.crop(bbox)
    side = round(max(art.size) / (1 - 2 * margin_frac))
    canvas = Image.new("RGBA", (side, side), (0, 0, 0, 0))
    canvas.paste(art, ((side - art.width) // 2, (side - art.height) // 2), art)
    return canvas


def write_png(image: Image.Image, path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    image.save(path, format="PNG")
    print(f"  wrote {path} ({image.width}x{image.height})")


def main() -> None:
    if not MASTER.exists():
        raise SystemExit(f"master icon {MASTER} not found")

    master = square_master(Image.open(MASTER), MARGIN_FRAC)
    print("Square master:")
    write_png(master, MASTER)
    for path in SAME_MASTER_COPIES:
        write_png(master, path)

    print("Derived icons:")
    for path, size in SQUARE_PNGS:
        write_png(master.resize((size, size), Image.LANCZOS), path)

    biggest = max(s for s, _ in ICO_SIZES)
    ico_source = master.resize((biggest, biggest), Image.LANCZOS)
    for path in ICO_TARGETS:
        path.parent.mkdir(parents=True, exist_ok=True)
        ico_source.save(path, format="ICO", sizes=ICO_SIZES)
        print(f"  wrote {path} (multi-size .ico)")


if __name__ == "__main__":
    main()
