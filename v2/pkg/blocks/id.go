// Package blocks - ID generation utilities.
package blocks

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// IDGenerator provides unique ID generation.
type IDGenerator struct {
	counter uint64
}

// NewIDGenerator creates a new ID generator.
func NewIDGenerator() *IDGenerator {
	return &IDGenerator{}
}

// NanoID generates a URL-safe nanoid-style ID.
// Default length is 21 characters (126 bits of randomness).
func (g *IDGenerator) NanoID() string {
	return g.NanoIDWithLength(21)
}

// NanoIDWithLength generates a nanoid with specified length.
func (g *IDGenerator) NanoIDWithLength(length int) string {
	// URL-safe alphabet (64 chars)
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_-"

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to less random but still unique
		return g.fallbackID(length)
	}

	for i := range bytes {
		bytes[i] = alphabet[bytes[i]%64]
	}

	return string(bytes)
}

// SessionID generates a session ID with user prefix.
func (g *IDGenerator) SessionID(userID string) string {
	timestamp := time.Now().UnixNano()
	random := g.NanoIDWithLength(8)
	return fmt.Sprintf("%s_%d_%s", sanitizeForID(userID), timestamp, random)
}

// BlockID generates a block ID.
func (g *IDGenerator) BlockID() string {
	return g.NanoID()
}

// fallbackID generates a fallback ID using timestamp and counter.
func (g *IDGenerator) fallbackID(length int) string {
	ts := time.Now().UnixNano()
	count := atomic.AddUint64(&g.counter, 1)
	combined := fmt.Sprintf("%d%d", ts, count)

	// Encode to base64 and truncate
	encoded := base64.URLEncoding.EncodeToString([]byte(combined))
	encoded = strings.TrimRight(encoded, "=")

	if len(encoded) > length {
		return encoded[:length]
	}
	return encoded
}

// sanitizeForID removes characters that shouldn't be in IDs.
func sanitizeForID(s string) string {
	var result strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			result.WriteRune(c)
		}
	}
	if result.Len() == 0 {
		return "user"
	}
	return result.String()
}

// Global ID generator instance
var globalIDGen = NewIDGenerator()

// NewBlockID generates a new block ID using the global generator.
func NewBlockID() string {
	return globalIDGen.BlockID()
}

// NewSessionID generates a new session ID using the global generator.
func NewSessionID(userID string) string {
	return globalIDGen.SessionID(userID)
}

// NewNanoID generates a new nanoid using the global generator.
func NewNanoID() string {
	return globalIDGen.NanoID()
}
