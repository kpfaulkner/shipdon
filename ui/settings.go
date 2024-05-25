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
	"github.com/kpfaulkner/shipdon/config"
	log "github.com/sirupsen/logrus"
	"os"
)

const (
	DarkMode  = "DarkMode"
	LightMode = "LightMode"
)

func openSettingsWindow(cfg *config.Config) {
	w := new(app.Window)
	w.Option(
		app.Title("Settings"),
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
	var instanceURLEditor component.TextField
	var saveButton widget.Clickable

	radioButtonsGroup := new(widget.Enum)

	// set up existing values.
	instanceURLEditor.SetText(cfg.InstanceURL)
	if cfg.DarkMode {
		radioButtonsGroup.Value = DarkMode
	} else {
		radioButtonsGroup.Value = LightMode
	}

	for {
		switch event := w.Event().(type) {
		case app.DestroyEvent:
			w.Perform(system.ActionClose)
			return
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			if saveButton.Clicked(gtx) {

				if txt := instanceURLEditor.Text(); txt != "" {
					cfg.InstanceURL = txt
				}

				cfg.DarkMode = radioButtonsGroup.Value == DarkMode
				cfg.Save()
				w.Perform(system.ActionClose)
				return

			}
			layout.NW.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Max.X = gtx.Dp(unit.Dp(300))
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return instanceURLEditor.Layout(gtx, th, "Instance URL")
					}),

					layout.Rigid(func(gtx C) D {
						return layout.Spacer{Height: unit.Dp(10)}.Layout(gtx)
					}),

					// light vs dark mode
					layout.Rigid(material.RadioButton(th, radioButtonsGroup, LightMode, "Light Mode").Layout),
					layout.Rigid(material.RadioButton(th, radioButtonsGroup, DarkMode, "Dark Mode").Layout),
					layout.Rigid(func(gtx C) D {
						return layout.Spacer{Height: unit.Dp(10)}.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return material.Button(th, &saveButton, "Save").Layout(gtx)
					}),
				)
			})

			log.Debugf("RADIO BUTTON %s", radioButtonsGroup.Value)

			event.Frame(gtx.Ops)
		}
	}
}

func openInstanceWindow() string {
	w := new(app.Window)
	w.Option(
		app.Title("Instance URL"),
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
				instanceURL := ""
				if txt := editor.Text(); txt != "" {
					instanceURL = txt
				}
				w.Perform(system.ActionClose)
				// now go do something.
				return instanceURL

			}
			layout.Center.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Max.X = gtx.Dp(unit.Dp(300))
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return editor.Layout(gtx, th, "Enter instance URL (eg. https://hachyderm.io")
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Spacer{Height: unit.Dp(10)}.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return material.Button(th, &notifyBtn, "notify").Layout(gtx)
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
