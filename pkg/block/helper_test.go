package block

import (
	"testing"
)

// TestShortenResourceName tests different variations of the ephemeral resource name.
func TestResourceNameShortener(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // We will check the length of the output
	}{
		{
			name:     "Short name (less than 64 characters)",
			input:    "ephemeral-persistent-volume-claim",
			expected: len("ephemeral-persistent-volume-claim"),
		},
		{
			name:     "Exact 64 characters",
			input:    "ephemeral-persistent-volume-claim-for-application-persistent-volume-clai",
			expected: 64,
		},
		{
			name:     "Name longer than 64 characters",
			input:    "ephemeral-persistent-volume-claim-for-application-persistent-volume-claim",
			expected: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResourceNameShortener(tt.input)
			if len(got) != tt.expected {
				t.Errorf("ShortenResourceName() = %v (length %d), expected length %d", got, len(got), tt.expected)
			}
		})
	}
}
