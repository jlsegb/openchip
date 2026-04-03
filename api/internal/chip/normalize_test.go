package chip

import "testing"

func TestNormalizeNineDigitLegacy(t *testing.T) {
	result, err := Normalize("123456789")
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	if result.Normalized != "000000123456789" {
		t.Fatalf("unexpected normalized value: %s", result.Normalized)
	}
	if result.Raw != "123456789" {
		t.Fatalf("unexpected raw value: %s", result.Raw)
	}
}

func TestNormalizeTenDigitHexCases(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantISO    string
		wantVendor string
	}{
		{
			name:       "all zeroes",
			input:      "0000000000",
			wantISO:    "000000000000000",
			wantVendor: "Unknown manufacturer",
		},
		{
			name:       "small value",
			input:      "000000000A",
			wantISO:    "000000000000010",
			wantVendor: "AVID legacy",
		},
		{
			name:       "mixed case",
			input:      "0a00000000",
			wantISO:    "000042949672960",
			wantVendor: "AVID legacy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Normalize(tt.input)
			if err != nil {
				t.Fatalf("Normalize returned error: %v", err)
			}
			if result.Normalized != tt.wantISO {
				t.Fatalf("normalized = %s, want %s", result.Normalized, tt.wantISO)
			}
			if result.ManufacturerHint != tt.wantVendor {
				t.Fatalf("manufacturer = %s, want %s", result.ManufacturerHint, tt.wantVendor)
			}
		})
	}
}

func TestNormalizeFifteenDigitISO(t *testing.T) {
	result, err := Normalize("985000000000123")
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if result.Normalized != "985000000000123" {
		t.Fatalf("unexpected normalized value: %s", result.Normalized)
	}
	if result.ManufacturerHint != "HomeAgain" {
		t.Fatalf("unexpected manufacturer hint: %s", result.ManufacturerHint)
	}
}

func TestNormalizeStripsSpacesDashesAndUppercasesHex(t *testing.T) {
	result, err := Normalize(" 0a-00 00-0000 ")
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if result.Normalized != "000042949672960" {
		t.Fatalf("unexpected normalized value: %s", result.Normalized)
	}
	if result.ManufacturerHint != "AVID legacy" {
		t.Fatalf("unexpected manufacturer hint: %s", result.ManufacturerHint)
	}
}

func TestNormalizeInvalidInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "too short", input: "12345678"},
		{name: "too long", input: "1234567890123456"},
		{name: "not hex", input: "0G00000000"},
		{name: "letters in numeric ISO", input: "98500000000012A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Normalize(tt.input); err == nil {
				t.Fatalf("expected error for input %q", tt.input)
			}
		})
	}
}

func TestManufacturerHintKnownPrefixes(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		normalized string
		want       string
	}{
		{name: "homeagain", raw: "985000000000001", normalized: "985000000000001", want: "HomeAgain"},
		{name: "24petwatch", raw: "982000000000001", normalized: "982000000000001", want: "24PetWatch / Allflex"},
		{name: "datamars", raw: "981000000000001", normalized: "981000000000001", want: "Datamars / PetLink / Bayer ResQ"},
		{name: "trovan 956", raw: "956000000000001", normalized: "956000000000001", want: "Trovan/AKC"},
		{name: "trovan 900", raw: "900000000000001", normalized: "900000000000001", want: "Trovan/AKC"},
		{name: "various manufacturer range", raw: "901000000000001", normalized: "901000000000001", want: "Various ISO manufacturers"},
		{name: "avid raw prefix", raw: "0A00000000", normalized: "000042949672960", want: "AVID legacy"},
		{name: "unknown", raw: "123000000000001", normalized: "123000000000001", want: "Unknown manufacturer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ManufacturerHint(tt.raw, tt.normalized)
			if got != tt.want {
				t.Fatalf("ManufacturerHint(%q, %q) = %q, want %q", tt.raw, tt.normalized, got, tt.want)
			}
		})
	}
}
