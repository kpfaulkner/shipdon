package ui

import (
	"context"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"math/rand"
	"net/http"
	"path"
	"slices"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/richtext"
	"git.sr.ht/~gioverse/skel/stream"
	"github.com/inkeliz/giohyperlink"
	"github.com/kpfaulkner/shipdon/config"
	"github.com/kpfaulkner/shipdon/events"
	mastodon2 "github.com/kpfaulkner/shipdon/mastodon"
	"github.com/mattn/go-mastodon"
	log "github.com/sirupsen/logrus"
)

type ShipdonTheme struct {
	material.Theme

	LinkColour             color.NRGBA
	IconBackgroundColour   color.NRGBA
	TitleBackgroundColour  color.NRGBA
	StatusBackgroundColour color.NRGBA

	// Actual icon such as reply, star, boost etc when action has been performed (ie the status has been boosted, fav etc)
	IconActiveColour color.NRGBA

	BoostedColour color.NRGBA

	// Similarly icon colour if nothing has been done.
	IconInactiveColour color.NRGBA
}

type ImageDownloadDetails struct {
	request ImageDetails
	img     image.Image
}

type ImageDetails struct {
	name string
	url  string

	// resize for avatar but not for images in toots.
	resize bool
	width  int
	height int
}

// ugly globals
var (
	defaultImage  = makeBlankImage()
	defaultAvatar = widget.Image{
		Src: paint.NewImageOp(defaultImage),
	}

	imageCache   = NewImageCache()
	imageChannel = make(chan ImageDetails, 1000)
)

// UI defines the state of a single application window's UI.
type UI struct {
	parentCtx      context.Context
	controller     *stream.Controller
	cancel         context.CancelFunc
	w              *app.Window
	composeColumn  *ComposeColumn
	messageColumns []*MessageColumn
	columnList     widget.List

	// Fields for easy runtime tracing.
	tracing   bool
	traceDone context.CancelFunc

	backend *mastodon2.MastodonBackend

	eventListener *events.EventListener

	// Theme used... will be determined by config
	th  *ShipdonTheme
	cfg *config.Config
}

func NewUI(
	controller *stream.Controller,
	w *app.Window,
	composeColumn *ComposeColumn,
	messageColumns []*MessageColumn,
	backend *mastodon2.MastodonBackend,
	eventListener *events.EventListener,
	cfg *config.Config) *UI {
	ui := &UI{
		controller:     controller,
		w:              w,
		composeColumn:  composeColumn,
		messageColumns: messageColumns,
		backend:        backend,
		eventListener:  eventListener,
		cfg:            cfg,
	}

	ui.columnList.List.Axis = layout.Horizontal

	if cfg.DarkMode {
		ui.th = GenerateDarkTheme()
	} else {
		ui.th = GenerateLightTheme()
	}

	return ui
}

