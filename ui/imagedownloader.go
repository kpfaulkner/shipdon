package ui

import (
	"gioui.org/op/paint"
	"gioui.org/widget"
	log "github.com/sirupsen/logrus"
	"golang.org/x/image/draw"
	"image"
	"math"
	"time"
)

func downloadImages(ch chan ImageDetails) {

	downloadResponseCh := make(chan ImageDownloadDetails, 10000)
	requestChannel := make(chan ImageDetails, 10000)

	for i := 0; i < 5; i++ {
		go func(req chan ImageDetails, respCh chan ImageDownloadDetails, ii int) {
			for m := range req {
				log.Debugf("downloader %d reports req queue length %d", ii, len(req))
				log.Debugf("downloader %d reports resp queue length %d", ii, len(respCh))
				img, err := downloadImage(m.url)
				if err != nil {
					log.Errorf("Error downloading image %s\n", m.url)
					continue
				}

				if m.resize {
					img2 := resizeImage(*img, m.width, m.height)
					respCh <- ImageDownloadDetails{
						request: m,
						img:     img2,
					}
				} else {
					respCh <- ImageDownloadDetails{
						request: m,
						img:     *img,
					}
				}
			}
		}(requestChannel, downloadResponseCh, i)
	}

	for {
		select {
		case resp := <-downloadResponseCh:
			imageCache.Set(resp.request.name, ImageCacheEntry{img: resp.img, imgWidget: widget.Image{
				Src: paint.NewImageOp(resp.img)}, lastUsed: time.Now(), status: Processed})

		case req := <-ch:
			entry := imageCache.Get(req.name)
			if entry.status == Processing {
				// already processing... skip queuing
				continue
			}
			imageCache.Set(req.name, ImageCacheEntry{lastUsed: time.Now(), status: Processing})
			requestChannel <- req
		}
	}
}

func resizeImage(img image.Image, width, height int) *image.RGBA {
	bounds := img.Bounds()

	if width == 0 && height == 0 {
		return nil
	}

	if width == 0 {
		width = bounds.Dx() * height / bounds.Dy()
	}
	if height == 0 {
		height = bounds.Dy() * width / bounds.Dx()
	}

	if width > 500 || height > 500 {
		scaleFactor := float64(500) / math.Max(float64(width), float64(height))
		width = int(float64(width) * scaleFactor)
		height = int(float64(height) * scaleFactor)
	}

	newImg := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(newImg, newImg.Bounds(), img, bounds, draw.Over, nil)

	return newImg
}
