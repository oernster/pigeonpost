package taskbar

import "testing"

func TestBadgeLabel(t *testing.T) {
	cases := map[int]string{
		-5:  "",
		0:   "",
		1:   "1",
		9:   "9",
		10:  "9+",
		250: "9+",
	}
	for total, want := range cases {
		if got := BadgeLabel(total); got != want {
			t.Errorf("BadgeLabel(%d) = %q, want %q", total, got, want)
		}
	}
}

func TestRenderBadgeEmptyIsNil(t *testing.T) {
	if renderBadge("") != nil {
		t.Error("an empty label should render no image")
	}
}

func TestRenderBadgeDrawsDiscAndText(t *testing.T) {
	img := renderBadge("8")
	if img == nil {
		t.Fatal("expected an image for a non-empty label")
	}
	if img.Bounds().Dx() != iconSize || img.Bounds().Dy() != iconSize {
		t.Fatalf("icon is %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), iconSize, iconSize)
	}

	var red, white bool
	pixels := img.Bounds().Dx() * img.Bounds().Dy()
	for i := 0; i < pixels; i++ {
		r, g, b, a := img.Pix[i*4+0], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3]
		if a == 0xFF && r > 200 && g < 90 && b < 90 {
			red = true
		}
		if r > 240 && g > 240 && b > 240 {
			white = true
		}
	}
	if !red {
		t.Error("expected red disc pixels")
	}
	if !white {
		t.Error("expected white text pixels")
	}
}
