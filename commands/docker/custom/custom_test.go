package custom

import "testing"

func TestFileFromCustomName(t *testing.T) {
	old := customName
	defer func() { customName = old }()

	customName = "custom-demo"
	got := fileFromCustomName()
	want := "custom-demo.yml"

	if got != want {
		t.Fatalf("fileFromCustomName() = %q, want %q", got, want)
	}
}
