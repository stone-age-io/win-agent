package tasks

import (
	"testing"
)

// TestReverseString tests the string reversal utility
func TestReverseString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple string",
			input: "hello",
			want:  "olleh",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "single character",
			input: "a",
			want:  "a",
		},
		{
			name:  "palindrome",
			input: "racecar",
			want:  "racecar",
		},
		{
			name:  "with spaces",
			input: "hello world",
			want:  "dlrow olleh",
		},
		{
			name:  "unicode characters",
			input: "hello 世界",
			want:  "界世 olleh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reverseString(tt.input)
			if got != tt.want {
				t.Errorf("reverseString() = %v, want %v", got, tt.want)
			}
		})
	}
}
