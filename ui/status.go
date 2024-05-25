package ui

import (
	"fmt"
	"gioui.org/x/richtext"
	"github.com/k3a/html2text"
	mastodon2 "github.com/kpfaulkner/shipdon/mastodon"
	"github.com/mattn/go-mastodon"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/shiny/materialdesign/icons"
	"golang.org/x/image/draw"
	"image"
	"image/color"
	"path"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var ()

type StatusState struct {
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

	// actual status from mastodon... a copy.
	status mastodon.Status

	// expectingUpdate tracks whether the task has issued a mutation but has not
	// seen any results from it yet. This is useful for displaying an activity
	// indicator.
	expectingUpdate bool

	th *ShipdonTheme
}

// NewStatusState builds an empty state.
func NewStatusState(componentState ComponentState, th *ShipdonTheme) *StatusState {
	return &StatusState{
		ComponentState: componentState,
		th:             th,
	}
}

// syncStatusToUI configures the widgets of the task to match the provided task
// state.
func (ss *StatusState) syncStatusToUI(status mastodon.Status, gtx C) {

	usernameSpans := ss.generateNameSpanStyles(status)
	ss.NameStyle = richtext.Text(&ss.Name, ss.th.Shaper, usernameSpans...)

	detailsSpans := ss.generateDetailsSpanStyles(status)
	ss.DetailStyle = richtext.Text(&ss.Details, ss.th.Shaper, detailsSpans...)

	ss.Avatar = generateAvatar(status)
	media, url := generateMedia(status)

	// storage widget.Image for later use
	if media != nil {
		ss.img = media
		ss.imgOrigURL = url
	} else {
		// make sure to clear out imgWidget...
		ss.img = nil
		ss.imgOrigURL = ""
	}
	ss.status = status

	// if favourited then list name of favourite.
	if status.FavouritesCount > 0 && mastodon2.AccountID == status.Account.ID {

		// if we're generating our own content... then just use acct.
		if status.Content != "" {
			span := richtext.SpanStyle{
				Content:     fmt.Sprintf("%d", status.FavouritesCount),
				Color:       ss.th.ContrastFg,
				Size:        unit.Sp(15),
				Font:        fonts[0].Font,
				Interactive: false,
			}
			ss.DescriptionStyle = richtext.Text(&ss.Description, ss.th.Shaper, span)
		}
	}
}

// generates the avatar. This may just be grabbing a cached image OR
// it could be compositing multiple images together in the case of a boost.
func generateAvatar(status mastodon.Status) widget.Image {
	if status.Reblog == nil {
		return loadAvatar(status.Account.Username, status.Account.Avatar)
	}

	// mergedKey is used for caching the composite image.
	mergedKey := status.Account.Username + ":" + status.Reblog.Account.Username
	imgEntry := imageCache.Get(mergedKey)

	if imgEntry.status == Processed {
		return imgEntry.imgWidget
	}

	// get multiple images and composite them together.
	boosterAvatar := imageCache.Get(status.Account.Username)
	if boosterAvatar.status == NotProcessed {
		imageChannel <- ImageDetails{
			name:   status.Account.Username,
			url:    status.Account.Avatar,
			resize: true,
			width:  50,
			height: 50,
		}
	}

	boostedAvatar := imageCache.Get(status.Reblog.Account.Username)
	if boostedAvatar.status == NotProcessed {
		imageChannel <- ImageDetails{
			name:   status.Reblog.Account.Username,
			url:    status.Reblog.Account.Avatar,
			resize: true,
			width:  50,
			height: 50,
		}
	}

	if boosterAvatar.status != Processed || boostedAvatar.status != Processed {
		log.Debugf("returning default avatar due to booster status %v and boosted status %v", boosterAvatar.status, boostedAvatar.status)
		return defaultAvatar
	}

	// have both... so composite them together. Original behind and larger.
	newImage := image.NewRGBA(image.Rect(0, 0, 50, 50))
	draw.Draw(newImage, newImage.Bounds(), boostedAvatar.img, image.Point{}, draw.Src)

	smallBoostedAvatar := resizeImage(boosterAvatar.img, 30, 30)

	draw.Draw(newImage, image.Rectangle{
		Min: image.Point{20, 20},
		Max: image.Point{50, 50},
	}, smallBoostedAvatar, image.Point{}, draw.Src)

	// now store in cache for later use.
	imageCache.Set(mergedKey, ImageCacheEntry{img: newImage, imgWidget: widget.Image{
		Src: paint.NewImageOp(newImage)}, lastUsed: imageCache.Get(mergedKey).lastUsed, status: Processed})

	return widget.Image{
		Src: paint.NewImageOp(newImage),
	}
}

// generateMedia retrieves the previewImage from the Mastodon server.
// Will rescale to something suitable to view.
func generateMedia(status mastodon.Status) (*widget.Image, string) {

	var imgEntry ImageCacheEntry

	// if reblog, get original media

	if status.Reblog != nil && len(status.Reblog.MediaAttachments) > 0 {
		imgEntry = imageCache.Get(status.Reblog.MediaAttachments[0].PreviewURL)
		if imgEntry.status == NotProcessed {
			imageChannel <- ImageDetails{
				name:   status.Reblog.MediaAttachments[0].PreviewURL,
				url:    status.Reblog.MediaAttachments[0].PreviewURL,
				resize: true,
				width:  400,
				height: 0,
			}
			// no image to view yet... will be updated later.
			return nil, ""
		}

		if imgEntry.status == Processed {
			return &imgEntry.imgWidget, status.Reblog.MediaAttachments[0].URL
		}
	}

	if len(status.MediaAttachments) > 0 {
		// get multiple images and composite them together.
		imgEntry = imageCache.Get(status.MediaAttachments[0].PreviewURL)
		if imgEntry.status == NotProcessed {
			imageChannel <- ImageDetails{
				name:   status.MediaAttachments[0].PreviewURL,
				url:    status.MediaAttachments[0].PreviewURL,
				resize: true,
				width:  400,
				height: 0,
			}
			// no image to view yet... will be updated later.
			return nil, ""
		}
		if imgEntry.status == Processed {
			return &imgEntry.imgWidget, status.MediaAttachments[0].URL
		}
	}

	return nil, ""
}

func loadAvatar(username string, avatarURL string) widget.Image {
	imgEntry := imageCache.Get(username)
	if imgEntry.status == Processed {
		return imgEntry.imgWidget
	}

	if imgEntry.status == NotProcessed {
		// otherwise queue a download of the avatar
		imageChannel <- ImageDetails{
			name:   username,
			url:    avatarURL,
			resize: true,
			width:  50,
			height: 50,
		}
	}

	log.Debugf("avatar for %s is default", username)
	return defaultAvatar
}

func (ss *StatusState) generateDetailsSpanStyles(status mastodon.Status) []richtext.SpanStyle {

	var content string
	var mentions []mastodon.Mention

	if status.Content != "" {
		content = status.Content
		mentions = status.Mentions
	} else {
		if status.Reblog != nil {
			content = status.Reblog.Content
			mentions = status.Reblog.Mentions
		}
	}

	spans := []richtext.SpanStyle{}

	// replace for moment... just to test.
	//details = `<p>my first Go project, a small tic tac toe peer-to-peer game (over TCP)</p><p><a href="https://github.com/Abdenasser/tcp-tac-toe" rel="nofollow noopener noreferrer" translate="no" target="_blank"><span class="invisible">https://</span><span class="ellipsis">github.com/Abdenasser/tcp-tac-</span><span class="invisible">toe</span></a></p><p>Discussions: <a href="https://discu.eu/q/https://github.com/Abdenasser/tcp-tac-toe" rel="nofollow noopener noreferrer" translate="no" target="_blank"><span class="invisible">https://</span><span class="ellipsis">discu.eu/q/https://github.com/</span><span class="invisible">Abdenasser/tcp-tac-toe</span></a></p><p><a href="https://mastodon.social/tags/golang" class="mention hashtag" rel="nofollow noopener noreferrer" target="_blank">#<span>golang</span></a> <a href="https://mastodon.social/tags/programming" class="mention hashtag" rel="nofollow noopener noreferrer" target="_blank">#<span>programming</span></a></p>`
	t := html2text.HTML2TextWithOptions(content, html2text.WithLinksInnerText())
	text := []rune(t)
	inURL := false
	var urlString []rune
	var nonURLString []rune
	for i := 0; i < len(text); i++ {
		if text[i] == '<' {

			if len(nonURLString) > 0 {
				s := string(nonURLString)
				span := richtext.SpanStyle{
					Content: s,
					Color:   ss.th.Fg,

					Size: unit.Sp(15),
					Font: fonts[0].Font,
				}
				spans = append(spans, span)
				nonURLString = []rune{}
			}
			// check if the text started with <http to see if we're a URL or not.
			// otherwise will fail when people genuinely use < and > in the toot.
			if len(text) > i+5 && strings.ToLower(string(text[i:i+5])) == "<http" {
				inURL = true
				continue
			}
		}

		if text[i] == '>' && inURL {
			inURL = false

			linkText := string(urlString)

			span := richtext.SpanStyle{
				Content: "",
				Color:   ss.th.LinkColour,
				Size:    unit.Sp(15),
				Font:    fonts[0].Font,
			}

			textToRemove := linkText

			if urlIsUsername(linkText) {
				textToRemove = path.Base(linkText)
				if textToRemove[0] == '@' {
					span.Set("username", path.Base(linkText)[1:])
					// search for mentions in status
					for _, mention := range mentions {
						if mention.Username == path.Base(linkText)[1:] {
							span.Set("userID", fmt.Sprintf("%d", mention.ID))
						}
					}
				}
			} else if urlIsTag(linkText) {
				// If URL is tag... then remove #tag from the previous span and add it to the current span.
				sp := strings.Split(linkText, "/")
				tagText := sp[len(sp)-1]
				textToRemove = "#" + tagText
				span.Set("tag", path.Base(linkText))
			} else {
				span.Set("url", linkText)
			}

			if len(spans) > 0 {
				// replace tag text in previous span
				spans[len(spans)-1].Content = strings.ReplaceAll(spans[len(spans)-1].Content, textToRemove, "")
			}

			span.Content = textToRemove
			span.Interactive = true
			spans = append(spans, span)
			urlString = []rune{}
			continue
		}

		if inURL {
			urlString = append(urlString, rune(text[i]))
		} else {
			nonURLString = append(nonURLString, rune(text[i]))
		}
	}

	if len(nonURLString) > 0 {
		span := richtext.SpanStyle{
			Content: string(nonURLString),
			Color:   ss.th.Fg,
			Size:    unit.Sp(15),
			Font:    fonts[0].Font,
		}
		spans = append(spans, span)
		nonURLString = []rune{}
	}

	return spans
}

func (ss *StatusState) generateNameSpanStyles(status mastodon.Status) []richtext.SpanStyle {

	var spans []richtext.SpanStyle

	// if we're generating our own content... then just use acct.
	if status.Content != "" {
		span := richtext.SpanStyle{
			Content:     status.Account.DisplayName,
			Color:       ss.th.Fg,
			Size:        unit.Sp(15),
			Font:        fonts[0].Font,
			Interactive: true,
		}
		span.Set("username", status.Account.Username)
		span.Set("userID", fmt.Sprintf("%d", status.Account.ID))
		spans = append(spans, span)
	} else {
		if status.Reblog != nil {
			span := richtext.SpanStyle{
				Content:     status.Account.DisplayName,
				Color:       ss.th.Fg,
				Size:        unit.Sp(17),
				Font:        fonts[0].Font,
				Interactive: true,
			}
			span.Set("username", status.Account.Username)
			span.Set("userID", fmt.Sprintf("%d", status.Account.ID))
			spans = append(spans, span)

			span2 := richtext.SpanStyle{
				Content: " boosted ",
				Color:   ss.th.BoostedColour,
				Size:    unit.Sp(16),
				Font:    fonts[0].Font,
			}
			spans = append(spans, span2)

			span3 := richtext.SpanStyle{
				Content:     status.Reblog.Account.DisplayName,
				Color:       ss.th.Fg,
				Size:        unit.Sp(17),
				Font:        fonts[0].Font,
				Interactive: true,
			}
			span3.Set("username", status.Reblog.Account.Username)
			span3.Set("userID", fmt.Sprintf("%d", status.Reblog.Account.ID))
			spans = append(spans, span3)
		}
	}

	return spans
}

// not fool proof
func urlIsTag(text string) bool {
	if strings.Contains(text, "/tags/") {
		return true
	}
	return false
}

func urlIsUsername(text string) bool {

	if path.Base(text)[0] == '@' && strings.Contains(text, "/@") {
		return true
	}
	return false
}

// StatusStyle defines the presentation of a task.
type StatusStyle struct {
	state   *StatusState
	Name    richtext.SpanStyle
	Details richtext.SpanStyle

	Loader material.LoaderStyle
	Error  material.LabelStyle
}

// NewStatusStyle builds the widget style information to display the task
// with the given state. The style is ephemeral, created and discarded
// each frame. TODO(kpfaulkner) check if we really need this.
func NewStatusStyle(th *material.Theme, state *StatusState) StatusStyle {
	i := StatusStyle{
		state: state,
	}

	return i
}

// Layout inserts operations describing this task's UI into the operation
// list within gtx.
func (i StatusStyle) Layout(gtx C) D {

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
					return i.layoutStatus(gtx)
				}),
			)
		}),
	)
}

