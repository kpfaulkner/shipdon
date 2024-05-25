package ui

import (
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// imageButton is an attempt to make a clickable image
// The plan is to base this off gio-example iconAndTextButton
type imageButton struct {
	theme   *material.Theme
	button  *widget.Clickable
	img     *widget.Image
	origURL string
	inset   layout.Inset
}

func newImageButton(th *material.Theme, button *widget.Clickable, img *widget.Image) imageButton {
	return imageButton{
		theme:  th,
		button: button,
		img:    img,
		inset:  layout.UniformInset(unit.Dp(1)),
	}
}

func (b imageButton) Layout(gtx layout.Context) layout.Dimensions {

	return b.button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,

			layout.Rigid(func(gtx C) D {
				return b.inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					if b.img != nil {
						gtx.Constraints.Min = b.img.Src.Size()
						b.img.Layout(gtx)
					}
					return layout.Dimensions{
						Size: b.img.Src.Size(),
					}
				})
			},
			))
	})
}
