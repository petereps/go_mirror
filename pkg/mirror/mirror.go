package mirror

import (
	"log"

	"github.com/petereps/gomirror/pkg/docker"

	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/docker/docker/client"

	"github.com/sirupsen/logrus"
)

// Mirror proxies requests to an upstream server, and
// mirrors request to a mirror server
type Mirror struct {
	*httputil.ReverseProxy
	client *http.Client
	cfg    *Config
}

// New returns an initialized Mirror instance
func New(cfg *Config) (*Mirror, error) {
	primaryServerURL, err := url.Parse(cfg.Primary.URL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(primaryServerURL)
	if cfg.Primary.DockerLookup.Enabled {
		cli, err := client.NewEnvClient()
		if err != nil {
			log.Fatalf("could not get docker client: %+v", err)
		}

		dockerDNS := docker.NewDNSResolver(cli, cfg.Primary.DockerLookup.HostIdentifier)
		proxy = dockerDNS.ReverseProxy(primaryServerURL)
	}

	return &Mirror{
		proxy,
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
			Debugln("error in mirrored request")
		return
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		entry.WithError(err).
			Debugln("error reading mirrored request")
		return
	}

	entry.WithField("response", string(body)).
		Debugln("mirrored response")
}

func (m *Mirror) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// add path and query string (doing this manually so things like localhost work to mirror)
	path := r.URL.EscapedPath()
	query := r.URL.RawQuery
	proxyReqURL := m.cfg.Mirror.URL
	lastChar := proxyReqURL[len(proxyReqURL)-1]
	if lastChar == '/' {
		proxyReqURL = proxyReqURL[:len(proxyReqURL)-2]
	}
	if query != "" {
		query = "?" + query
	}
	proxyReqURL = fmt.Sprintf("%s%s%s", proxyReqURL, path, query)

	logrus.WithField("mirror_url", proxyReqURL).Debugln()

	proxyReq, proxyReqErr := http.NewRequest(
		r.Method, proxyReqURL, nil,
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
