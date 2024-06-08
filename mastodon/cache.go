package mastodon

import (
	"fmt"
	"github.com/mattn/go-mastodon"
	"slices"
	"sync"
	"time"
)

type TimelineDetails struct {
	name    string
	sinceID mastodon.ID

	// messages is a slice of status ids which can then be looked up in the messageCache.
	messages      []mastodon.ID
	lastRefreshed time.Time
}

type TimelineCache struct {

	// holds IDs of status' for a timeline
	timelineMessageCache map[string]TimelineDetails

	// holds ALL  status details (all timelines)
	messageCache map[mastodon.ID]mastodon.Status

	lock sync.RWMutex
}

func NewTimelineCache() *TimelineCache {
	return &TimelineCache{
		timelineMessageCache: make(map[string]TimelineDetails),
		messageCache:         make(map[mastodon.ID]mastodon.Status),
	}
}

func (tc *TimelineCache) PrintStats() error {

	tc.lock.RLock()

	for timeline, cache := range tc.timelineMessageCache {
		fmt.Printf("TimelineCache: timeline %s : cache items %d\n", timeline, len(cache.messages))
	}
	fmt.Printf("TimelineCache : messageCache items %d\n", len(tc.messageCache))

	tc.lock.RUnlock()
	return nil
}

func (tc *TimelineCache) ClearTimeline(timeline string) error {

	tc.lock.Lock()

	if details, ok := tc.timelineMessageCache[timeline]; ok {
		details.messages = []mastodon.ID{}
		tc.timelineMessageCache[timeline] = details
	}
	tc.lock.Unlock()
	return nil
}

func (tc *TimelineCache) AddToTimeline(timeline string, clearExisting bool, messages []mastodon.Status, shouldSort bool) error {
	var details TimelineDetails
	var ok bool

	tc.lock.RLock()

	if details, ok = tc.timelineMessageCache[timeline]; ok {
		// timeline was recently refreshed (last 10 seconds)...  leave it.
		if time.Now().Before(details.lastRefreshed.Add(time.Second * 10)) {
			tc.lock.RUnlock()
			return nil
		}
	}
	tc.lock.RUnlock()

	// if clear existing, just make sure details.messages is empty
	if clearExisting {
		details.messages = []mastodon.ID{}
	}

	tc.lock.Lock()
	for _, i := range messages {
		details.messages = append(details.messages, i.ID)
		tc.messageCache[i.ID] = i
	}
	tc.lock.Unlock()

	if shouldSort {
		slices.Sort(details.messages)
		slices.Reverse(details.messages)
	}

	if len(details.messages) > 0 {
		details.sinceID = details.messages[0]
	} else {
		details.sinceID = "0" // TODO(kpfaulkner) confirm if this is ok.
	}
	details.lastRefreshed = time.Now()

	tc.lock.Lock()
	tc.timelineMessageCache[timeline] = details
	tc.lock.Unlock()
	return nil
}

func (tc *TimelineCache) AddToMessageCache(messages []mastodon.Status) error {
	tc.lock.Lock()
	for _, i := range messages {
		tc.messageCache[i.ID] = i
	}
	tc.lock.Unlock()
	return nil
}

func (tc *TimelineCache) GetTimelineDetails(timelineID string) (TimelineDetails, bool) {
	tc.lock.RLock()
	defer tc.lock.RUnlock()

	td, ok := tc.timelineMessageCache[timelineID]
	return td, ok
}

func (tc *TimelineCache) GetAllStatusForTimeline(timelineID string) []mastodon.Status {
	tc.lock.RLock()
	defer tc.lock.RUnlock()

	td, ok := tc.timelineMessageCache[timelineID]
	if !ok {
		return nil
	}

	var statuses []mastodon.Status
	for _, id := range td.messages {
		statuses = append(statuses, tc.messageCache[id])
	}
	return statuses

}

func (tc *TimelineCache) LogCacheDetails() {

	for {
		time.Sleep(30 * time.Second)
	}
}

func (tc *TimelineCache) UpdateTimeline(timelineID string, sinceID mastodon.ID, messages []mastodon.ID) {
	tc.lock.Lock()
	defer tc.lock.Unlock()

	td := tc.timelineMessageCache[timelineID]
	td.sinceID = sinceID
	td.messages = messages
	tc.timelineMessageCache[timelineID] = td
}
