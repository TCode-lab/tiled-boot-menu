package theme

import "testing"

func TestBuildGeometry(t *testing.T) {
	th := Build(48, 7, 4)
	if th.Width != 448 || th.Height != 236 {
		t.Fatalf("window = %dx%d", th.Width, th.Height)
	}
	if th.ContentW != 376 || th.ContentH != 220 {
		t.Fatalf("content = %dx%d", th.ContentW, th.ContentH)
	}
	if th.LargeTile != 100 {
		t.Fatalf("large tile = %d", th.LargeTile)
	}
	th = Build(52, 6, 3)
	if th.Width != 3*8+48+(6*52+5*4+16) {
		t.Fatalf("width = %d", th.Width)
	}
	if th.Height != 16+(3*52+2*4+16) {
		t.Fatalf("height = %d", th.Height)
	}
}
