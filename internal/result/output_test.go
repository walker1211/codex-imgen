package result

import "testing"

func TestRenderTextSuccess(t *testing.T) {
	res := Result{OK: true, Path: "/tmp/generated/image.png"}
	got := RenderText(res)
	if got != "/tmp/generated/image.png\n" {
		t.Fatalf("RenderText = %q", got)
	}
}

func TestRenderJSONFailure(t *testing.T) {
	res := Result{OK: false, Prompt: "dragon", Error: "image path not found in codex output"}
	got := RenderJSON(res)
	want := "{\"ok\":false,\"prompt\":\"dragon\",\"error\":\"image path not found in codex output\"}\n"
	if got != want {
		t.Fatalf("RenderJSON = %q, want %q", got, want)
	}
}
