package mirror

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/petereps/gomirror/pkg/testutils"

	"github.com/stretchr/testify/assert"
)

func assertHeaders(t *testing.T, headers []Header, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, pair := range headers {
			assert.Equal(t, pair.Value, r.Header.Get(pair.Key))
		}

		next.ServeHTTP(w, r)
	})
}

func assertNoHeaders(t *testing.T, headers []Header, next http.Handler) http.Handler {
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

	cfg := &Config{
		Primary: PrimaryConfig{
			URL:             backendServer.URL,
			DoMirrorBody:    true,
			DoMirrorHeaders: true,
		},
		Mirror: MirrorConfig{URL: mirroredServer.URL},
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

	mirrorHeaders := []Header{
		Header{
			Key:   "x-my-header",
			Value: "my-special-header",
		},
		Header{
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

	cfg := &Config{
		Primary: PrimaryConfig{
			URL:             backendServer.URL,
			DoMirrorBody:    true,
			DoMirrorHeaders: true,
		},
		Mirror: MirrorConfig{
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

	cfg := &Config{
		Primary: PrimaryConfig{
			URL:          backendServer.URL,
			DoMirrorBody: false,
		},
		Mirror: MirrorConfig{
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

func TestPaths(t *testing.T) {
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/primary", r.URL.EscapedPath())
		assert.Equal(t, "testing=123", r.URL.RawQuery)
		println(r.URL.String())
		w.Write([]byte("primary"))
	})
	backendServer := httptest.NewServer(assertBody(t, "hello", final))
	defer backendServer.Close()

	done := make(chan struct{})

	mirrorFinal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("mirror"))
		println(r.URL.String())
		assert.Equal(t, "testing=123", r.URL.RawQuery)
		assert.Equal(t, "/mirror/primary", r.URL.EscapedPath())
		close(done)
	})

	mirroredServer := httptest.NewServer(
		assertBody(t, "", mirrorFinal),
	)
	defer mirroredServer.Close()

	println(backendServer.URL)
	println(mirroredServer.URL)

	cfg := &Config{
		Primary: PrimaryConfig{
			URL:          backendServer.URL,
			DoMirrorBody: false,
		},
		Mirror: MirrorConfig{
			URL: mirroredServer.URL + "/mirror",
		},
	}
	mirror, err := New(cfg)
	assert.NoError(t, err)

	mirrorProxy := httptest.NewServer(mirror)
	defer mirrorProxy.Close()

	body := strings.NewReader("hello")

	proxyReq, err := http.NewRequest(
		http.MethodPost,
		mirrorProxy.URL+"/primary?testing=123",
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

func TestDockerProxy(t *testing.T) {
	r := testutils.GetServerContainer()
	r.Expire(30)

	done := make(chan struct{})

	mirrorFinal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("mirror"))
		println(r.URL.String())
		assert.Equal(t, "testing=123", r.URL.RawQuery)
		assert.Equal(t, "/mirror/primary", r.URL.EscapedPath())
		close(done)
	})

	mirroredServer := httptest.NewServer(
		assertBody(t, "", mirrorFinal),
	)
	defer mirroredServer.Close()

	cfg := &Config{
		Primary: PrimaryConfig{
			URL:          "http://testing-app.com",
			DoMirrorBody: false,
			DockerLookup: DockerLookupConfig{
				Enabled:        true,
				HostIdentifier: "VIRTUAL_HOST",
			},
		},
		Mirror: MirrorConfig{
			URL: mirroredServer.URL + "/mirror",
		},
	}
	mirror, err := New(cfg)
	assert.NoError(t, err)

	mirrorProxy := httptest.NewServer(mirror)
	defer mirrorProxy.Close()

	proxyReq, err := http.NewRequest(
		http.MethodGet,
		mirrorProxy.URL+"/primary?testing=123",
		nil,
	)
	assert.NoError(t, err)

	client := &http.Client{}

	response, err := client.Do(proxyReq)
	assert.NoError(t, err)

	resStr, err := ioutil.ReadAll(response.Body)
	assert.NoError(t, err)
	assert.Equal(t, string(resStr), "hello world")

	select {
	case <-time.After(5 * time.Second):
		panic("timed out waiting for mirror")
	case <-done:
	}

	done = make(chan struct{})
	// change the ip
	r2 := testutils.GetServerContainer()
	defer r2.Close()
	r2.Expire(30)

	r.Close()
	<-time.After(5 * time.Second)

	proxyReq, err = http.NewRequest(
		http.MethodGet,
		mirrorProxy.URL+"/primary?testing=123",
		nil,
	)
	assert.NoError(t, err)

	client = &http.Client{}

	response, err = client.Do(proxyReq)
	assert.NoError(t, err)

	resStr, err = ioutil.ReadAll(response.Body)
	assert.NoError(t, err)
	assert.Equal(t, string(resStr), "hello world")

	select {
	case <-time.After(5 * time.Second):
		panic("timed out waiting for mirror")
	case <-done:
	}
}
