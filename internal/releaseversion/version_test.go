package releaseversion

import "testing"

func TestParse(t *testing.T) {
	for _, input := range []string{"v0.34.0", "0.34.1", "0.35.0-rc.1", "0.35.0+build.1", "v2026.10.31.0", "2027.01.02.1", "2027.1.2.1", "2024.02.29.10"} {
		if _, err := Parse(input); err != nil {
			t.Errorf("Parse(%q): %v", input, err)
		}
	}
	for _, input := range []string{"", "latest", "nightly", "vv0.34.0", "2026.02.29.0", "2026.04.31.0", "2026.13.01.0", "2026.00.01.0", "0000.01.01.0", "2026.001.01.0", "2026.01.00.0", "2026.01.01.-1", "2026.01.01.01", "2026.01.01.0-rc.1", "2026.01.01.0+build", "2026.01.01.0.1", "2026.01.01.18446744073709551616"} {
		if _, err := Parse(input); err == nil {
			t.Errorf("Parse(%q) succeeded", input)
		}
	}
}

func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		want     int
	}{
		{"0.9.0", "0.10.0", -1},
		{"0.34.0-rc.1", "0.34.0", -1},
		{"0.34.0+build.1", "0.34.0", 0},
		{"0.34.0", "2026.10.31.0", -1},
		{"9999.0.0", "2026.10.31.0", -1},
		{"2026.10.31.9", "2026.10.31.10", -1},
		{"2026.10.31.10", "2026.11.01.0", -1},
		{"2026.12.31.1", "2027.01.01.0", -1},
		{"v2027.01.02.1", "2027.1.2.1", 0},
	} {
		from, err := Parse(tc.from)
		if err != nil {
			t.Fatal(err)
		}
		to, err := Parse(tc.to)
		if err != nil {
			t.Fatal(err)
		}
		if got := from.Compare(to); got != tc.want || to.Compare(from) != -tc.want {
			t.Errorf("%s vs %s: got %d, want %d", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestStable(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  bool
	}{{"0.34.0", true}, {"0.35.0-rc.1", false}, {"2026.10.31.0", true}} {
		v, err := Parse(tc.input)
		if err != nil || v.Stable() != tc.want {
			t.Fatalf("Stable(%q)=%t, %v", tc.input, v.Stable(), err)
		}
	}
}
