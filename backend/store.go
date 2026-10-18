package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// ImageStore is a deliberately simple, process-memory-only cache for
// uploaded photos. There is no database and nothing is written to disk:
// this is intentional (see the deployment guide) and means every image
// is lost on restart/redeploy and across replicas if you ever scale to
// more than one instance. It exists only long enough for LINE's servers
// to fetch the image after a push message is sent.
type storedImage struct {
	data        []byte
	contentType string
	expiresAt   time.Time
}

type ImageStore struct {
	mu     sync.RWMutex
	images map[string]storedImage
	ttl    time.Duration
}

func NewImageStore(ttl time.Duration) *ImageStore {
	s := &ImageStore{
		images: make(map[string]storedImage),
		ttl:    ttl,
	}
	go s.janitor()
	return s
}

func (s *ImageStore) Put(data []byte, contentType string) string {
	id := randomID()
	s.mu.Lock()
	s.images[id] = storedImage{
		data:        data,
		contentType: contentType,
		expiresAt:   time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
	return id
}

func (s *ImageStore) Get(id string) ([]byte, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	img, ok := s.images[id]
	if !ok || time.Now().After(img.expiresAt) {
		return nil, "", false
	}
	return img.data, img.contentType, true
}

func (s *ImageStore) janitor() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for id, img := range s.images {
			if now.After(img.expiresAt) {
				delete(s.images, id)
			}
		}
		s.mu.Unlock()
	}
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
