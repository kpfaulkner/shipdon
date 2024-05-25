package ui

import (
	"fmt"
	"gioui.org/widget"
	log "github.com/sirupsen/logrus"
	"image"
	"sync"
	"time"
)

type DownloadStatus int

const (
	NotProcessed DownloadStatus = iota
	Processing
	Processed
)

// Modify to be an Widget image cache. See if memory improves. FIXME(kpfaulkner) INVESTIGATE!
type ImageCacheEntry struct {
	imgWidget widget.Image
	img       image.Image
	lastUsed  time.Time
	status    DownloadStatus
}

// Image cache, can be used for avatars of embedded media
type ImageCache struct {
	lock  sync.RWMutex
	cache map[string]ImageCacheEntry
}

func NewImageCache() *ImageCache {
	ic := &ImageCache{
		cache: make(map[string]ImageCacheEntry),
	}

	return ic
}

// Use Lock/UnLock due to updating last used time.
func (c *ImageCache) Get(key string) ImageCacheEntry {
	c.lock.Lock()
	defer c.lock.Unlock()
	entry, ok := c.cache[key]
	if !ok {
		return ImageCacheEntry{status: NotProcessed}
	}
	entry.lastUsed = time.Now()
	c.cache[key] = entry
	return entry
}

func (c *ImageCache) Set(key string, entry ImageCacheEntry) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache[key] = entry
}

func (c *ImageCache) FlushOldEntries() {

	c.lock.Lock()
	defer c.lock.Unlock()
	fmt.Printf("ImageCache had %d entries.\n", len(c.cache))
	for k, v := range c.cache {
		if time.Since(v.lastUsed) > 10*time.Minute {
			log.Debugf("Deleting from imagecache %s\n", k)
			delete(c.cache, k)
		}
	}
	fmt.Printf("ImageCache now has %d entries.\n", len(c.cache))
}
