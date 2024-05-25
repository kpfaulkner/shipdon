package ui

import (
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"image/color"
)

// imageButton is an attempt to make a clickable image
// The plan is to base this off gio-example iconAndTextButton
type iconButton struct {
	theme      *ShipdonTheme
	button     *widget.Clickable
	icon       *widget.Icon
	inset      layout.Inset
	iconColour color.NRGBA
}

func newIconButton(th *ShipdonTheme, button *widget.Clickable, icon *widget.Icon, iconColour color.NRGBA) iconButton {
	return iconButton{
		theme:      th,
		button:     button,
		icon:       icon,
		inset:      layout.UniformInset(unit.Dp(1)),
		iconColour: iconColour,
	}
}

// Layout is the function that lays out the icon button
// Put circle around icon. Need stack layout
func (b iconButton) Layout(gtx C) D {

	bgColour := b.theme.Palette.Bg
	bgColour.R -= 10
	bgColour.G -= 10
	bgColour.B -= 10

	return b.button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)

		return layout.Stack{
			Alignment: layout.Center,
		}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				shape := clip.Ellipse{Max: gtx.Constraints.Min}
				paint.FillShape(gtx.Ops, bgColour, shape.Op(gtx.Ops))

				return layout.Dimensions{
					Size: gtx.Constraints.Min,
				}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return b.inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return b.icon.Layout(gtx, b.iconColour)
				})
			}),
		)
	})
}
