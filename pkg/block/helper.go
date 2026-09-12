package block

import (
	"crypto/sha256"
	"encoding/base64"
)

// resource name maxs lenght
const maxResourceNameLength = 64

// ResourceNameShortener shortens the resource name if it exceeds maxResourceNameLength of 64.
// It replaces the last 5 characters with the first 5 characters of a hash (base64 encoded).
func ResourceNameShortener(name string) string {
	if len(name) <= maxResourceNameLength {
		return name
	}

	// Create a SHA-256 hash of the name
	hash := sha256.Sum256([]byte(name))

	// Convert the hash to a base64 string
	hashBase64 := base64.RawURLEncoding.EncodeToString(hash[:])

	// Truncate the name and replace the last 5 characters with the first 5 characters of the hash
	truncatedName := name[:maxResourceNameLength-6] + "-" + hashBase64[:5]

	return truncatedName
}
