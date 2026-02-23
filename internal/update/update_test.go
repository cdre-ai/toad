package update

import "testing"

func TestIsNewer(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.0.1", "1.0.0", true},
		{"1.1.0", "1.0.9", true},
		{"2.0.0", "1.9.9", true},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.1", false},
		{"0.9.0", "1.0.0", false},
		{"1.2.3", "1.2.3", false},
		{"1.10.0", "1.9.0", true},
		{"1.0.0", "0.99.99", true},
		// Pre-release comparisons
		{"0.1.0-beta.4", "0.1.0-beta.3", true},
		{"0.1.0-beta.3", "0.1.0-beta.4", false},
		{"0.1.0-beta.3", "0.1.0-beta.3", false},
		{"0.1.0-beta.10", "0.1.0-beta.9", true},
		// Stable beats pre-release of same version
		{"0.1.0", "0.1.0-beta.4", true},
		{"0.1.0-beta.4", "0.1.0", false},
		// Higher version beats lower pre-release
		{"0.2.0-beta.1", "0.1.0-beta.9", true},
		{"0.1.0-beta.1", "0.0.9", true},
	}

	for _, tt := range tests {
		got := isNewer(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input   string
		want    [3]int
		wantPre string
	}{
		{"1.2.3", [3]int{1, 2, 3}, ""},
		{"0.1.0", [3]int{0, 1, 0}, ""},
		{"10.20.30", [3]int{10, 20, 30}, ""},
		{"1.2", [3]int{1, 2, 0}, ""},
		{"1", [3]int{1, 0, 0}, ""},
		{"", [3]int{0, 0, 0}, ""},
		{"1.2.3-beta", [3]int{1, 2, 3}, "beta"},
		{"0.1.0-beta.4", [3]int{0, 1, 0}, "beta.4"},
		{"1.0.0-rc.1", [3]int{1, 0, 0}, "rc.1"},
	}

	for _, tt := range tests {
		got, gotPre := parseSemver(tt.input)
		if got != tt.want {
			t.Errorf("parseSemver(%q) parts = %v, want %v", tt.input, got, tt.want)
		}
		if gotPre != tt.wantPre {
			t.Errorf("parseSemver(%q) pre = %q, want %q", tt.input, gotPre, tt.wantPre)
		}
	}
}

func TestCheckDevVersion(t *testing.T) {
	info, err := Check("dev")
	if err != nil {
		t.Fatalf("Check(dev) returned error: %v", err)
	}
	if info != nil {
		t.Errorf("Check(dev) should return nil info, got %+v", info)
	}
}

func TestCheckEmptyVersion(t *testing.T) {
	info, err := Check("")
	if err != nil {
		t.Fatalf(`Check("") returned error: %v`, err)
	}
	if info != nil {
		t.Errorf(`Check("") should return nil info, got %+v`, info)
	}
}
