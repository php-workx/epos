package store

import (
	"crypto/rand"
	"fmt"
)

// GenerateID creates a random ticket ID with the format "epo-XXXX" where
// XXXX is a 4-character alphanumeric string using lowercase letters and digits.
// The ID is suitable for use as a filename within the .tickets directory.
func GenerateID(prefix string) string {
	if prefix == "" {
		prefix = "epo"
	}
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read: %v", err))
	}
	id := make([]byte, 0, len(prefix)+1+4)
	id = append(id, prefix...)
	id = append(id, '-')
	for _, c := range b {
		id = append(id, charset[c%byte(len(charset))])
	}
	return string(id)
}

// GenerateIDWithSuffix creates a human-readable slug ID from a title.
// It lowercases the title, replaces spaces and special characters with hyphens,
// and appends a short random suffix for uniqueness.
func GenerateIDWithSuffix(title string) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

	// Build a slug from the title.
	slug := make([]byte, 0, 32)
	words := false
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z':
			slug = append(slug, byte(r))
			words = true
		case r >= 'A' && r <= 'Z':
			slug = append(slug, byte(r+'a'-'A'))
			words = true
		case r >= '0' && r <= '9':
			slug = append(slug, byte(r))
			words = true
		case words && len(slug) > 0 && slug[len(slug)-1] != '-':
			slug = append(slug, '-')
			words = false
		}
	}
	// Trim trailing hyphen.
	for len(slug) > 0 && slug[len(slug)-1] == '-' {
		slug = slug[:len(slug)-1]
	}
	// Fallback when title contains no alphanumeric characters.
	if len(slug) == 0 {
		slug = []byte("ticket")
	}
	// Truncate to max 32 chars for the slug portion.
	if len(slug) > 32 {
		slug = slug[:32]
	}

	// Generate a 4-char random suffix for uniqueness.
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read: %v", err))
	}
	suffix := make([]byte, 4)
	for i, c := range b {
		suffix[i] = charset[c%byte(len(charset))]
	}

	return fmt.Sprintf("epo-%s-%s", string(slug), string(suffix))
}
