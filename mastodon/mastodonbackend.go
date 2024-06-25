package mastodon

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/kpfaulkner/shipdon/config"
	"github.com/kpfaulkner/shipdon/events"
	"github.com/mattn/go-mastodon"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	AppName    = "shipdon"
	AppWebsite = "https://github.com/kpfaulkner/shipdon"

	// get 20 messages at a time.
	MastodonLimit   = 20
	MastodonSleepMS = 100
)

var (
	seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))

	scopes      = []string{"read", "write", "follow"}
	instanceURL = "hachyderm.io"
	AccountID   mastodon.ID
)

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func String(length int) string {
	return StringWithCharset(length, charset)
}

type Notification struct {
	ID        mastodon.ID
	Type      string
	CreatedAt time.Time
	Account   mastodon.Account
	StatusID  mastodon.ID
}

type MastodonBackend struct {
	app    *mastodon.Application
	client *mastodon.Client

	// cache of messages.
	// key is timeline name
	timelineMessageCache *TimelineCache

	// keep local copy of notifications. These are specialised enough that they
	// wont need to be keeped in the cache (I hope)
	notifications []Notification

	eventListener *events.EventListener
	lock          sync.RWMutex

	// cache of lists?
	// Key is list Title
	listDetails map[string]mastodon.List

	// lastRefreshed (was stored in cache... but try local copy)
	lastRefreshed map[string]time.Time

	config *config.Config

	// used when we're investigating a specific user.
	userInfo         *mastodon.Account
	userRelationship *mastodon.Relationship

	ctx context.Context
}

func NewMastodonBackend(eventListener *events.EventListener, config *config.Config) (*MastodonBackend, error) {
	c := MastodonBackend{}
	appConfig := &mastodon.AppConfig{
		Server:       "https://hachyderm.io",
		ClientName:   "shipdon",
		Scopes:       "read write follow",
		Website:      "https://github.com/kpfaulkner/shipdon",
		RedirectURIs: "urn:ietf:wg:oauth:2.0:oob",
	}
	app, err := mastodon.RegisterApp(context.Background(), appConfig)
	if err != nil {
		log.Fatal(err)
	}

	c.app = app

	c.listDetails = make(map[string]mastodon.List)
	c.lastRefreshed = make(map[string]time.Time)

	c.timelineMessageCache = NewTimelineCache()
	c.eventListener = eventListener

	go c.timelineMessageCache.LogCacheDetails()
	c.eventListener.RegisterReceiver(events.REFRESH_MESSAGES, c.RefreshMessagesCallback)
	c.config = config
	return &c, nil
}

// LoginWithPassword logs in to Mastodon using username + password combination
// The username/password are stored in PLAIN TEXT in the config file. It is NOT recommended to use this method.
// TODO(kpfaulkner) secure password in SOME fashion.
func (c *MastodonBackend) LoginWithPassword(username string, password string) error {

	if c.config.InstanceURL == "" {
		return errors.New("missing config data")
	}

	app, err := mastodon.RegisterApp(context.Background(), &mastodon.AppConfig{
		Server:     c.config.InstanceURL,
		ClientName: "shipdon",
		Scopes:     "read write follow",
		Website:    "https://github.com/kpfaulkner/shipdon",
	})
	if err != nil {
		log.Fatal(err)
	}

	client := mastodon.NewClient(&mastodon.Config{
		Server:       c.config.InstanceURL,
		ClientID:     app.ClientID,
		ClientSecret: app.ClientSecret,
	})

	err = client.Authenticate(context.Background(), username, password)
	if err != nil {
		log.Fatal(err)
	}

	c.client = client

	return nil
}

