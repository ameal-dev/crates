package analyzer

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

const cacheTTL = 24 * time.Hour

type cacheEntry struct {
	matches   []ConceptMatch
	createdAt time.Time
}

// Cache stores diff classification results keyed by SHA-256 of the diff.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

// NewCache creates a new classification cache.
func NewCache() *Cache {
	return &Cache{entries: make(map[string]cacheEntry)}
}

// Get retrieves cached matches for a diff, returning nil if not found or expired.
func (c *Cache) Get(diff string) []ConceptMatch {
	key := hashDiff(diff)
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil
	}
	if time.Since(entry.createdAt) > cacheTTL {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil
	}
	result := make([]ConceptMatch, len(entry.matches))
	copy(result, entry.matches)
	return result
}

// Set stores classification results for a diff.
func (c *Cache) Set(diff string, matches []ConceptMatch) {
	key := hashDiff(diff)
	stored := make([]ConceptMatch, len(matches))
	copy(stored, matches)
	c.mu.Lock()
	c.entries[key] = cacheEntry{matches: stored, createdAt: time.Now()}
	c.mu.Unlock()
}

func hashDiff(diff string) string {
	h := sha256.Sum256([]byte(diff))
	return hex.EncodeToString(h[:])
}
