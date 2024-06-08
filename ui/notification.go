package ui

import (
	"fmt"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/richtext"
	"github.com/k3a/html2text"
	"github.com/mattn/go-mastodon"
	"image"
)

var ()

type NotificationState struct {
	ComponentState

	// needsUpdate tracks whether our widgets state is known to be ahead of the
	// current backend state, so that we need to issue a new mutation.
	needsUpdate bool
	// updateError holds the latest error from trying to update the task.
	updateError string

	// These fields hold interactive state for the user-manipulable fields of
	// a task.
	Name      richtext.InteractiveText
	NameStyle richtext.TextStyle

	// Description used to indicate boosts or favourites.
	Description      richtext.InteractiveText
	DescriptionStyle richtext.TextStyle

	Details          richtext.InteractiveText
	DetailStyle      richtext.TextStyle
	StatusToggle     widget.Clickable
	ReplyButton      widget.Clickable
	BoostButton      widget.Clickable
	FavouriteButton  widget.Clickable
	ViewThreadButton widget.Clickable

	Avatar widget.Image

	// first image (if exists) to show.
	img        *widget.Image
	imgOrigURL string
	imgButton  widget.Clickable

	//Media widget.Clickable

	targetStatus bool

	notification mastodon.Notification

	// expectingUpdate tracks whether the task has issued a mutation but has not
	// seen any results from it yet. This is useful for displaying an activity
	// indicator.
	expectingUpdate bool

	th *ShipdonTheme
}

// NewNotificationState builds an empty state.
func NewNotificationState(componentState ComponentState, th *ShipdonTheme) *NotificationState {
	return &NotificationState{
		ComponentState: componentState,
		th:             th,
	}
}

func (ss *NotificationState) generateAvatar(notification mastodon.Notification) widget.Image {
	switch notification.Type {
	case "favourite":
		// generate avatar with both current user and person who favourited.
		return generateAvatar(notification.Account, &notification.Status.Account)
	}

	return generateAvatar(notification.Account, nil)
}

func (ss *NotificationState) syncNotificationToUI(notification mastodon.Notification, gtx C) {

	usernameSpans := ss.generateNameSpanStyles(notification)
	ss.NameStyle = richtext.Text(&ss.Name, ss.th.Shaper, usernameSpans...)
	ss.Avatar = ss.generateAvatar(notification)

	detailsSpans := ss.generateDetailsSpanStyles(notification)
	ss.DetailStyle = richtext.Text(&ss.Details, ss.th.Shaper, detailsSpans...)

	ss.notification = notification

}

func (ss *NotificationState) generateDetailsSpanStyles(notification mastodon.Notification) []richtext.SpanStyle {

	spans := []richtext.SpanStyle{}

	switch notification.Type {
	case "favourite":
		spans = generateDetailsSpanStyles(*notification.Status, ss.th)
	case "update":
		spans = generateDetailsSpanStyles(*notification.Status, ss.th)
	case "poll":
		spans = generatePollSpanStyles(*notification.Status, ss.th)
	}

	return spans
}

func (ss *NotificationState) generateNameSpanStyles(notification mastodon.Notification) []richtext.SpanStyle {

	var spans []richtext.SpanStyle

	switch notification.Type {
	case "follow":
		span := richtext.SpanStyle{
			Content:     notification.Account.DisplayName,
			Color:       ss.th.Fg,
			Size:        unit.Sp(17),
			Font:        fonts[0].Font,
			Interactive: true,
		}
		span.Set("username", notification.Account.Username)
		span.Set("userID", notification.Account.ID)
		spans = append(spans, span)

		span2 := richtext.SpanStyle{
			Content: " followed you ",
			Color:   ss.th.BoostedColour,
			Size:    unit.Sp(16),
			Font:    fonts[0].Font,
		}
		spans = append(spans, span2)
	case "favourite":
		span := richtext.SpanStyle{
			Content:     notification.Account.DisplayName,
			Color:       ss.th.Fg,
			Size:        unit.Sp(17),
			Font:        fonts[0].Font,
			Interactive: true,
		}
		span.Set("username", notification.Account.Username)
		span.Set("userID", notification.Account.ID)
		spans = append(spans, span)

		span2 := richtext.SpanStyle{
			Content: " favourited your status ",
			Color:   ss.th.BoostedColour,
			Size:    unit.Sp(16),
			Font:    fonts[0].Font,
		}
		spans = append(spans, span2)
	case "poll":
		span := richtext.SpanStyle{
			Content:     notification.Account.DisplayName,
			Color:       ss.th.Fg,
			Size:        unit.Sp(17),
			Font:        fonts[0].Font,
			Interactive: true,
		}
		span.Set("username", notification.Account.Username)
		span.Set("userID", notification.Account.ID)
		spans = append(spans, span)

		span2 := richtext.SpanStyle{
			Content: " 's poll just ended",
			Color:   ss.th.BoostedColour,
			Size:    unit.Sp(16),
			Font:    fonts[0].Font,
		}
		spans = append(spans, span2)
	case "reblog":
		fmt.Printf("reblog\n")
	case "update":
		fmt.Printf("update\n")
		span := richtext.SpanStyle{
			Content:     notification.Account.DisplayName,
			Color:       ss.th.Fg,
			Size:        unit.Sp(17),
			Font:        fonts[0].Font,
			Interactive: true,
		}
		span.Set("username", notification.Account.Username)
		span.Set("userID", notification.Account.ID)
		spans = append(spans, span)

		span2 := richtext.SpanStyle{
			Content: " updated a post ",
			Color:   ss.th.BoostedColour,
			Size:    unit.Sp(16),
			Font:    fonts[0].Font,
		}
		spans = append(spans, span2)

	case "mention":
		fmt.Printf("mention\n")
	}
	return spans
}

