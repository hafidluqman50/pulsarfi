package auth

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

type nonceEntry struct {
	value  string
	expiry time.Time
}

type NonceStore struct {
	mu      sync.Mutex
	entries map[string]nonceEntry // key = lowercase wallet address
}

func NewNonceStore() *NonceStore {
	ns := &NonceStore{entries: make(map[string]nonceEntry)}
	go ns.cleanup()
	return ns
}

func (ns *NonceStore) Issue(address string) string {
	b := make([]byte, 16)
	rand.Read(b)
	nonce := hex.EncodeToString(b)

	ns.mu.Lock()
	ns.entries[strings.ToLower(address)] = nonceEntry{
		value:  nonce,
		expiry: time.Now().Add(5 * time.Minute),
	}
	ns.mu.Unlock()
	return nonce
}

// Consume validates and deletes the nonce atomically (one-time use).
func (ns *NonceStore) Consume(address, nonce string) bool {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	key := strings.ToLower(address)
	entry, ok := ns.entries[key]
	if !ok || time.Now().After(entry.expiry) || entry.value != nonce {
		return false
	}
	delete(ns.entries, key)
	return true
}

func (ns *NonceStore) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ns.mu.Lock()
		now := time.Now()
		for k, v := range ns.entries {
			if now.After(v.expiry) {
				delete(ns.entries, k)
			}
		}
		ns.mu.Unlock()
	}
}