// Run executes the window event loop.
func (u *UI) Run() error {
	// Ensure that our window-scoped context is cancelled when our window closes.
	defer u.cancel()
	var ops op.Ops
	var inset = layout.UniformInset(8)

	err := u.backend.LoginWithOAuth2()
	if err != nil {
		log.Warningf("unable to login with OAuth. %v", err)

		instanceURL := openInstanceWindow()
		if instanceURL != "" {
			instanceLoginURL, err := u.backend.GenerateOAuthLoginURL(instanceURL)
			if err != nil {
				// FIXME(kpfaulkner) panic for now.. handle nicely later.
				panic("unable to query instance URL")
			}
			if err := giohyperlink.Open(instanceLoginURL); err != nil {
				log.Debugf("error: opening hyperlink: %v", err)
			}
		} else {
			// FIXME(kpfaulkner) panic for now.. handle nicely later.
			panic("unable to query instance URL")
		}

		// open browser to get code.
		code := openLoginWindow()
		if code != "" {
			u.backend.GenerateConfigWithCode(code)
		} else {
			// FIXME(kpfaulkner) panic for now.. handle nicely later.
			panic("bad code")
		}
		log.Debugf("CODE is %s\n", code)
	}

	// separate go routine for downloading of images
	go downloadImages(imageChannel)

	// get list from Mastodon, then create correct number of message columns
	u.messageColumns = u.generateMessageColumns()

	// regular refresh of all columns
	go func() {
		for {

			for _, col := range u.messageColumns {
				if col.shouldNotAutoRefresh {
					continue
				}
				// put random delays in so we're not hammering the server all at once.
				go func(col *MessageColumn) {
					time.Sleep(time.Duration(rand.Intn(5000)) * time.Millisecond)
					events.FireEvent(events.NewRefreshEvent(col.timelineID, true, getRefreshTypeForColumnType(col.columnType)))
				}(col)
			}

			// fire off a refresh every minute.
			time.Sleep(1 * time.Minute)
		}
	}()

	// regular image cache expiry
	// start a goroutine to flush old entries and invalidate the window so it will refresh.
	go func() {
		for {
			//imageCache.PrintStats()
			imageCache.FlushOldEntries()
			u.delayInvalidate(1)
			time.Sleep(5 * time.Minute)
		}
	}()

	// Iterate events from our Gio window.
	for {
		ev := u.w.Event()
		switch ev := ev.(type) {
		case app.DestroyEvent:
			// If we get a destroy event, the window is closing. This may be due to
			// the user manually closing it, or the OS forcing it to close. Either way,
			// we return to end the event loop.
			return ev.Err
		case app.ConfigEvent:
			//decorated = ev.Config.Decorated
		case app.FrameEvent:

			// If we get a frame event, it's time to display a new frame. First, we
			// build a layout context using our operation list and info from the frame
			// event.
			gtx := app.NewContext(&ops, ev)

			paint.FillShape(gtx.Ops, u.th.StatusBackgroundColour, clip.Rect{Max: gtx.Constraints.Max}.Op())

			err := u.handleComposeColumnEvents(gtx)
			if err != nil {
				log.Errorf("error handling compose column events %+v", err)
			}

			err = u.handleMessageColumnEvents(gtx)
			if err != nil {
				log.Errorf("error handling message column events %+v", err)
			}

			inset.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {

					fc := layoutAllMessageCols(u.messageColumns)
					fc = append([]layout.FlexChild{layout.Flexed(1, u.composeColumn.Layout)}, fc...)

					th := material.NewTheme()
					listStyle := material.List(th, &u.columnList)
					listStyle.AnchorStrategy = material.Overlay
					ls := listStyle.Layout(gtx, len(fc), func(gtx C, index int) D {

						return layout.Flex{
							Axis: layout.Horizontal,
							//Alignment: layout.Middle,
						}.Layout(gtx, fc[index])
					})

					return ls

				})

			ev.Frame(gtx.Ops)
			u.controller.Sweep()

		}
	}
	return nil
}

// generateMessageColumns queries mastodon for list names
// then generates the right number of columns (including home and notifications)
func (u *UI) generateMessageColumns() []*MessageColumn {
	var columns []*MessageColumn

	// search, home and notifications are special cases.
	searchColumn := NewMessageColumn(NewComponentState(u.controller, u.backend), "search", "search", SearchColumn, u.th)
	searchColumn.shouldNotAutoRefresh = true
	columns = append(columns, searchColumn)
	columns = append(columns, NewMessageColumn(NewComponentState(u.controller, u.backend), "home", "home", HomeColumn, u.th))
	columns = append(columns, NewMessageColumn(NewComponentState(u.controller, u.backend), "notifications", "notifications", NotificationsColumn, u.th))

	lists, err := u.backend.GetLists()
	if err != nil {
		log.Errorf("unable to get lists %v", err)
		return columns
	}

	// should be a map, but leave as slice for now.
	for _, l := range lists {
		//columnID := fmt.Sprintf("!%s", l.ID)
		columnID := string(l.ID)
		if !slices.Contains(u.cfg.ListsToNotDisplay, columnID) {
			columns = append(columns, NewMessageColumn(NewComponentState(u.controller, u.backend), l.Title, columnID, ListColumn, u.th))
		}
	}

	return columns
}

// addNewHashTagColumn adds a new column for a hashtag if one doesn't already exist
// Also will have to start polling for new content.
func (u *UI) addNewHashTagColumn(tag string) {

	columnAlreadyExists := false
	// checks that username column doesn't already exist.
	for _, c := range u.messageColumns {
		if c.timelineID == tag {
			columnAlreadyExists = true
			break
		}
	}

	if !columnAlreadyExists {
		u.messageColumns = append(u.messageColumns, NewMessageColumn(NewComponentState(u.controller, u.backend), tag, tag, HashTagColumn, u.th))
		events.FireEvent(events.NewRefreshEvent(tag, true, events.HASHTAG_REFRESH))
	}
}

