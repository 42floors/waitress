package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"github.com/nfnt/resize"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
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
  Postfix   string // path postfix for the proxy
	Format    string // image format on the proxy
	Watermark string // filename of the watermark
	Port      string // Server port
	Binding   string // Server binding

	watermarkImage  image.Image
	backgroundColor color.Color
}

func (h proxyHandler) urlFor(u *url.URL) *url.URL {
	n := new(url.URL)
	n.Scheme = h.Scheme
	n.Host = h.Host

  if (len(strings.Split(u.Path, "/")) == 3) {
    n.Path = h.Prefix + strings.Join(strings.Split(u.Path, "/")[2:], "")
  } else {
      log.Println(u.Path);
    n.Path = h.Prefix + u.Path
  }

	n.Path = n.Path[0:strings.LastIndex(n.Path, ".")]
	n.Path = n.Path + h.Postfix + "." + h.Format

	n.RawQuery = ""
	return n
}

func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Recover from panic and throw a 500
	var (
		status           int
		duration         time.Duration
		err1, err2, err3 error
		resp             *http.Response
		m                image.Image
	)
	startTime := time.Now()

	resp, err1 = http.Get(h.urlFor(r.URL).String())
	defer resp.Body.Close()

	if err1 != nil || resp.StatusCode != http.StatusOK {
		status = resp.StatusCode
		w.WriteHeader(status)
	}

	if err1 == nil && resp.StatusCode == http.StatusOK {
		m, _, err2 = image.Decode(resp.Body)
		if err2 != nil {
			status = http.StatusInternalServerError
			w.WriteHeader(status)
		} else {
			status = http.StatusOK
			err3 = h.serveImage(w, r, m)
			if err3 != nil {
				status = http.StatusInternalServerError
				w.WriteHeader(status)
			}
		}
	}

	duration = time.Since(startTime)

	logStatement := "%v\t%v\n"
	logStatement += "                    Completed %d %v in %dms\n"
	if err1 != nil {
		logStatement += "                    error > " + err1.Error()
	}
	if err2 != nil {
		logStatement += "                    error > " + err2.Error()
	}
	if err3 != nil {
		logStatement += "                    error > " + err3.Error()
	}
	log.Printf(logStatement, r.Method, r.URL.String(), status, http.StatusText(status), int(duration.Nanoseconds()/1000000))
}

func (h *proxyHandler) resizeImage(m image.Image, options map[string]interface{}) image.Image {
	dstRect := image.Rect(0, 0, options["width"].(int), options["height"].(int))

    var backgroundColor color.Color

    if _, ok := options["backgroundColor"]; ok {
        backgroundColor = options["backgroundColor"].(color.Color)
    } else {
        backgroundColor = h.backgroundColor
    }

	if options["crop"].(bool) {
		m = resizeAndCrop(m, dstRect)
	} else if options["enforce"].(bool) {
		m = resize.Resize(uint(dstRect.Dx()), uint(dstRect.Dy()), m, resize.MitchellNetravali)
	} else if options["minimum"].(bool) {
		srcRect := m.Bounds()
		srcRatio := float32(srcRect.Dx()) / float32(srcRect.Dy())
		dstRatio := float32(dstRect.Dx()) / float32(dstRect.Dy())
		if srcRatio > dstRatio {
			dstRect.Max.X = int(float32(dstRect.Max.Y) * srcRatio)
		} else {
			dstRect.Max.Y = int(float32(dstRect.Max.X) / srcRatio)
		}

		m = resize.Resize(uint(dstRect.Dx()), uint(dstRect.Dy()), m, resize.MitchellNetravali)
	} else {

		m = resizeAndPad(m, dstRect, backgroundColor)
	}

	return m
}

func (h *proxyHandler) serveImage(w http.ResponseWriter, r *http.Request, m image.Image) error {
	options := extractOptions(r, m)
	m = h.resizeImage(m, options)

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

	etag := md5.New()
	body := new(bytes.Buffer)
	bodyWriter := io.MultiWriter(body, etag)

	switch options["mimeType"].(string) {
	case "image/jpeg":
		err = jpeg.Encode(bodyWriter, m, &jpeg.Options{Quality: 85})
	case "image/png":
		err = png.Encode(bodyWriter, m)
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("ETag", hex.EncodeToString(etag.Sum(nil)))
		body.WriteTo(w)
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

	if (h.Watermark != "") {
		file, err = os.Open(h.Watermark)
		h.watermarkImage, _, err = image.Decode(file)
		if err != nil {
			return nil, err
		}
	}

	h.backgroundColor = color.RGBA{80, 80, 80, 1}
	return h, nil
}

var configFile string
var serverPort string
var serverBinding string

func init() {
	flag.StringVar(&configFile, "config", "", "conf file (see config.yml.sample)")
	flag.StringVar(&serverPort, "port", "3000", "run the server on the specified port")
	flag.StringVar(&serverBinding, "binding", "0.0.0.0", "bind the server to the specified ip")
}

func main() {
	flag.Parse()

	var (
		h *proxyHandler
		err error
	)

	if (configFile != "") {
		h, err = parseConfigFile(configFile)

    if err != nil {
      log.Fatal("Unable to find / parse config file (does the watermark exists?).")
    }
  }

	if (h != nil) {
		if (h.Port != "" && serverPort == "3000") {
			serverPort = h.Port;
		}

		if (h.Binding != "" && serverBinding == "0.0.0.0") {
			serverBinding = h.Binding
		}
	}

	if (h.Port != "") {
		serverPort = h.Port;
	}

	if (h.Binding != "") {
	  serverBinding = h.Binding
	}

	http.Handle("/", h)
	err = http.ListenAndServe(serverBinding + ":" + serverPort, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
