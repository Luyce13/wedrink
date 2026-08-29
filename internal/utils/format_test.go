package utils_test

import (
	"testing"

	"wedrink/internal/utils"
)

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{name: "zero", input: 0, expected: "0"},
		{name: "negative zero", input: -0.0, expected: "0"},
		{name: "single digit", input: 5, expected: "5"},
		{name: "two digits", input: 42, expected: "42"},
		{name: "three digits", input: 999, expected: "999"},
		{name: "four digits - thousands", input: 1000, expected: "1,000"},
		{name: "four digits with extra", input: 1234, expected: "1,234"},
		{name: "five digits - ten thousands", input: 12345, expected: "12,345"},
		{name: "six digits - hundred thousands", input: 125000, expected: "125,000"},
		{name: "seven digits - millions", input: 1250000, expected: "1,250,000"},
		{name: "eight digits", input: 12345678, expected: "12,345,678"},
		{name: "nine digits", input: 123456789, expected: "123,456,789"},
		{name: "ten digits - billions", input: 1234567890, expected: "1,234,567,890"},
		{name: "negative small", input: -45, expected: "-45"},
		{name: "negative hundreds", input: -999, expected: "-999"},
		{name: "negative thousands", input: -1000, expected: "-1,000"},
		{name: "negative hundred thousands", input: -125000, expected: "-125,000"},
		{name: "negative millions", input: -1000000, expected: "-1,000,000"},
		{name: "rounding down", input: 125000.4, expected: "125,000"},
		{name: "rounding up", input: 125000.6, expected: "125,001"},
		{name: "rounding negative down", input: -125000.4, expected: "-125,000"},
		{name: "rounding negative up", input: -125000.6, expected: "-125,001"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := utils.FormatNumber(tc.input)
			if result != tc.expected {
				t.Errorf("FormatNumber(%f) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}
