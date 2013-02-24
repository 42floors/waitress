package main

import (
	"flag"
	"github.com/nfnt/resize"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"launchpad.net/goyaml"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type proxyHandler struct {
	Host      string // host of proxy
	Scheme    string // scheme of the proxy (typically http)
	Prefix    string // path prefix for the proxy
	Format    string // image format on the proxy
	Watermark string // filename of the watermark
  watermarkImage image.Image
}

func (h proxyHandler) urlFor(u *url.URL) *url.URL {
	n := new(url.URL)
	n.Scheme = h.Scheme
	n.Host = h.Host
	n.Path = h.Prefix + u.Path
	n.Path = n.Path[0:strings.LastIndex(n.Path, ".")]
	n.Path = n.Path + "." + h.Format
	n.RawQuery = ""
	return n
}

func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		status   int
		duration time.Duration
	)
	startTime := time.Now()

  resp, err1 := http.Get(h.urlFor(r.URL).String())
  defer resp.Body.Close()

  if err1 != nil || resp.StatusCode != http.StatusOK {
    status = resp.StatusCode
    w.WriteHeader(status)
  }

  if resp.StatusCode == http.StatusOK {
    m, _, err2 := image.Decode(resp.Body)
    if err1 != nil && err2 != nil {
      status = http.StatusInternalServerError
      w.WriteHeader(status)
    } else {
      status = http.StatusOK
      err3 := h.serveImage(w, r, m)
      if err3 != nil {
        status = http.StatusInternalServerError
        w.WriteHeader(status)
      }
    }
  }

	duration = time.Since(startTime)
	log.Printf("%v\t%v\n                    Completed %d %v in %dms", r.Method, r.URL.String(), status, http.StatusText(status), int(duration.Nanoseconds()/1000000))
}

func (h *proxyHandler) serveImage(w http.ResponseWriter, r *http.Request, m image.Image) error {
	options := extractOptions(r, m)

	srcRect := m.Bounds()
	dstRect := image.Rect(0, 0, options["width"].(int), options["height"].(int))

	if options["crop"].(bool) {
		resizeRect, cropRect := resizeAndCropRects(srcRect, dstRect)
		m = resize.Resize(uint(resizeRect.Dx()), uint(resizeRect.Dy()), m, resize.MitchellNetravali)
		m = crop(m, cropRect)
	} else {
		m = resize.Resize(uint(dstRect.Dx()), uint(dstRect.Dy()), m, resize.MitchellNetravali)
	}

	var err error
	m, err = h.watermark(m)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}

	const (
		year       = time.Hour * 24 * 7 * 52
		timeFormat = "Mon, 02 Jan 2006 15:04:05 GMT"
	)
	w.Header().Set("Content-type", options["mimeType"].(string))
	w.Header().Set("Expires", time.Now().Add(year).UTC().Format(timeFormat))
	w.Header().Set("Cache-Control", "public, max-age=15724800")

	switch options["mimeType"].(string) {
	case "image/jpeg":
		err = jpeg.Encode(w, m, &jpeg.Options{Quality: 85})
	case "image/png":
		err = png.Encode(w, m)
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}

	return nil
}

func parseConfigFile(n string) (*proxyHandler, error) {
	file, err := os.Open(n)

	if err != nil {
		return nil, err
	}

	var h *proxyHandler
	data, err := ioutil.ReadAll(file)

	if err != nil {
		return nil, err
	}

	err = goyaml.Unmarshal(data, &h)
	if err != nil {
		return nil, err
	}

	file, err = os.Open(h.Watermark)
	h.watermarkImage, _, err = image.Decode(file)
	if err != nil {
		return nil, err
	}

	return h, nil
}

var configFile    string
var serverPort    string
var serverBinding string

func init() {
	flag.StringVar(&configFile, "config", "", "conf file")
	flag.StringVar(&serverPort, "port", "3000", "run the server on the specified port")
	flag.StringVar(&serverBinding, "binding", "0.0.0.0", "bind the server to the specified ip")
}

func main() {
	flag.Parse()
	h, err := parseConfigFile(configFile)
	if err != nil {
		log.Fatal("Unable to find / parse config file.")
	}
	http.Handle("/", h)
	http.ListenAndServe(serverBinding + ":" + serverPort, nil)
}
