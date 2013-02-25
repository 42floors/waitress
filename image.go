package main

import (
	"github.com/nfnt/resize"
	"image"
	"image/color"
	"image/draw"
	"log"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

func parseSize(s string) map[string]interface{} {
	sizeExps := map[string]*regexp.Regexp{
		"scale":       regexp.MustCompile(`^(\d+)\%$`),
		"width":       regexp.MustCompile(`^(\d+)([\^!><#])?$`),
		"area":        regexp.MustCompile(`^(\d+)@$`),
		"height":      regexp.MustCompile(`^x(\d+)([\^!><#])?$`),
		"scaleXY":     regexp.MustCompile(`^(\d+)%x(\d+)%$`),
		"widthHeight": regexp.MustCompile(`^(\d+)x(\d+)([\^!><#])?$`)}

	options := make(map[string]interface{})
	options["crop"] = false
	options["minimum"] = false
	options["enforce"] = false

	for key, exp := range sizeExps {
		if exp.MatchString(s) {
			matches := exp.FindStringSubmatch(s)
			switch key {
			case "widthHeight":
				options["width"], _ = strconv.Atoi(matches[1])
				options["height"], _ = strconv.Atoi(matches[2])
				switch matches[3] {
				case "#":
					options["crop"] = true
				case "^":
					options["minimum"] = true
				case "!":
					options["enforce"] = true
				}
			case "width":
				options["width"], _ = strconv.Atoi(matches[1])
				switch matches[2] {
				case "#":
					options["crop"] = true
				case "^":
					options["minimum"] = true
				case "!":
					options["enforce"] = true
				}
			case "height":
				options["height"], _ = strconv.Atoi(matches[1])
				switch matches[2] {
				case "#":
					options["crop"] = true
				case "^":
					options["minimum"] = true
				case "!":
					options["enforce"] = true
				}
			}
		}
	}

	return options
}

func extractOptions(r *http.Request, m image.Image) map[string]interface{} {
	sizeOptions := parseSize(r.URL.Query().Get("s"))

	options := map[string]interface{}{"format": "png"}
	options["format"] = r.URL.Path[strings.LastIndex(r.URL.Path, ".")+1:]
	options["mimeType"] = mime.TypeByExtension(strings.Join([]string{".", options["format"].(string)}, ""))
	options["width"] = sizeOptions["width"]
	options["height"] = sizeOptions["height"]
	options["crop"] = sizeOptions["crop"]
	options["minimum"] = sizeOptions["minimum"]
	options["enforce"] = sizeOptions["enforce"]

	w := options["width"] != nil
	h := options["height"] != nil
	if !w && !h {
		options["width"], options["height"] = m.Bounds().Dx(), m.Bounds().Dy()
	} else if !w && h {
		options["width"] = int(float32(options["height"].(int)) * (float32(m.Bounds().Dx()) / float32(m.Bounds().Dy())))
	} else if w && !h {
		options["height"] = int(float32(options["width"].(int)) / (float32(m.Bounds().Dx()) / float32(m.Bounds().Dy())))
	}

	return options
}

func crop(m image.Image, r image.Rectangle) image.Image {
	switch m.(type) {
	case *image.RGBA:
		m = m.(*image.RGBA).SubImage(r)
	case *image.YCbCr:
		m = m.(*image.YCbCr).SubImage(r)
	case *image.RGBA64:
		m = m.(*image.RGBA64).SubImage(r)
	default:
		log.Panic("Unknown color.Model")
	}
	return m
}

func resizeAndCrop(m image.Image, dstRect image.Rectangle) image.Image {
	srcRect := m.Bounds()
	srcRatio := float32(srcRect.Dx()) / float32(srcRect.Dy())
	dstRatio := float32(dstRect.Dx()) / float32(dstRect.Dy())

	// First find what size to resize the source image to, then find the
	// region to crop
	var resizeRect, cropRect image.Rectangle
	if dstRatio > srcRatio { // dst is wider than the src
		w, h := dstRect.Dx(), int(float32(dstRect.Dx())/srcRatio)
		resizeRect = image.Rect(0, 0, w, h)
		hOffset := int(float32(resizeRect.Dy()-dstRect.Dy()) / 2)
		cropRect = image.Rect(0, hOffset, w, dstRect.Dy()+hOffset)
	} else { // dst is taller or the same as the src
		h, w := dstRect.Dy(), int(srcRatio*float32(dstRect.Dy()))
		resizeRect = image.Rect(0, 0, w, h)
		wOffset := int(float32(resizeRect.Dx()-dstRect.Dx()) / 2)
		cropRect = image.Rect(wOffset, 0, dstRect.Dx()+wOffset, h)
	}

	m = resize.Resize(uint(resizeRect.Dx()), uint(resizeRect.Dy()), m, resize.MitchellNetravali)
	m = crop(m, cropRect)

	return m
}

func resizeAndPad(m image.Image, dstRect image.Rectangle, bgColor color.Color) image.Image {
	srcRect := m.Bounds()
	srcRatio := float32(srcRect.Dx()) / float32(srcRect.Dy())
	dstRatio := float32(dstRect.Dx()) / float32(dstRect.Dy())

	bg := image.NewRGBA(image.Rect(0, 0, dstRect.Dx(), dstRect.Dy()))
	draw.Draw(bg, bg.Bounds(), image.NewUniform(bgColor), image.ZP, draw.Src)
	var wOffset, hOffset int
	if srcRatio >= dstRatio { // wider
		m = resize.Resize(uint(dstRect.Dx()), 0, m, resize.MitchellNetravali)
		hOffset = int(float32(bg.Bounds().Dy()-m.Bounds().Dy()) / 2)
	} else {
		m = resize.Resize(0, uint(dstRect.Dy()), m, resize.MitchellNetravali)
		wOffset = int(float32(bg.Bounds().Dx()-m.Bounds().Dx()) / 2)
	}
	log.Println(wOffset, hOffset)
	draw.Draw(bg, image.Rect(wOffset, hOffset, m.Bounds().Dx()+wOffset, m.Bounds().Dy()+hOffset), m, image.ZP, draw.Over)
	return bg
}

func (h *proxyHandler) watermark(m image.Image) (image.Image, error) {
	mRect := m.Bounds()
	if mRect.Dx()*mRect.Dy() <= 10000 {
		return m, nil
	}

	wWidth := uint(float32(mRect.Dx()) * 0.29)
	scaledWatermark := resize.Resize(wWidth, 0, h.watermarkImage, resize.MitchellNetravali)

	b := m.Bounds()
	r := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(r, r.Bounds(), m, b.Min, draw.Src)

	margin := int(float32(mRect.Dx()) * 0.04)
	location := image.Rect(r.Bounds().Dx()-margin-scaledWatermark.Bounds().Dx(), r.Bounds().Dy()-margin-scaledWatermark.Bounds().Dy(), r.Bounds().Dx()-margin, r.Bounds().Dy()-margin)
	draw.DrawMask(r, location, scaledWatermark, image.ZP, nil, image.ZP, draw.Over)
	return r, nil
}
