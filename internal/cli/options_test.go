package cli

import "testing"

func TestParseOptionsSubmitCommand(t *testing.T) {
	cmd, err := ParseCommand([]string{"submit", "--count", "4", "--concurrency", "2", "draw a dragon"})
	if err != nil {
		t.Fatalf("ParseCommand returned error: %v", err)
	}

	if cmd.Name != "submit" {
		t.Fatalf("name = %q", cmd.Name)
	}
	if cmd.Prompt != "draw a dragon" {
		t.Fatalf("prompt = %q", cmd.Prompt)
	}
	if cmd.Count != 4 {
		t.Fatalf("count = %d", cmd.Count)
	}
	if cmd.Concurrency != 2 {
		t.Fatalf("concurrency = %d", cmd.Concurrency)
	}
}

func TestParseOptionsRunCommandWithImages(t *testing.T) {
	cmd, err := ParseCommand([]string{"--image", "./1.png", "--image", "./2.png", "draw a dragon"})
	if err != nil {
		t.Fatalf("ParseCommand returned error: %v", err)
	}
	if cmd.Name != "run" {
		t.Fatalf("name = %q", cmd.Name)
	}
	if len(cmd.Images) != 2 {
		t.Fatalf("images = %v", cmd.Images)
	}
	if cmd.Images[0] != "./1.png" || cmd.Images[1] != "./2.png" {
		t.Fatalf("images = %v", cmd.Images)
	}
	if cmd.Prompt != "draw a dragon" {
		t.Fatalf("prompt = %q", cmd.Prompt)
	}
}

func TestParseOptionsSubmitCommandWithImages(t *testing.T) {
	cmd, err := ParseCommand([]string{"submit", "--image", "./1.png", "--image", "./2.png", "draw a dragon"})
	if err != nil {
		t.Fatalf("ParseCommand returned error: %v", err)
	}
	if cmd.Name != "submit" {
		t.Fatalf("name = %q", cmd.Name)
	}
	if len(cmd.Images) != 2 {
		t.Fatalf("images = %v", cmd.Images)
	}
}

func TestParseOptionsDoctorOpenClaw(t *testing.T) {
	cmd, err := ParseCommand([]string{"doctor", "openclaw"})
	if err != nil {
		t.Fatalf("ParseCommand returned error: %v", err)
	}
	if cmd.Name != "doctor" {
		t.Fatalf("name = %q", cmd.Name)
	}
	if cmd.DoctorTarget != "openclaw" {
		t.Fatalf("doctor target = %q", cmd.DoctorTarget)
	}
}

func TestParseOptionsDoctorRequiresTarget(t *testing.T) {
	if _, err := ParseCommand([]string{"doctor"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsDoctorRejectsUnknownTarget(t *testing.T) {
	if _, err := ParseCommand([]string{"doctor", "telegram"}); err == nil {
		t.Fatal("expected error")
	}
}
