package mirror

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/petereps/go_mirror/pkg/config"
	"github.com/stretchr/testify/assert"
)

func assertHeaders(t *testing.T, headers []config.Header, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, pair := range headers {
			assert.Equal(t, pair.Value, r.Header.Get(pair.Key))
		}

		next.ServeHTTP(w, r)
	})
}

func assertNoHeaders(t *testing.T, headers []config.Header, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, pair := range headers {
			assert.Empty(t, r.Header.Get(pair.Key))
		}

		next.ServeHTTP(w, r)
	})
}

func assertBody(t *testing.T, body string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, body, string(requestBody))
		next.ServeHTTP(w, r)
	})
}

func returnBody(body string, responseCode int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(responseCode)
		w.Write([]byte(body))
	})
}

func TestMirrorProxyResponse(t *testing.T) {
	primaryResponse := []byte("primary")
	mirrorResponse := []byte("second")
	primaryResponseCode := http.StatusAccepted
	mirrorReceived := false

	final := returnBody(string(primaryResponse), primaryResponseCode)
	backendServer := httptest.NewServer(
		assertBody(t, "hello", final),
	)
	defer backendServer.Close()

	done := make(chan struct{})

	finalMirror := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the mirror proxy")
		w.Write(mirrorResponse)
		mirrorReceived = true
		close(done)
	})
	mirroredServer := httptest.NewServer(assertBody(t, "hello", finalMirror))
	defer mirroredServer.Close()

	cfg := &config.Config{
		Primary: config.PrimaryConfig{
			URL:             backendServer.URL,
			DoMirrorBody:    true,
			DoMirrorHeaders: true,
		},
		Mirror: config.MirrorConfig{URL: mirroredServer.URL},
	}
	mirror, err := New(cfg)
	assert.NoError(t, err)

	mirrorProxy := httptest.NewServer(mirror)
	defer mirrorProxy.Close()

	body := strings.NewReader("hello")

	proxyReq, err := http.NewRequest(
		http.MethodPost,
		mirrorProxy.URL,
		body,
	)
	assert.NoError(t, err)

	client := &http.Client{}

	response, err := client.Do(proxyReq)
	assert.NoError(t, err)

	resStr, err := ioutil.ReadAll(response.Body)
	assert.NoError(t, err)

	assert.Equal(t, string(primaryResponse), string(resStr))

	select {
	case <-time.After(5 * time.Second):
		panic("timed out waiting for mirror")
	case <-done:
	}

	assert.True(t, mirrorReceived)
}

func TestMirrorProxyHeaders(t *testing.T) {
	primaryResponse := []byte("primary")
	mirrorResponse := []byte("second")
	primaryResponseCode := http.StatusAccepted
	mirrorReceived := false

	mirrorHeaders := []config.Header{
		config.Header{
			Key:   "x-my-header",
			Value: "my-special-header",
		},
		config.Header{
			Key:   "x-my-other-header",
			Value: "my-other-special-header",
		},
	}

	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(primaryResponseCode)
		w.Write(primaryResponse)
	})
	backendServer := httptest.NewServer(assertNoHeaders(t, mirrorHeaders, final))
	defer backendServer.Close()

	done := make(chan struct{})

	mirrorFinal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the mirror proxy")
		w.Write(mirrorResponse)
		mirrorReceived = true
		close(done)
	})

	mirroredServer := httptest.NewServer(
		assertHeaders(t, mirrorHeaders, mirrorFinal),
	)
	defer mirroredServer.Close()

	cfg := &config.Config{
		Primary: config.PrimaryConfig{
			URL:             backendServer.URL,
			DoMirrorBody:    true,
			DoMirrorHeaders: true,
		},
		Mirror: config.MirrorConfig{
			URL:     mirroredServer.URL,
			Headers: mirrorHeaders,
		},
	}
	mirror, err := New(cfg)
	assert.NoError(t, err)

	mirrorProxy := httptest.NewServer(mirror)
	defer mirrorProxy.Close()

	body := strings.NewReader("hello")

	proxyReq, err := http.NewRequest(
		http.MethodPost,
		mirrorProxy.URL,
		body,
	)
	assert.NoError(t, err)

	client := &http.Client{}

	response, err := client.Do(proxyReq)
	assert.NoError(t, err)

	resStr, err := ioutil.ReadAll(response.Body)
	assert.NoError(t, err)

	assert.Equal(t, string(primaryResponse), string(resStr))

	select {
	case <-time.After(5 * time.Second):
		panic("timed out waiting for mirror")
	case <-done:
	}

	assert.True(t, mirrorReceived)
}

func TestMirrorNoBody(t *testing.T) {
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("primary"))
	})
	backendServer := httptest.NewServer(assertBody(t, "hello", final))
	defer backendServer.Close()

	done := make(chan struct{})

	mirrorFinal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("mirror"))
		close(done)
	})

	mirroredServer := httptest.NewServer(
		assertBody(t, "", mirrorFinal),
	)
	defer mirroredServer.Close()

	cfg := &config.Config{
		Primary: config.PrimaryConfig{
			URL:          backendServer.URL,
			DoMirrorBody: false,
		},
		Mirror: config.MirrorConfig{
			URL: mirroredServer.URL,
		},
	}
	mirror, err := New(cfg)
	assert.NoError(t, err)

	mirrorProxy := httptest.NewServer(mirror)
	defer mirrorProxy.Close()

	body := strings.NewReader("hello")

	proxyReq, err := http.NewRequest(
		http.MethodPost,
		mirrorProxy.URL,
		body,
	)
	assert.NoError(t, err)

	client := &http.Client{}

	response, err := client.Do(proxyReq)
	assert.NoError(t, err)

	resStr, err := ioutil.ReadAll(response.Body)
	assert.NoError(t, err)
	assert.Equal(t, string(resStr), "primary")

	select {
	case <-time.After(5 * time.Second):
		panic("timed out waiting for mirror")
	case <-done:
	}
}
