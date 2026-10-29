package domain

import (
	"testing"
)

func TestClassifyChange(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		wantType ChangeType
		wantSafe bool
	}{
		{"1.2.3", "1.2.4", ChangePatch, true},
		{"1.2.3", "1.3.0", ChangeMinor, true},
		{"1.2.3", "2.0.0", ChangeMajor, false},
		{"11.20.0", "12.2.1", ChangeMajor, false},
		{"0.164.0", "0.165.0", ChangeMinor, true},
		{"16.1", "17.3", ChangeMajor, false},
		{"26.2.1", "26.2.2", ChangePatch, true},
		{"v2.1.0", "v2.1.1", ChangePatch, true},
		{"v2.1.0", "v3.0.0", ChangeMajor, false},
		{"invalid", "also-invalid", ChangeUnknown, false},
	}

	for _, tt := range tests {
		gotType, gotSafe := ClassifyChange(tt.current, tt.latest)
		if gotType != tt.wantType || gotSafe != tt.wantSafe {
			t.Errorf("ClassifyChange(%q, %q) = (%v, %v); want (%v, %v)",
				tt.current, tt.latest, gotType, gotSafe, tt.wantType, tt.wantSafe)
		}
	}
}
