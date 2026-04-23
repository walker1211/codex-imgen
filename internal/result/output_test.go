package result

import "testing"

func TestRenderTextMultipleImages(t *testing.T) {
	res := Result{
		OK: true,
		Images: []ImageResult{
			{Index: 1, Path: "/tmp/1.png"},
			{Index: 2, Path: "/tmp/2.png"},
		},
	}
	got := RenderText(res)
	want := "/tmp/1.png\n/tmp/2.png\n"
	if got != want {
		t.Fatalf("RenderText = %q, want %q", got, want)
	}
}
