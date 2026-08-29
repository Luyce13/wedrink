package utils

import (
	"math"
	"strconv"
	"strings"
)

// FormatNumber formats a float64 as a comma-separated integer string with comma after every 3 digits.
// Values are rounded to the nearest integer.
// Examples:
//
//	0 -> "0"
//	999 -> "999"
//	1000 -> "1,000"
//	125000 -> "125,000"
//	1250000 -> "1,250,000"
//	-125000 -> "-125,000"
func FormatNumber(val float64) string {
	rounded := math.Round(val)
	n := int64(math.Abs(rounded))
	s := strconv.FormatInt(n, 10)
	l := len(s)
	if l <= 3 {
		if rounded < 0 && n != 0 {
			return "-" + s
		}
		return s
	}

	var parts []string
	rem := l % 3
	if rem > 0 {
		parts = append(parts, s[:rem])
	}
	for i := rem; i < l; i += 3 {
		parts = append(parts, s[i:i+3])
	}
	res := strings.Join(parts, ",")
	if rounded < 0 && n != 0 {
		return "-" + res
	}
	return res
}