// NotificationStyle defines the presentation of a task.
type NotificationStyle struct {
	state    *NotificationState
	Name     richtext.SpanStyle
	Details  richtext.SpanStyle
	PollList widget.List

	Loader material.LoaderStyle
	Error  material.LabelStyle
}

func NewNotificationStyle(th *material.Theme, notification *NotificationState) NotificationStyle {
	i := NotificationStyle{
		state: notification,
	}

	return i
}

// Layout inserts operations describing this task's UI into the operation
// list within gtx.
func (i NotificationStyle) Layout(gtx C) D {

	// Stack a rounded rect as the background of the status
	// Then add the name/details etc etc on top of rounded rect
	return layout.Stack{}.Layout(gtx,
		// The order child widgets are provided is from the bottom of the stack
		// to the top. Our first child is "expanded" meaning that its constraints
		// will be set to require it to be at least as large as all "stacked"
		// children. This makes it easy to build a "surface" underneath other
		// widgets.
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {

			// Rounded rects to cover a particular status
			rrect := clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(10))

			// Here is background of status areas (currently white). Need to modify
			paint.FillShape(gtx.Ops, i.state.th.Bg, rrect.Op(gtx.Ops))
			return D{Size: gtx.Constraints.Min}
		}),

		// This child is "stacked", so its size can be anything, and it will
		// determine the size of "expanded children.
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			// Flex lets you lay out widgets in Horizontal or vertical lines with
			// many options for how to use and partition the space.
			// All "rigid" children are allocated space first, and then any
			// remaining space is either divided among "flexed" children
			// according to their weights OR divided by the Spacing strategy
			// if there are no flexed children.
			return layout.Flex{
				// Lay out the children sequentially horizontally.
				Axis: layout.Horizontal,
				// Align their vertical centers across the row.
				Alignment: layout.Middle,
			}.Layout(gtx,

				// This flexed child has a weight of 1, and there are no
				// other flexed children, this means it gets all of the
				// space not occupied by rigid children.
				layout.Flexed(1, func(gtx C) D {
					return i.layoutNotification(gtx)
				}),
			)
		}),
	)
}

func (i NotificationStyle) layoutNotification(gtx C) D {

	// special case notification layout
	switch i.state.notification.Type {
	case "poll":
		return i.layoutPoll(gtx)
	}

	// default to usual
	const spacing = unit.Dp(4)
	return layout.UniformInset(spacing).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// Within it, we lay out another flex of vertically-stacked
		// widgets.
		// We manipulate the constraints to require the next widget
		// to fill all available horizontal space. This ensures that
		// the text editors can be clicked all across the row.
		gtx.Constraints.Min.X = gtx.Constraints.Max.X

		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,

			// Name is rigid... since that's going to (usually) take a fixed amount of space.
			// It's the message/details that are going to be lengthy.
			layout.Rigid(func(gtx C) D {
				return layout.Flex{
					Axis: layout.Horizontal,
					//Spacing: layout.SpaceBetween,
				}.Layout(gtx,

					// avatar?
					layout.Rigid(func(gtx C) D {
						return i.state.Avatar.Layout(gtx)
					}),

					// Name
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return i.state.NameStyle.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						sideLength := gtx.Dp(50)
						gtx.Constraints = layout.Exact(gtx.Constraints.Constrain(image.Pt(sideLength, sideLength)))
						return D{Size: gtx.Constraints.Min}
					}),
				)
			}),

			layout.Rigid(layout.Spacer{Height: spacing}.Layout),

			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// default to just layout of the details (probably a status update)
				return i.state.DetailStyle.Layout(gtx)
			}),
		)
	})
}

func (i NotificationStyle) layoutPoll(gtx C) D {

	listStyle := material.List(&i.state.th.Theme, &i.PollList)
	listStyle.AnchorStrategy = material.Overlay

	// default to usual
	const spacing = unit.Dp(4)
	return layout.UniformInset(spacing).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// Within it, we lay out another flex of vertically-stacked
		// widgets.
		// We manipulate the constraints to require the next widget
		// to fill all available horizontal space. This ensures that
		// the text editors can be clicked all across the row.
		gtx.Constraints.Min.X = gtx.Constraints.Max.X

		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,

			// Name is rigid... since that's going to (usually) take a fixed amount of space.
			// It's the message/details that are going to be lengthy.
			layout.Rigid(func(gtx C) D {
				return layout.Flex{
					Axis: layout.Horizontal,
					//Spacing: layout.SpaceBetween,
				}.Layout(gtx,

					// avatar?
					layout.Rigid(func(gtx C) D {
						return i.state.Avatar.Layout(gtx)
					}),

					// Name
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return i.state.NameStyle.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						sideLength := gtx.Dp(50)
						gtx.Constraints = layout.Exact(gtx.Constraints.Constrain(image.Pt(sideLength, sideLength)))
						return D{Size: gtx.Constraints.Min}
					}),
				)
			}),

			layout.Rigid(layout.Spacer{Height: spacing}.Layout),

			layout.Rigid(func(gtx C) D {
				content := html2text.HTML2TextWithOptions(i.state.notification.Status.Content, html2text.WithLinksInnerText())

				l := material.Label(&i.state.th.Theme, unit.Sp(15), "Poll : "+content)
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				content := ""
				for ii, o := range i.state.notification.Status.Poll.Options {
					content = content + fmt.Sprintf("%d : %s\n", ii, o.Title)
				}

				l := material.Label(&i.state.th.Theme, unit.Sp(15), content)
				return l.Layout(gtx)
			}),
		)
	})
}
