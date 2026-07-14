package models

import "testing"

func TestNormalizeGPUName(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"already clean", "NVIDIA GeForce RTX 4090", "NVIDIA GeForce RTX 4090"},
		{"leading and trailing whitespace", "  NVIDIA GeForce RTX 4090\t", "NVIDIA GeForce RTX 4090"},
		{"darwin name with embedded newline", " Apple M4\n      Type+Darwin", "Apple M4 Type+Darwin"},
		{"internal whitespace runs", "A  B\n\nC", "A B C"},
		{"whitespace only", " \n\t ", ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeGPUName(c.input); got != c.expected {
				t.Fatalf("NormalizeGPUName(%q) = %q, expected %q", c.input, got, c.expected)
			}
		})
	}
}
