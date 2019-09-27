package mirror

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
)

// Mirror proxies requests to an upstream server, and
// mirrors request to a mirror server
type Mirror struct {
	*httputil.ReverseProxy
	upstreamMirror *url.URL
	client         *http.Client
}

// New returns an initialized Mirror instance
func New(primaryServer, mirrorServer string) (*Mirror, error) {
	primaryServerURL, err := url.Parse(primaryServer)
	if err != nil {
		return nil, err
	}

	mirrorServerURL, err := url.Parse(mirrorServer)
	if err != nil {
		return nil, err
	}

	return &Mirror{
		httputil.NewSingleHostReverseProxy(primaryServerURL),
		mirrorServerURL,
		&http.Client{
			Timeout: time.Minute * 1,
		},
	}, nil
}

func (m *Mirror) mirror(proxyReq *http.Request) {
	entry := logrus.WithField("url", proxyReq.URL.String())

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
	// we need to buffer the body if we want to read it here and send it
	// in the request.
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	mirroredURL := r.URL
	mirroredURL.Host = m.upstreamMirror.Host
	if mirroredURL.Scheme == "" {
		mirroredURL.Scheme = "http"
	}
	proxyReq, proxyReqErr := http.NewRequest(
		r.Method, mirroredURL.String(), r.Body,
	)
	if proxyReqErr != nil {
		logrus.WithError(err).
			Errorln("error creating mirroring request")
	}

	proxyReq.Header = r.Header

	r.Body = ioutil.NopCloser(bytes.NewReader(body))
	proxyReq.Body = ioutil.NopCloser(bytes.NewReader(body))

	if proxyReqErr == nil {
		go m.mirror(proxyReq)
	}
	m.ReverseProxy.ServeHTTP(w, r)
}

// Serve serves the mirror
func (m *Mirror) Serve(address string) error {
	return http.ListenAndServe(address, m)
}
