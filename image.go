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

func hueToRGB(p, q, t float32) uint8 {

	if t < 0.0 {
		t = t + 1.0
	}

	if t > 1.0 {
		t = t - 1.0
	}

	if t < 1.0/6.0 {
		v := p + (q-p)*6*t
		return uint8(v * 255)
	}

	if t < 1.0/2.0 {
		v := q
		return uint8(v * 255)
	}

	if t < 2.0/3.0 {
		v := p + (q-p)*(2.0/3.0-t)*6
		return uint8(v * 255)
	}

	return uint8(p * 255)
}

// h [0,1], s [0,1], l [0,1], a [0,1]
func hslaToRGBA(h, s, l, a float32) color.Color {
	var r, g, b uint8

	if s == 0 {
		v := uint8(l + 0.5) // Roudning
		r, g, b = v, v, v
	} else {
		var q, p float32
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = (l + s) - (l * s)
		}
		p = 2.0*l - q

		r = hueToRGB(p, q, h+1.0/3.0)
		g = hueToRGB(p, q, h)
		b = hueToRGB(p, q, h-1.0/3.0)
	}

	return color.RGBA{r, g, b, uint8(a * 255)}
}

func parseColor(s string) color.Color {
	var c color.Color

	colorExps := map[string]*regexp.Regexp{
		"hex":  regexp.MustCompile(`^#((\d|[aAbBcCdDeEfF]){6})$`),
		"rgb":  regexp.MustCompile(`^rgb\((\d{1,3}),(\d{1,3}),(\d{1,3})\)$`),
		"rgba": regexp.MustCompile(`^rgba\((\d{1,3}),(\d{1,3}),(\d{1,3}),(\d(\.\d+)?)\)$`),
		"hsl":  regexp.MustCompile(`^hsl\((\d{1,3}(\.\d+)?),(\d{1,3}(\.\d+)?)%,(\d{1,3}(\.\d+)?)%\)$`),
		"hsla": regexp.MustCompile(`^hsla\((\d{1,3}(\.\d+)?),(\d{1,3}(\.\d+)?)%,(\d{1,3}(\.\d+)?)%,(\d(\.\d+)?)\)$`)}

	for key, exp := range colorExps {
		if exp.MatchString(s) {
			matches := exp.FindStringSubmatch(s)
			switch key {
			case "hex":
				rgb, _ := strconv.ParseUint(matches[1], 16, 32)
				c = color.RGBA{uint8(rgb >> 16), uint8((rgb >> 8) & 0xFF), uint8(rgb & 0xFF), 255}
			case "rgb":
				r, _ := strconv.ParseUint(matches[1], 10, 8)
				g, _ := strconv.ParseUint(matches[2], 10, 8)
				b, _ := strconv.ParseUint(matches[3], 10, 8)
				c = color.RGBA{uint8(r), uint8(g), uint8(b), 255}
			case "rgba":
				r, _ := strconv.ParseUint(matches[1], 10, 8)
				g, _ := strconv.ParseUint(matches[2], 10, 8)
				b, _ := strconv.ParseUint(matches[3], 10, 8)
				a, _ := strconv.ParseFloat(matches[4], 32)
				c = color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a * 255)}
			case "hsl":
				h, _ := strconv.ParseFloat(matches[1], 32)
				s, _ := strconv.ParseFloat(matches[3], 32)
				l, _ := strconv.ParseFloat(matches[5], 32)
				a := 1.0
				c = hslaToRGBA(float32(h)/360.0, float32(s)/100.0, float32(l)/100.0, float32(a))
			case "hsla":
				h, _ := strconv.ParseFloat(matches[1], 32)
				s, _ := strconv.ParseFloat(matches[3], 32)
				l, _ := strconv.ParseFloat(matches[5], 32)
				a, _ := strconv.ParseFloat(matches[7], 32)
				c = hslaToRGBA(float32(h)/360.0, float32(s)/100.0, float32(l)/100.0, float32(a))
			}
		}
	}

	return c
}

func parseSize(s string) map[string]interface{} {
	sizeExps := map[string]*regexp.Regexp{
		"scale":       regexp.MustCompile(`^(\d+)\%$`),
		"width":       regexp.MustCompile(`^(\d+)([\^!><#])?$`),
		"area":        regexp.MustCompile(`^(\d+)@$`),
		"height":      regexp.MustCompile(`^x(\d+)([\^!><#])?$`),
		"scaleXY":     regexp.MustCompile(`^(\d+)%x(\d+)%$`),
		"widthHeight": regexp.MustCompile(`^(\d+)x(\d+)([\*\^!><#])?$`)}

	options := make(map[string]interface{})
	options["crop"] = false
	options["minimum"] = false
	options["maximum"] = false
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
				case "*":
					options["maximum"] = true
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
				case "*":
					options["maximum"] = true
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
				case "*":
					options["maximum"] = true
				case "!":
					options["enforce"] = true
				}
			}
		}
	}

	return options
}

func extractOptions(r *http.Request, m image.Image) map[string]interface{} {
	sizeQuery := r.URL.Query().Get("s")
	backgroundQuery := r.URL.Query().Get("bg")

	if sizeQuery == "" {
		queries := strings.Split(strings.Split(r.URL.Path, "/")[1], "&")
		queryFn := func(value string) bool { return value[0:2] == "s=" }
		for _, value := range queries {
			if queryFn(value) {
				sizeQuery = value[2:]
			}
		}
	}

	options := map[string]interface{}{"format": "png"}

	sizeOptions := parseSize(sizeQuery)

	options["format"] = r.URL.Path[strings.LastIndex(r.URL.Path, ".")+1:]
	options["mimeType"] = mime.TypeByExtension(strings.Join([]string{".", options["format"].(string)}, ""))
	options["width"] = sizeOptions["width"]
	options["height"] = sizeOptions["height"]
	options["crop"] = sizeOptions["crop"]
	options["minimum"] = sizeOptions["minimum"]
	options["maximum"] = sizeOptions["maximum"]
	options["enforce"] = sizeOptions["enforce"]

	if backgroundQuery != "" {
		backgroundColor := parseColor(backgroundQuery)
		options["backgroundColor"] = backgroundColor
	}

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
	draw.Draw(bg, image.Rect(wOffset, hOffset, m.Bounds().Dx()+wOffset, m.Bounds().Dy()+hOffset), m, image.ZP, draw.Over)
	return bg
}

func (h *proxyHandler) watermark(m image.Image) (image.Image, error) {
	if h.watermarkImage == nil {
		return m, nil
	}

	mRect := m.Bounds()
	if mRect.Dx()*mRect.Dy() <= 90000 {
		return m, nil
	}

	wWidth := int(float32(mRect.Dx()) * 0.05)
	if wWidth > h.watermarkImage.Bounds().Dx() {
		wWidth = h.watermarkImage.Bounds().Dx()
	}
	scaledWatermark := resize.Resize(uint(wWidth), 0, h.watermarkImage, resize.MitchellNetravali)

	b := m.Bounds()
	r := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(r, r.Bounds(), m, b.Min, draw.Src)

	margin := int(float32(mRect.Dx()) * 0.03)
	location := image.Rect(r.Bounds().Dx()-margin-scaledWatermark.Bounds().Dx(), r.Bounds().Dy()-margin-scaledWatermark.Bounds().Dy(), r.Bounds().Dx()-margin, r.Bounds().Dy()-margin)
	draw.DrawMask(r, location, scaledWatermark, image.ZP, nil, image.ZP, draw.Over)
	return r, nil
}
