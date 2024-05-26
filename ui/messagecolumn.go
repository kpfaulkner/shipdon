package ui

import (
	"gioui.org/font/gofont"
	"gioui.org/unit"
	"github.com/kpfaulkner/shipdon/events"
	mastodon2 "github.com/kpfaulkner/shipdon/mastodon"
	"github.com/mattn/go-mastodon"
	"golang.org/x/exp/shiny/materialdesign/icons"
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"git.sr.ht/~gioverse/skel/stream"
	log "github.com/sirupsen/logrus"
)

var fonts = gofont.Collection()

const (
	HomeColumn ColumnType = iota
	NotificationsColumn
	HashTagColumn
	ListColumn
	UserColumn
	ThreadColumn
	SearchColumn
)

// Define some convenient type aliases to make some things more concise.
type (
	C = layout.Context
	D = layout.Dimensions
)

type ColumnType int

// ComponentState wraps the two things any UI component needs to use reactive
// UI state.
type ComponentState struct {
	controller *stream.Controller
	backend    *mastodon2.MastodonBackend
}

func NewComponentState(controller *stream.Controller, backend *mastodon2.MastodonBackend) ComponentState {
	return ComponentState{
		controller: controller,
		backend:    backend,
	}
}

type StatusStateCacheEntry struct {
	statusState *StatusState
	lastUsed    time.Time
}

// MessageColumn defines the top-level presentation of your UI.
type MessageColumn struct {
	// th configures styling of widgets.
	th *ShipdonTheme
	ComponentState
	gtx C

	// type of column...  hashtag, list, home, notifications
	columnType ColumnType

	// Columns are for viewing messages timelines.
	// Used for display
	timelineName string

	// unique ID that identifies the timeline.
	// Might be "home", "notifications"... or something like !42 (a list) or #whatever (a hashtag)
	timelineID string

	// cache of StatusStates with key as statusid. See if we benefit from reuse.
	statusStateCache map[mastodon.ID]StatusStateCacheEntry

	// list of status states to display
	statusStateList []*StatusState
	statusList      widget.List

	// if now is later than timeStampForRefresh, then refresh via mastodon call.
	timeStampForRefresh time.Time

	//messages []madon.Status

	// indicate maximum status to display in column
	maxStatusToDisplay int

	recentlyIncreasedLimit bool

	removeColumnButton widget.Clickable

	icon *widget.Icon
}

// NewMessageColumn builds a messageColumns using a controller and backend.
func NewMessageColumn(componentState ComponentState, timelineName string, timelineID string, columnType ColumnType, th *ShipdonTheme) *MessageColumn {
	// Build a default theme.
	// Try to select a decent system font.
	th.Face = `Segoe UI, SF Pro, Dejavu Sans, Roboto, Noto Sans, sans-serif`
	p := &MessageColumn{
		th:                  th,
		ComponentState:      componentState,
		timelineName:        timelineName,
		timelineID:          timelineID,
		timeStampForRefresh: time.Now().Add(-10 * time.Second),
		maxStatusToDisplay:  20,
		statusStateCache:    make(map[mastodon.ID]StatusStateCacheEntry),
	}

	p.statusList.List.Axis = layout.Vertical
	ic, err := widget.NewIcon(icons.NavigationCancel)
	if err != nil {
		log.Fatal(err)
	}
	p.icon = ic
	p.columnType = columnType

	return p
}

// Layout builds your UI within the operation list in gtx.
func (p *MessageColumn) Layout(gtx C) D {

	// Make sure width is 400 (arbitrary for now)... but the height is taken from gtx passed in.
	// This appears to be the height of the parent.
	sideLength := gtx.Dp(400)
	gtx.Constraints.Min = image.Point{X: sideLength, Y: gtx.Constraints.Max.Y}
	gtx.Constraints.Max = image.Point{X: sideLength, Y: gtx.Constraints.Max.Y}

	haveRemoveButton := true
	if p.timelineName == "home" || p.timelineName == "notifications" {
		haveRemoveButton = false
	}

	// if no search results, then no columne
	if p.timelineName == "search" {
		messages, err := p.backend.GetTimeline(p.timelineID)
		if err != nil {
			log.Errorf("unable to get timeline for %s: %s", p.timelineName, err)
		}

		if len(messages) == 0 {
			return D{
				Size:     image.Point{0, 0},
				Baseline: 0,
			}
		}

	}

	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutHeader(gtx, haveRemoveButton)
		}),
		layout.Flexed(1, p.layoutStatusList),
	)
}

