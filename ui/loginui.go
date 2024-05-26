package ui

import (
	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"os"
)

func openLoginWindow() string {
	w := new(app.Window)
	w.Option(
		app.Title("OAuth Login"),
		app.Size(unit.Dp(800), unit.Dp(600)))
	var ops op.Ops

	go func() {
		for {
			ev := w.Event()
			if _, ok := ev.(app.DestroyEvent); ok {
				return
			}
		}
	}()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	var editor component.TextField
	var notifyBtn widget.Clickable

	for {
		switch event := w.Event().(type) {
		case app.DestroyEvent:
			os.Exit(0)
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			if notifyBtn.Clicked(gtx) {
				code := ""
				if txt := editor.Text(); txt != "" {
					code = txt
				}

				w.Perform(system.ActionClose)

				// now go do something.
				return code

			}
			layout.Center.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Max.X = gtx.Dp(unit.Dp(300))
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return editor.Layout(gtx, th, "enter code")
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Spacer{Height: unit.Dp(10)}.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return material.Button(th, &notifyBtn, "submit").Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Spacer{Height: unit.Dp(10)}.Layout(gtx)
					}),
				)
			})
			event.Frame(gtx.Ops)
		}
	}
}