// layoutStatus displays the text editors used to manipulate the task.
func (i StatusStyle) layoutStatus(gtx C) D {
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

			// DETAILS
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return i.state.DetailStyle.Layout(gtx)
			}),

			layout.Rigid(layout.Spacer{Height: spacing}.Layout),

			// image/media?
			layout.Rigid(func(gtx C) D {

				if i.state.img != nil {
					ib := newImageButton(&i.state.th.Theme, &i.state.imgButton, i.state.img)
					return ib.Layout(gtx)
				}

				// if no image.. then just empty
				return layout.Dimensions{
					Size:     image.Point{},
					Baseline: 0,
				}
			}),
			layout.Rigid(layout.Spacer{Height: spacing}.Layout),

			// Horizontal for putting in globalIcons (reply, boost etc)
			layout.Rigid(func(gtx C) D {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Spacing:   layout.SpaceBetween,
					Alignment: layout.Start,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						ic, _ := widget.NewIcon(icons.ContentReply)
						replyButton := newIconButton(i.state.th, &i.state.ReplyButton, ic, i.state.th.IconBackgroundColour)
						//if i.state.status. {
						//	replyButton.iconColour = i.state.th.IconActiveColour
						//} else {
						replyButton.iconColour = i.state.th.IconInactiveColour
						//

						return replyButton.Layout(gtx)

					}),
					layout.Rigid(layout.Spacer{Width: spacing}.Layout),
					layout.Rigid(func(gtx C) D {
						ic, _ := widget.NewIcon(icons.CommunicationCallMissedOutgoing)
						boostButton := newIconButton(i.state.th, &i.state.BoostButton, ic, i.state.th.IconBackgroundColour)
						if i.state.status.Reblogged.(bool) {
							boostButton.iconColour = i.state.th.IconActiveColour
						} else {
							boostButton.iconColour = i.state.th.IconInactiveColour
						}
						return boostButton.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: spacing}.Layout),
					layout.Rigid(func(gtx C) D {
						ic, _ := widget.NewIcon(icons.ActionStarRate)
						favButton := newIconButton(i.state.th, &i.state.FavouriteButton, ic, i.state.th.IconBackgroundColour)
						if i.state.status.Favourited.(bool) {
							favButton.iconColour = i.state.th.IconActiveColour
						} else {
							favButton.iconColour = i.state.th.IconInactiveColour
						}

						return favButton.Layout(gtx)
					}),

					layout.Rigid(layout.Spacer{Width: spacing}.Layout),

					// Description
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return i.state.DescriptionStyle.Layout(gtx)
					}),

					layout.Rigid(layout.Spacer{Width: spacing}.Layout),

					layout.Rigid(func(gtx C) D {
						var dim layout.Dimensions
						if i.state.status.InReplyToID != nil {
							ic, _ := widget.NewIcon(icons.ActionList)
							threadButton := newIconButton(i.state.th, &i.state.ViewThreadButton, ic, i.state.th.IconBackgroundColour)
							threadButton.iconColour = i.state.th.IconInactiveColour
							dim = threadButton.Layout(gtx)
						} else {
							dim = layout.Dimensions{
								Size:     image.Point{},
								Baseline: 0,
							}
						}
						return dim

					}),

					// This needs to be calculated... FIXME(kpfaulkner)
					// Unsure why width 200 + changing Flex to be SpaceBetween works, but it does
					// So will leave it and figure it out later :)
					layout.Rigid(layout.Spacer{Width: 200}.Layout),

					layout.Rigid(func(gtx C) D {
						return material.Label(&i.state.th.Theme, unit.Sp(15), fmt.Sprintf("%s", statusAge(i.state.status.CreatedAt))).Layout(gtx)
					}),
				)
			}),
		)
	})
}

// statusAge return days, hours or minutes
func statusAge(createdAt time.Time) string {
	if time.Since(createdAt).Hours() < 1 {
		return fmt.Sprintf("%dm", int(time.Since(createdAt).Minutes()))
	} else if time.Since(createdAt).Hours() > 24 {
		return fmt.Sprintf("%dd", int(time.Since(createdAt).Hours()/24))
	} else {
		return fmt.Sprintf("%dh", int(time.Since(createdAt).Hours()))
	}
}

func makeBlankImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{0, 100, 0, 255}}, image.Point{}, draw.Src)
	return img
}