// layoutHeader displays a simple top bar.
func (p *MessageColumn) layoutHeader(gtx C, haveRemoveButton bool) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			paint.FillShape(gtx.Ops, p.th.ContrastBg, clip.Rect{Max: gtx.Constraints.Min}.Op())
			return D{Size: gtx.Constraints.Min}
		}),

		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			in := layout.UniformInset(unit.Dp(1))

			// Flex lets you lay out widgets in Horizontal or vertical lines with
			// many options for how to use and partition the space.
			// All "rigid" children are allocated space first, and then any
			// remaining space is either divided among "flexed" children
			// according to their weights OR divided by the Spacing strategy
			// if there are no flexed children.
			return layout.Flex{
				// Lay out the children sequentially horizontally.
				Axis: layout.Horizontal,
			}.Layout(gtx,

				// This flexed child has a weight of 1, and there are no
				// other flexed children, this means it gets all of the
				// space not occupied by rigid children.
				layout.Flexed(1, func(gtx C) D {
					return layout.UniformInset(12).Layout(gtx, func(gtx C) D {
						l := material.H6(&p.th.Theme, p.timelineName)
						l.Color = p.th.ContrastFg
						return l.Layout(gtx)
					})
				}),

				// add remove column button
				layout.Rigid(func(gtx C) D {
					if !haveRemoveButton {
						return layout.Dimensions{
							Size:     image.Point{},
							Baseline: 0,
						}
					}
					return in.Layout(gtx, material.IconButton(&p.th.Theme, &p.removeColumnButton, p.icon, "Remove Column").Layout)
				}),
			)
		}),
	)
}

// layoutStatusList displays a list of messages
// Need to provide a maximum until figure out how to extend.
func (p *MessageColumn) layoutStatusList(gtx C) D {

	var err error
	messages, err := p.backend.GetTimeline(p.timelineID)
	if err != nil {
		log.Errorf("unable to get timeline for %s: %s", p.timelineName, err)
		material.Body1(&p.th.Theme, err.Error()).Layout(gtx)
	}

	if len(messages) == 0 {
		return D{
			Size:     image.Point{400, 600},
			Baseline: 0,
		}
	}

	p.statusStateList = []*StatusState{}

	// any that are not in statusStateCache, add them.
	for _, status := range messages {
		if s, ok := p.statusStateCache[status.ID]; !ok {
			newStatusState := NewStatusState(p.ComponentState, p.th)
			newStatusState.syncStatusToUI(status, gtx)
			p.statusStateCache[status.ID] = StatusStateCacheEntry{
				statusState: newStatusState,
			}
		} else {

			// make sure updates have occured, such as likes, boosts, etc.
			s.statusState.status = status
		}

		s := p.statusStateCache[status.ID]
		s.lastUsed = time.Now()
		p.statusStateCache[status.ID] = s

		// update images since they might have been downloaded since last time
		p.statusStateCache[status.ID].statusState.Avatar = generateAvatar(status)
		media, url := generateMedia(status)

		// storage widget.Image for later use
		if media != nil {
			p.statusStateCache[status.ID].statusState.img = media
			p.statusStateCache[status.ID].statusState.imgOrigURL = url
		} else {
			// make sure to clear out imgWidget...
			p.statusStateCache[status.ID].statusState.img = nil
			p.statusStateCache[status.ID].statusState.imgOrigURL = ""
		}

		p.statusStateList = append(p.statusStateList, p.statusStateCache[status.ID].statusState)
	}

	// prune cache entries not used.
	for k, v := range p.statusStateCache {
		if time.Since(v.lastUsed) > 10*time.Minute {
			delete(p.statusStateCache, k)
		}
	}

	paint.FillShape(gtx.Ops, p.th.StatusBackgroundColour, clip.Rect{Max: gtx.Constraints.Max}.Op())
	listStyle := material.List(&p.th.Theme, &p.statusList)
	listStyle.AnchorStrategy = material.Overlay

	ls := listStyle.Layout(gtx, len(p.statusStateList), func(gtx C, index int) D {

		// if we're at the end of the list, then get older.
		// except if the list is less than 40...  then we're seeing all there is.
		// FIXME(kpfaulkner) strengthen logic here.
		if (len(p.statusStateList)-20 < index) && len(p.statusStateList) > 30 {
			log.Debugf("retrieve older status updates")
			// cause messages to get refreshed...
			events.FireEvent(events.NewGetOlderRefreshEvents(p.timelineID, getRefreshTypeForColumnType(p.columnType)))
		} else {

			// if we've scrolled and have a tasklist thats greater than visible (assumption) but drawing the first one
			// then refresh.
			if len(p.statusStateList) > 40 && index == 0 {
				events.FireEvent(events.NewRefreshEvent(p.timelineID, true, getRefreshTypeForColumnType(p.columnType)))
			}

		}

		const baseInset = unit.Dp(12)
		inset := layout.Inset{
			Left:   baseInset,
			Right:  baseInset,
			Top:    baseInset * .5,
			Bottom: baseInset * .5,
		}
		if index == 0 {
			inset.Top = baseInset
		}
		if index == len(p.statusStateList)-1 {
			inset.Bottom = baseInset
		}
		return inset.Layout(gtx, NewStatusStyle(&p.th.Theme, p.statusStateList[index]).Layout)
	})

	return ls
}

func getRefreshTypeForColumnType(columnType ColumnType) events.RefreshType {
	switch columnType {
	case HomeColumn:
		return events.HOME_REFRESH
	case ListColumn:
		return events.LIST_REFRESH
	case NotificationsColumn:
		return events.NOTIFICATION_REFRESH
	case HashTagColumn:
		return events.HASHTAG_REFRESH
	case UserColumn:
		return events.USER_REFRESH
	}
	return events.LIST_REFRESH
}