// addNewUsernameColumn adds a new column for a username if one doesn't already exist
// Also will have to start polling for new content.
func (u *UI) addNewUsernameColumn(username string, userID string) {

	columnAlreadyExists := false
	// checks that username column doesn't already exist.
	for _, c := range u.messageColumns {
		if c.timelineID == userID {
			columnAlreadyExists = true
			break
		}
	}

	if !columnAlreadyExists {
		u.messageColumns = append(u.messageColumns, NewMessageColumn(NewComponentState(u.controller, u.backend), username, userID, UserColumn, u.th))
		events.FireEvent(events.NewRefreshEvent(userID, true, events.USER_REFRESH))
	}
}

func (u *UI) createColumnForThreadWithStatus(status mastodon.Status) {
	if status.InReplyToID != nil {
		statusID := string(status.ID)
		u.messageColumns = append(u.messageColumns, NewMessageColumn(NewComponentState(u.controller, u.backend), "thread", statusID, ThreadColumn, u.th))
		events.FireEvent(events.NewRefreshEvent(statusID, true, events.THREAD_REFRESH))
	}
}

func (u *UI) handleComposeColumnEvents(gtx layout.Context) error {
	_, ok := u.composeColumn.cancelButton.Update(gtx)
	if ok {
		u.composeColumn.replyStatusID = "0"
		u.composeColumn.postTootDetails.SetText("")
	}

	if u.composeColumn.cancelButton.Hovered() {
		log.Debugf("CANCEL HOVERED")
	}

	_, ok = u.composeColumn.postTootButton.Update(gtx)
	if ok {
		err := u.backend.Post(u.composeColumn.postTootDetails.Text(), u.composeColumn.replyStatusID)
		if err != nil {
			log.Errorf("error posting %+v", err)
		}

		// clear out toot and any reply details.
		u.composeColumn.postTootDetails.SetText("")
		u.composeColumn.replyStatusID = "0"
	}

	_, ok = u.composeColumn.settingsButton.Update(gtx)
	if ok {
		openSettingsWindow(u.cfg)
	}

	_, ok = u.composeColumn.refreshButton.Update(gtx)
	if ok {
		for _, col := range u.messageColumns {
			events.FireEvent(events.NewRefreshEvent(col.timelineID, false, getRefreshTypeForColumnType(col.columnType)))
		}

		// totally unscientific... but sleep a little then refresh :)
		u.delayInvalidate(2)
	}

	performSearch := false
	// submit search via return/enter
	if ev, ok := u.composeColumn.searchQuery.Update(gtx); ok {
		if _, ok := ev.(widget.SubmitEvent); ok {
			performSearch = true
		}
	}

	_, ok = u.composeColumn.searchButton.Update(gtx)
	if ok {
		performSearch = true
	}

	if performSearch {
		u.composeColumn.searchResults = nil
		log.Debugf("searching for %s", u.composeColumn.searchQuery.Text())
		res, err := u.backend.Search(u.composeColumn.searchQuery.Text())
		if err != nil {
			log.Errorf("error searching %+v", err)
			return nil
		}
		u.delayInvalidate(2)
		u.composeColumn.searchResults = res.Statuses
	}

	return nil
}

