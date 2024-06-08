package ui

import (
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/mattn/go-mastodon"
	"golang.org/x/exp/shiny/materialdesign/icons"
	"image"
	"image/color"
)

// ComposeColumn defines the top-level presentation of your UI.
type ComposeColumn struct {
	// th configures styling of widgets.
	th *ShipdonTheme
	ComponentState
	gtx C

	postTootDetails widget.Editor
	searchQuery     widget.Editor
	searchButton    widget.Clickable
	postTootButton  widget.Clickable
	settingsButton  widget.Clickable
	refreshButton   widget.Clickable
	cancelButton    widget.Clickable

	// if we're replying... know the status that we're replying to.
	replyStatusID mastodon.ID

	// status list used when showing search results
	searchResults []*mastodon.Status
}

// NewComposeColumn builds a messageColumns using a controller and backend.
func NewComposeColumn(componentState ComponentState, th *ShipdonTheme) *ComposeColumn {
	p := &ComposeColumn{
		th:             th,
		ComponentState: componentState,
	}

	p.postTootDetails.SingleLine = false
	p.postTootDetails.Submit = false
	p.searchQuery.SingleLine = true
	p.searchQuery.Submit = true
	return p
}

// Layout builds your UI within the operation list in gtx.
func (p *ComposeColumn) Layout(gtx C) D {

	// width 300, height is whatever parent already set.
	sideLength := gtx.Dp(300)
	gtx.Constraints.Min = image.Point{X: sideLength, Y: gtx.Constraints.Max.Y}
	gtx.Constraints.Max = image.Point{X: sideLength, Y: gtx.Constraints.Max.Y}
	paint.FillShape(gtx.Ops, p.th.StatusBackgroundColour, clip.Rect{Max: gtx.Constraints.Max}.Op())
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(p.layoutCreateStatusForm),
	)
}

// layoutCreateStatusForm displays a simple form for creating tasks.
func (p *ComposeColumn) layoutCreateStatusForm(gtx C) D {

	return layout.UniformInset(12).Layout(gtx, func(gtx C) D {
		return layout.Flex{
			Alignment: layout.Middle,
		}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				gtx.Constraints = gtx.Constraints.AddMin(image.Point{Y: 100})

				// Toot Details (ie the message)
				return layout.Flex{
					Axis:    layout.Vertical,
					Spacing: layout.SpaceEnd,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						gtx.Constraints = gtx.Constraints.AddMin(image.Point{Y: 100})
						ed := material.Editor(&p.th.Theme, &p.postTootDetails, "Toot")
						return ed.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: 10}.Layout),
					layout.Rigid(func(gtx C) D {
						ed := material.Editor(&p.th.Theme, &p.searchQuery, "Search")
						ed.Color = color.NRGBA{R: 200, G: 200, B: 0, A: 255}
						return ed.Layout(gtx)
					}),

					// search results... split into status, user or hashtag?
					layout.Rigid(layout.Spacer{Height: 10}.Layout),
				)
			}),

			// misc buttons...
			layout.Rigid(func(gtx C) D {
				gtx.Constraints = gtx.Constraints.AddMin(image.Point{Y: 100})
				return layout.Flex{
					Axis: layout.Vertical,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						ic, _ := widget.NewIcon(icons.ImageBrush)
						//settingsButton := material.IconButton(p.th, &p.postTootButton, ic, "Post").Layout(gtx)
						postToot := newIconButton(p.th, &p.postTootButton, ic, p.th.IconActiveColour)
						return postToot.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						ic, _ := widget.NewIcon(icons.ActionSettings)
						//settingsButton := material.IconButton(p.th, &p.settingsButton, ic, "Settings").Layout(gtx)
						settingsButton := newIconButton(p.th, &p.settingsButton, ic, p.th.IconActiveColour)
						return settingsButton.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						ic, _ := widget.NewIcon(icons.NavigationRefresh)
						refreshButton := newIconButton(p.th, &p.refreshButton, ic, p.th.IconActiveColour)
						return refreshButton.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						ic, _ := widget.NewIcon(icons.NavigationCancel)
						ib := newIconButton(p.th, &p.cancelButton, ic, p.th.IconActiveColour)
						return ib.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						var dim layout.Dimensions
						if p.searchQuery.Text() != "" {
							ic, _ := widget.NewIcon(icons.ActionSearch)
							searchButton := newIconButton(p.th, &p.searchButton, ic, p.th.IconActiveColour)
							searchButton.iconColour = p.th.IconActiveColour
							dim = searchButton.Layout(gtx)
						} else {
							dim = layout.Dimensions{
								Size:     image.Point{},
								Baseline: 0,
							}
						}
						return dim
					}),
				)
			}),
		)
	})
}

// layoutHeader displays a simple top bar.
func (p *ComposeColumn) layoutHeader(gtx C) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			paint.FillShape(gtx.Ops, p.th.ContrastBg, clip.Rect{Max: gtx.Constraints.Min}.Op())
			return D{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(12).Layout(gtx, func(gtx C) D {
				l := material.H6(&p.th.Theme, "Todo")
				l.Color = p.th.ContrastFg
				return l.Layout(gtx)
			})
		}),
	)
}
