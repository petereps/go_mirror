package mirror

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/petereps/go_mirror/pkg/config"

	"github.com/sirupsen/logrus"
)

// Mirror proxies requests to an upstream server, and
// mirrors request to a mirror server
type Mirror struct {
	*httputil.ReverseProxy
	client *http.Client
	cfg    *config.Config
}

// New returns an initialized Mirror instance
func New(cfg *config.Config) (*Mirror, error) {
	primaryServerURL, err := url.Parse(cfg.Primary.URL)
	if err != nil {
		return nil, err
	}

	return &Mirror{
		httputil.NewSingleHostReverseProxy(primaryServerURL),
		&http.Client{
			Timeout: time.Minute * 1,
		},
		cfg,
	}, nil
}

func (m *Mirror) mirror(proxyReq *http.Request) {
	entry := logrus.WithField("mirror_url", proxyReq.URL.String())
	entry.Debugln("mirroring")
	response, err := m.client.Do(proxyReq)
	if err != nil {
		entry.WithError(err).
			Errorln("error in mirrored request")
		return
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		entry.WithError(err).
			Errorln("error reading mirrored request")
		return
	}

	entry.WithField("response", string(body)).
		Debugln("mirrored response")
}

func (m *Mirror) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	proxyReq, proxyReqErr := http.NewRequest(
		r.Method, m.cfg.Mirror.URL, nil,
	)
	if proxyReqErr != nil {
		logrus.WithError(proxyReqErr).
			Errorln("error creating mirroring request")
	}

	if m.cfg.Primary.DoMirrorBody {
		// we need to buffer the body in order to send to both upstream
		// requests
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		r.Body = ioutil.NopCloser(bytes.NewReader(body))
		proxyReq.Body = ioutil.NopCloser(bytes.NewReader(body))
	}

	if m.cfg.Primary.DoMirrorHeaders {
		for key, value := range r.Header {
			proxyReq.Header.Set(key, value[0])
		}
	}

	for _, header := range m.cfg.Mirror.Headers {
		proxyReq.Header.Set(header.Key, header.Value)
	}

	for _, header := range m.cfg.Primary.Headers {
		r.Header.Set(header.Key, header.Value)
	}

	if proxyReqErr == nil {
		go m.mirror(proxyReq)
	}
	m.ReverseProxy.ServeHTTP(w, r)
}

// Serve serves the mirror
func (m *Mirror) Serve(address string) error {
	return http.ListenAndServe(address, m)
}