func (u *UI) handleMessageColumnEvents(gtx layout.Context) error {
	// Message Columns events
	for colNum, c := range u.messageColumns {

		_, ok := c.followClickable.Update(gtx)
		if ok {
			log.Debugf("follow/unfollow clickable for user %s", c.timelineName)
			account, relationship := c.backend.GetUserDetails()
			if account != nil && relationship != nil {
				err := c.backend.ChangeFollowStatusForUserID(account.ID, !relationship.Following)
				if err != nil {
					log.Errorf("error changing follow status %+v", err)
					return err
				}
				c.backend.RefreshUserRelationship()
				log.Debugf("invalidating UI")
				u.delayInvalidate(2)
			}
		}

		_, ok = c.removeColumnButton.Update(gtx)
		if ok {
			log.Debugf("remove column  %s", c.timelineID)

			if c.columnType == ListColumn {
				u.cfg.ListsToNotDisplay = append(u.cfg.ListsToNotDisplay, c.timelineID)
				u.cfg.Save()
			}

			if c.columnType == SearchColumn {
				// for search column we dont want to really remove it, but merely clear the search results
				// which will mean the column wont be displayed.
				if err := c.backend.ClearSearch(); err != nil {
					log.Errorf("error clearing search %+v", err)
				}
				u.composeColumn.searchQuery.SetText("")
				u.delayInvalidate(2)
			} else {
				// actually remove column
				if len(u.messageColumns) == colNum {
					u.messageColumns = u.messageColumns[:colNum]
				} else {
					u.messageColumns = append(u.messageColumns[:colNum], u.messageColumns[colNum+1:]...)
				}
			}
		}

		for _, t := range c.statusStateList {

			if t.img != nil {
				_, ok := t.imgButton.Update(gtx)
				if ok {
					log.Debugf("Opening image %+v\n", t.imgOrigURL)
					go openNewWindow(t.imgOrigURL)
				}
			}

			o, event, ok := t.Details.Update(gtx)
			if ok {
				switch event.Type {
				case richtext.Click:
					if url, ok := o.Get("url").(string); ok && url != "" {
						if err := giohyperlink.Open(url); err != nil {
							log.Debugf("error: opening hyperlink: %v", err)
						}
					}

					if tag, ok := o.Get("tag").(string); ok && tag != "" {
						log.Debugf("tag clicked %s\n", tag)
						u.addNewHashTagColumn(tag)
						//events.FireEvent(events.NewRefreshEvent(tag, false))
					}

					if username, ok := o.Get("username").(string); ok && username != "" {
						if userID, ok := o.Get("userID").(mastodon.ID); ok && userID != "" {
							log.Debugf("username clicked %s : %s\n", username, userID)
							u.addNewUsernameColumn(username, string(userID))
						}
					}

				}
			}

			o, event, ok = t.Name.Update(gtx)
			if ok {
				switch event.Type {
				case richtext.Click:

					if username, ok := o.Get("username").(string); ok && username != "" {
						if userID, ok := o.Get("userID").(mastodon.ID); ok && userID != "" {
							log.Debugf("username clicked %s : %s\n", username, userID)
							u.addNewUsernameColumn(username, string(userID))
						}
					}
				}
			}

			_, ok = t.BoostButton.Update(gtx)
			if ok {
				log.Debugf("boost for toot %+v\n", t.status.ID)
				rebloggedStatus := t.status.Reblogged.(bool)
				t.backend.Boost(t.status.ID, !rebloggedStatus)
			}

			_, ok = t.ReplyButton.Update(gtx)
			if ok {
				log.Debugf("reply for toot %+v\n", t.status.ID)
				u.composeColumn.postTootDetails.SetText(fmt.Sprintf("@%s ", t.status.Account.Acct))
				u.composeColumn.replyStatusID = t.status.ID
			}

			_, ok = t.FavouriteButton.Update(gtx)
			if ok {
				log.Debugf("favourite for toot %+v\n", t.status.ID)

				favStatus := t.status.Favourited.(bool)
				// wont display change until refresh happens...
				t.backend.SetFavourite(t.status.ID, !favStatus)
			}

			_, ok = t.ViewThreadButton.Update(gtx)
			if ok {
				log.Debugf("viewing threadfor toot %+v\n", t.status.ID)

				if t.status.InReplyToID != nil {
					u.createColumnForThreadWithStatus(t.status)
				}
			}

		}
	}
	return nil
}

func (u *UI) delayInvalidate(seconds int) {
	go func() {
		time.Sleep(time.Duration(seconds) * time.Second)
		u.w.Invalidate()
	}()
}

func layoutAllMessageCols(columns []*MessageColumn) []layout.FlexChild {

	var fc []layout.FlexChild
	for _, c := range columns {
		fc = append(fc, layout.Flexed(1, c.Layout))
	}

	return fc
}

// downloadImage is used to download a single image and return it via a channel
func downloadImage(url string) (*image.Image, error) {

	start := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	ext := path.Ext(url)
	var img image.Image
	switch ext {
	case ".gif":
		img, err = gif.Decode(resp.Body)
		if err != nil {
			return nil, err
		}
	default:
		img, _, err = image.Decode(resp.Body)
		if err != nil {
			return nil, err
		}
	}

	log.Debugf("download image %s : took %d ms", url, time.Now().Sub(start).Milliseconds())
	return &img, err
}
