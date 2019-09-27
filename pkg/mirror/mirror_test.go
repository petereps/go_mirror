package mirror

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMirrorProxy(t *testing.T) {
	primaryResponse := []byte("primary")
	mirrorResponse := []byte("second")
	primaryResponseCode := http.StatusAccepted
	mirrorReceived := false

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(primaryResponseCode)
		w.Write(primaryResponse)
	}))
	defer backendServer.Close()

	done := make(chan struct{})
	mirroredServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "this call was relayed by the mirror proxy")
		w.Write(mirrorResponse)
		mirrorReceived = true
		close(done)
	}))
	defer mirroredServer.Close()

	mirror, err := New(backendServer.URL, mirroredServer.URL)
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

	println(string(resStr))
	assert.Equal(t, string(primaryResponse), string(resStr))

	select {
	case <-time.After(5 * time.Second):
		panic("timed out waiting for mirror")
	case <-done:
	}

	assert.True(t, mirrorReceived)
}
