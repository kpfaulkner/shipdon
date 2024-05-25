package ui

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

// Image cache, can be used for avatars of embedded media
type StatusStateCache struct {
	lock  sync.RWMutex
	cache map[int64]StatusStateCacheEntry
}

func NewStatusStateCache() *StatusStateCache {
	sc := &StatusStateCache{
		cache: make(map[int64]StatusStateCacheEntry),
	}

	// start a goroutine to flush old entries
	go func() {
		for {
			sc.FlushOldEntries()
			time.Sleep(2 * time.Minute)
		}
	}()
	return sc
}

// Use Lock/UnLock due to updating last used time.
func (sc *StatusStateCache) Get(key int64) *StatusStateCacheEntry {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	entry, ok := sc.cache[key]
	if !ok {
		return nil
	}
	entry.lastUsed = time.Now()
	sc.cache[key] = entry
	return &entry
}

func (sc *StatusStateCache) Set(key int64, entry StatusStateCacheEntry) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	sc.cache[key] = entry
}

func (sc *StatusStateCache) FlushOldEntries() {

	sc.lock.Lock()
	defer sc.lock.Unlock()
	fmt.Printf("StatusStateCache had %d entries.\n", len(sc.cache))
	for k, v := range sc.cache {
		if time.Since(v.lastUsed) > 10*time.Minute {
			log.Debugf("Deleting from imagecache %s\n", k)
			delete(sc.cache, k)
		}
	}
	fmt.Printf("StatusStateCache now has %d entries.\n", len(sc.cache))
}
