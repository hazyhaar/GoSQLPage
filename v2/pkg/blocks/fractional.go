// Package blocks - fractional indexing for block ordering.
// Implements Figma/Linear-style fractional indexing for O(1) insertions.
package blocks

import (
	"strings"
)

const (
	// Base characters for fractional indexing (a-z for simplicity)
	base       = "abcdefghijklmnopqrstuvwxyz"
	baseLen    = len(base)
	midChar    = 'm' // Middle character
	firstChar  = 'a'
	lastChar   = 'z'
)

// FractionalIndex provides methods for generating fractional indices.
type FractionalIndex struct{}

// NewFractionalIndex creates a new FractionalIndex helper.
func NewFractionalIndex() *FractionalIndex {
	return &FractionalIndex{}
}

// Initial returns the initial position for the first item.
func (f *FractionalIndex) Initial() string {
	return string(midChar)
}

// Before generates a position before the given position.
func (f *FractionalIndex) Before(pos string) string {
	return f.Between("", pos)
}

// After generates a position after the given position.
func (f *FractionalIndex) After(pos string) string {
	return f.Between(pos, "")
}

// Between generates a position between two positions.
// If before is empty, generates position before after.
// If after is empty, generates position after before.
func (f *FractionalIndex) Between(before, after string) string {
	// Handle edge cases
	if before == "" && after == "" {
		return f.Initial()
	}

	if before == "" {
		// Insert before 'after'
		return f.decrementPosition(after)
	}

	if after == "" {
		// Insert after 'before'
		return f.incrementPosition(before)
	}

	// Generate position between before and after
	return f.midpoint(before, after)
}

// midpoint calculates a position between two positions.
func (f *FractionalIndex) midpoint(before, after string) string {
	// Pad strings to same length
	maxLen := max(len(before), len(after))
	before = padRight(before, maxLen, firstChar)
	after = padRight(after, maxLen, lastChar)

	result := strings.Builder{}

	for i := 0; i < maxLen; i++ {
		bc := before[i]
		ac := after[i]

		if bc == ac {
			result.WriteByte(bc)
			continue
		}

		// Find midpoint character
		mid := (int(bc) + int(ac)) / 2
		if mid == int(bc) {
			// Characters are adjacent, need to extend
			result.WriteByte(bc)
			// Append midpoint of remaining
			result.WriteByte(midChar)
			return result.String()
		}

		result.WriteByte(byte(mid))
		return result.String()
	}

	// Strings are equal (shouldn't happen), append midpoint
	result.WriteByte(midChar)
	return result.String()
}

// decrementPosition generates a position before the given position.
func (f *FractionalIndex) decrementPosition(pos string) string {
	if pos == "" {
		return f.Initial()
	}

	// Try to decrement the last character
	runes := []byte(pos)
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] > firstChar {
			runes[i]--
			// Add midpoint suffix if we'd collide
			if i == len(runes)-1 && runes[i] == firstChar {
				return string(runes) + string(midChar)
			}
			return string(runes)
		}
		// Continue to previous character
	}

	// All characters are 'a', prepend 'a' and use midpoint
	return string(firstChar) + string(midChar)
}

// incrementPosition generates a position after the given position.
func (f *FractionalIndex) incrementPosition(pos string) string {
	if pos == "" {
		return f.Initial()
	}

	// Try to increment the last character
	runes := []byte(pos)
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] < lastChar {
			runes[i]++
			return string(runes)
		}
		// Continue to previous character
	}

	// All characters are 'z', append midpoint
	return pos + string(midChar)
}

// padRight pads a string with the given character to reach the desired length.
func padRight(s string, length int, pad byte) string {
	if len(s) >= length {
		return s
	}
	b := make([]byte, length)
	copy(b, s)
	for i := len(s); i < length; i++ {
		b[i] = pad
	}
	return string(b)
}

// Compare compares two positions lexicographically.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func (f *FractionalIndex) Compare(a, b string) int {
	return strings.Compare(a, b)
}

// ValidateOrder checks if positions are in ascending order.
func (f *FractionalIndex) ValidateOrder(positions []string) bool {
	for i := 1; i < len(positions); i++ {
		if f.Compare(positions[i-1], positions[i]) >= 0 {
			return false
		}
	}
	return true
}
