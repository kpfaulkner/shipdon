package ui

import (
	"context"
	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	log "github.com/sirupsen/logrus"
	"image"
	"image/gif"
	"net/http"
	"path"
	"time"
)

const (
	MaxImageSize = 800
)

// open new window for viewing
func openNewWindow(url string) {
	w := new(app.Window)

	var ops op.Ops

	start := time.Now()

	// download, scale and save
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	ext := path.Ext(url)
	var img image.Image
	switch ext {
	case ".gif":
		img, err = gif.Decode(resp.Body)
		if err != nil {
			return
		}
	default:
		img, _, err = image.Decode(resp.Body)
		if err != nil {
			return
		}
	}

	log.Debugf("full image download took %d ms", time.Now().Sub(start).Milliseconds())
	ctx, _ := context.WithCancel(context.Background())

	go func() {
		<-ctx.Done()
		w.Perform(system.ActionClose)
	}()

	// if image size is greater than 800 in any dimension, then scale it down.
	sz := img.Bounds().Size()
	if sz.X > MaxImageSize || sz.Y > MaxImageSize {
		if sz.X > sz.Y {
			sz.Y = sz.Y * MaxImageSize / sz.X
			sz.X = MaxImageSize
		} else {
			sz.X = sz.X * MaxImageSize / sz.Y
			sz.Y = MaxImageSize
		}
	}

	opts := []app.Option{}
	opts = append(opts, app.Size(unit.Dp(sz.X), unit.Dp(sz.Y)))
	opts = append(opts, app.Title("Image Viewer"))
	w.Option(opts...)

	var inset = layout.UniformInset(8)

	i := widget.Image{
		Src: paint.NewImageOp(img),
		Fit: widget.Contain,
	}

	for {
		e := w.Event()
		if e, ok := e.(app.FrameEvent); ok {
			gtx := app.NewContext(&ops, e)

			inset.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {

					return layout.Flex{
						Axis: layout.Vertical,
					}.Layout(gtx,

						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return i.Layout(gtx)
						}),
					)

				})
			e.Frame(gtx.Ops)
		}
	}
}