// LoginWithOAuth2 login to Mastodon using OAuth2
// Can only log in if the config file has appID, appSecret, instance and Token info
func (c *MastodonBackend) LoginWithOAuth2() error {
	if c.config.ClientID == "" || c.config.ClientSecret == "" || c.config.InstanceURL == "" || c.config.Token == "" {
		return errors.New("missing config data")
	}

	cfg := &mastodon.Config{
		Server:       c.config.InstanceURL,
		ClientID:     c.config.ClientID,
		ClientSecret: c.config.ClientSecret,
		AccessToken:  c.config.Token,
	}

	c.client = mastodon.NewClient(cfg)
	c.ctx = context.Background()

	acct, err := c.client.GetAccountCurrentUser(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// keep track of account we're logged in with. Yes, global, yucky, but will do for now.
	AccountID = acct.ID
	return nil
}

// Logoff from Mastodon
func (c *MastodonBackend) Logoff() error {
	return nil
}

// GetThread (ie list of status which are related by replies
func (c *MastodonBackend) GetThread(statusID int64) ([]mastodon.Status, error) {
	//messages := c.timelineMessageCache.GetAllStatusForTimeline(timelineID)
	//return messages, nil
	return nil, nil
}

// clear search results.
func (c *MastodonBackend) ClearSearch() error {
	err := c.timelineMessageCache.ClearTimeline("search")
	if err != nil {
		log.Errorf("unable to clear search timeline: %s : err %s", "search", err)
		return err
	}
	return nil
}

func (c *MastodonBackend) Search(query string) (*mastodon.Results, error) {

	// default resolve to false. TODO(kpfaulkner) investigate what resolve really does (webfinger lookup)
	results, err := c.client.Search(c.ctx, query, true)
	if err != nil {
		log.Errorf("unable to search for query %s : err %s", query, err)
		return nil, err
	}

	nonPtrStatuses := []mastodon.Status{}
	for _, s := range results.Statuses {
		nonPtrStatuses = append(nonPtrStatuses, *s)
	}
	err = c.timelineMessageCache.AddToTimeline("search", true, nonPtrStatuses, true)
	if err != nil {
		log.Errorf("unable to add statuses to timelineID %s : err %s", "search", err)
		return nil, err
	}

	// shouldn't need to return results here.
	return results, nil
}

// GetTimeline get all messages for main timeline.
func (c *MastodonBackend) GetTimeline(timelineID string) ([]mastodon.Status, error) {
	messages := c.timelineMessageCache.GetAllStatusForTimeline(timelineID)
	return messages, nil
}

// ChangeFollowStatusForUserID follows (or unfollows) userID
func (c *MastodonBackend) ChangeFollowStatusForUserID(userID mastodon.ID, follow bool) error {

	if follow {
		_, err := c.client.AccountFollow(context.Background(), userID)
		if err != nil {
			return err
		}
	} else {
		_, err := c.client.AccountUnfollow(context.Background(), userID)
		if err != nil {
			return err
		}
	}
	return nil
}

// just grab notifications
func (c *MastodonBackend) GetNotifications() ([]*mastodon.Notification, error) {
	notifications := []*mastodon.Notification{}

	for _, n := range c.notifications {
		notification := &mastodon.Notification{
			ID:        n.ID,
			Type:      n.Type,
			CreatedAt: n.CreatedAt,
			Account:   n.Account,
		}
		if n.StatusID != "" {
			status := c.timelineMessageCache.messageCache[n.StatusID]
			notification.Status = &status
		}
		notifications = append(notifications, notification)
	}
	return notifications, nil
}

// Favourite a toot
func (c *MastodonBackend) SetFavourite(id mastodon.ID, fav bool) error {

	if fav {
		_, err := c.client.Favourite(c.ctx, id)
		if err != nil {
			log.Errorf("unable to favourite toot %d : err %s", id, err)
			return err
		}
	} else {
		_, err := c.client.Unfavourite(c.ctx, id)
		if err != nil {
			log.Errorf("unable to unfavourite toot %d : err %s", id, err)
			return err
		}
	}

	// set local cache?
	if s, ok := c.timelineMessageCache.messageCache[id]; ok {
		s.Favourited = fav
		c.timelineMessageCache.messageCache[id] = s
	}

	return nil
}

// Post new message to Mastodon
func (c *MastodonBackend) Post(msg string, replyStatusID mastodon.ID) error {
	status, err := c.client.PostStatus(c.ctx, &mastodon.Toot{
		Status:      msg,
		InReplyToID: replyStatusID,
	})
	if err != nil {
		log.Errorf("unable to post toot %v", err)
		return err
	}

	c.timelineMessageCache.AddToTimeline("home", false, []mastodon.Status{*status}, true)
	return nil
}

// GetLists get all the lists that we're subscribed to.
func (c *MastodonBackend) GetLists() ([]*mastodon.List, error) {

	lists, err := c.client.GetLists(c.ctx)
	if err != nil {
		log.Errorf("unable to get lists for accounterr %s", err)
		return nil, err
	}

	return lists, nil
}

// RefreshMessagesCallback refreshes the messages for a specific timeline.
// If the RefreshEvent.GetOlder is set, then we actually want to get older messages (due to user
// having scrolled back and wanting to see older messages)
// Was originally going to try and get all timelines... but will just do whatever the timeline passed in
// in the event. Will just trigger multiple events... one per timeline
func (c *MastodonBackend) RefreshMessagesCallback(e events.Event) error {
	re := e.(events.RefreshEvent)

	timelineID := re.TimelineID

	var statuses []*mastodon.Status
	var err error

	var details TimelineDetails

	if t, ok := c.lastRefreshed[timelineID]; ok {

		//if refreshed in last 10 seconds... ignore it.
		if time.Now().Before(t.Add(time.Second * 10)) {
			log.Debugf("discarding refresh event for %s due to already underway", timelineID)
			return nil
		}
	}

	c.lastRefreshed[timelineID] = time.Now()

	details, _ = c.timelineMessageCache.GetTimelineDetails(timelineID)

	params := mastodon.Pagination{Limit: MastodonLimit}

	// if we're getting older statuses, then we need to get the statuses before the oldest one we have.
	if len(details.messages) > 0 && re.GetOlder {
		params.MaxID = details.messages[len(details.messages)-1]
	} else {
		// regular get newer then prepend to existing statuses
		if !re.ClearExisting && len(details.messages) > 0 {
			params.SinceID = details.messages[0]
		}
	}

	// when added to cache, should it be sorted.
	shouldSort := true

	switch re.RefreshType {
	case events.HASHTAG_REFRESH:
		statuses, err = c.client.GetTimelineHashtag(context.Background(), timelineID, false, &params)
		if err != nil {
			log.Errorf("unable to get timelineID %s : err %s", timelineID, err)
			return nil
		}
	case events.LIST_REFRESH:
		statuses, err = c.client.GetTimelineList(context.Background(), mastodon.ID(timelineID), &params)
		if err != nil {
			log.Errorf("unable to get timelineID %s : err %s", timelineID, err)
			return nil
		}
	case events.HOME_REFRESH:
		statuses, err = c.client.GetTimelineHome(context.Background(), &params)
		if err != nil {
			log.Errorf("unable to get timelineID %s : err %s", timelineID, err)
			return nil
		}
	case events.NOTIFICATION_REFRESH:
		notifications, err := c.client.GetNotifications(context.Background(), &params)
		if err != nil {
			log.Errorf("unable to get notifications : err %s", err)
			return nil
		}

		if re.ClearExisting {
			c.notifications = []Notification{}
		}
		for _, n := range notifications {
			notification := Notification{
				ID:        n.ID,
				Type:      n.Type,
				CreatedAt: n.CreatedAt,
				Account:   n.Account,
			}

			if n.Status != nil {
				notification.StatusID = n.Status.ID
				c.timelineMessageCache.AddToMessageCache([]mastodon.Status{*n.Status})
			} else {
				notification.StatusID = ""
			}

			c.notifications = append(c.notifications, notification)
		}
		sort.Slice(c.notifications, func(i, j int) bool {
			return c.notifications[i].CreatedAt.After(c.notifications[j].CreatedAt)
		})
		return nil

	case events.USER_REFRESH:
		c.userInfo = nil
		c.userRelationship = nil
		statuses, err = c.client.GetAccountStatuses(context.Background(), mastodon.ID(re.TimelineID), &params)
		account, err := c.client.GetAccount(context.Background(), mastodon.ID(re.TimelineID))
		if err != nil {
			log.Errorf("unable to get accountID %s : err %s", re.TimelineID, err)
		} else {
			c.userInfo = account
		}
		err = c.RefreshUserRelationship()
		if err != nil {
			log.Errorf("unable to get relationship for userid %s :  %s : err %s", re.TimelineID, err)
		}

	case events.THREAD_REFRESH:
		done := false
		statusID := mastodon.ID(re.TimelineID)
		for !done {
			status, err := c.client.GetStatus(context.Background(), statusID)
			if err != nil {
				log.Errorf("unable to get statusID %s : err %s", timelineID, err)
				continue
			}
			statuses = append(statuses, status)
			if status.InReplyToID != nil {
				s := status.InReplyToID.(string)
				statusID = mastodon.ID(s)
			} else {
				done = true
			}
		}
		slices.Reverse(statuses)
		shouldSort = false
	}

	var nonPtrStatus []mastodon.Status
	for _, s := range statuses {
		nonPtrStatus = append(nonPtrStatus, *s)
	}
	err = c.timelineMessageCache.AddToTimeline(timelineID, re.ClearExisting, nonPtrStatus, shouldSort)
	if err != nil {
		log.Errorf("unable to add statuses to timelineID %s : err %s", timelineID, err)
		return err
	}

	return nil
}

// GetUserDetails is NOT the current user, but the user we've investigating (ie getting profile of).
func (c *MastodonBackend) GetUserDetails() (*mastodon.Account, *mastodon.Relationship) {
	return c.userInfo, c.userRelationship
}

func (c *MastodonBackend) RefreshUserRelationship() error {
	// based off c.userInfo, refresh the userdetails/relationship\
	relationship, err := c.client.GetAccountRelationships(context.Background(), []string{string(c.userInfo.ID)})
	if err != nil {
		log.Errorf("unable to get relationship for userid %s :  %s : err %s", c.userInfo.ID, err)
	}
	c.userRelationship = relationship[0]
	return nil
}

// Boost or unboost a toot
func (c *MastodonBackend) Boost(id mastodon.ID, boost bool) error {
	if boost {
		_, err := c.client.Reblog(context.Background(), id)
		if err != nil {
			log.Errorf("unable to boost toot %d : err %s", id, err)
			return err
		}
	} else {
		_, err := c.client.Unreblog(context.Background(), id)
		if err != nil {
			log.Errorf("unable to boost toot %d : err %s", id, err)
			return err
		}
	}

	// set local cache?
	if s, ok := c.timelineMessageCache.messageCache[id]; ok {
		s.Reblogged = boost
		c.timelineMessageCache.messageCache[id] = s
	}

	return nil
}

// GetAccountIDByUserName get the account ID by username.
func (c *MastodonBackend) GetAccountIDByUserName(username string) (interface{}, interface{}) {
	return nil, nil
}

// generateOAuthLoginURL will create a URL for the user to visit to authenticate.
func (c *MastodonBackend) GenerateOAuthLoginURL(instanceURL string) (string, error) {

	appConfig := &mastodon.AppConfig{
		Server:       instanceURL,
		ClientName:   "shipdon",
		Scopes:       "read write follow",
		Website:      "https://github.com/kpfaulkner/shipdon",
		RedirectURIs: "urn:ietf:wg:oauth:2.0:oob",
	}
	app, err := mastodon.RegisterApp(context.Background(), appConfig)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("clientID %+v\n", app.ClientID)
	c.config.ClientID = app.ClientID
	c.config.ClientSecret = app.ClientSecret
	c.config.InstanceURL = instanceURL

	// Have the user manually get the token and send it back to us
	u, err := url.Parse(app.AuthURI)
	if err != nil {
		log.Fatal(err)
	}

	return u.String(), nil
}

// generateConfigWithCode generates the config for Shipdon.
func (c *MastodonBackend) GenerateConfigWithCode(code string) error {

	cfg := &mastodon.Config{
		Server:       c.config.InstanceURL,
		ClientID:     c.config.ClientID,
		ClientSecret: c.config.ClientSecret,
		AccessToken:  code,
	}

	c.client = mastodon.NewClient(cfg)
	err := c.client.AuthenticateToken(context.Background(), code, "urn:ietf:wg:oauth:2.0:oob")
	if err != nil {
		log.Fatal(err)
	}

	// save to disk.
	c.config.Token = cfg.AccessToken
	c.writeConfigToFile()

	c.ctx = context.Background()

	acct, err := c.client.GetAccountCurrentUser(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Account is %v\n", acct)

	return nil
}

// writeConfigToFile writes the config to a file. Hardcoded for now... might tweak later.
func (c *MastodonBackend) writeConfigToFile() {
	c.config.Save()
}

func otherfunc() {
	//// Overwrite variables using Viper
	//instanceURL = viper.GetString("instance")
	//appID = viper.GetString("app_id")
	//appSecret = viper.GetString("app_secret")
	//
	//if instanceURL == "" {
	//	return errors.New("no instance provided")
	//}
	//
	//if verbose {
	//	errPrint("Instance: '%s'", instanceURL)
	//}
	//
	//if appID != "" && appSecret != "" {
	//	// We already have an app key/secret pair
	//	gClient, err = madon.RestoreApp(AppName, instanceURL, appID, appSecret, nil)
	//	if err != nil {
	//		return err
	//	}
	//	// Check instance
	//	if _, err := gClient.GetCurrentInstance(); err != nil {
	//		return errors.Wrap(err, "could not connect to server with provided app ID/secret")
	//	}
	//	if verbose {
	//		errPrint("Using provided app ID/secret")
	//	}
	//	return nil
	//}
	//
	//if appID != "" || appSecret != "" {
	//	errPrint("Warning: provided app id/secrets incomplete -- registering again")
	//}
	//
	//gClient, err = madon.NewApp(AppName, AppWebsite, scopes, madon.NoRedirect, instanceURL)
	//if err != nil {
	//	return errors.Wrap(err, "app registration failed")
	//}
	//
	//errPrint("Registered new application.")
	//return nil
}
