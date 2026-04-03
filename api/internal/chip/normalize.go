package chip

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[\s-]+`)

type Normalized struct {
	Raw              string `json:"raw"`
	Normalized       string `json:"normalized"`
	ManufacturerHint string `json:"manufacturer_hint"`
}

func Normalize(raw string) (Normalized, error) {
	if len(strings.TrimSpace(raw)) > 50 {
		return Normalized{}, fmt.Errorf("chip id must be 50 characters or fewer")
	}
	cleaned := strings.ToUpper(nonAlnum.ReplaceAllString(strings.TrimSpace(raw), ""))
	if cleaned == "" {
		return Normalized{}, fmt.Errorf("chip id is required")
	}

	var normalized string
	switch {
	case isDigits(cleaned) && len(cleaned) == 9:
		normalized = fmt.Sprintf("%015s", cleaned)
	case isDigits(cleaned) && len(cleaned) == 15:
		normalized = cleaned
	case len(cleaned) == 10 && isHex(cleaned):
		bytes, err := hex.DecodeString(cleaned)
		if err != nil {
			return Normalized{}, fmt.Errorf("invalid hex chip id")
		}
		acc := uint64(0)
		for _, b := range bytes {
			acc = (acc << 8) | uint64(b)
		}
		normalized = fmt.Sprintf("%015d", acc%1000000000000000)
	default:
		return Normalized{}, fmt.Errorf("unsupported chip id format")
	}

	return Normalized{
		Raw:              raw,
		Normalized:       normalized,
		ManufacturerHint: ManufacturerHint(cleaned, normalized),
	}, nil
}

func ManufacturerHint(raw, normalized string) string {
	if strings.HasPrefix(raw, "0A") {
		return "AVID legacy"
	}

	if len(normalized) >= 3 {
		prefix := normalized[:3]
		switch prefix {
		case "985":
			return "HomeAgain"
		case "982":
			return "24PetWatch / Allflex"
		case "981":
			return "Datamars / PetLink / Bayer ResQ"
		case "956", "900":
			return "Trovan/AKC"
		default:
			if prefix >= "900" && prefix <= "956" {
				return "Various ISO manufacturers"
			}
		}
	}

	return "Unknown manufacturer"
}

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isHex(value string) bool {
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
