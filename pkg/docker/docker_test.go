package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/petereps/go_mirror/pkg/testutils"

	"github.com/stretchr/testify/assert"

	"github.com/docker/docker/client"
)

func TestContainer(t *testing.T) {
	r := testutils.GetServerContainer()

	r.Expire(300)

	client, err := client.NewEnvClient()
	assert.NoError(t, err)

	resolver := NewDNSResolver(client, "VIRTUAL_HOST")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go resolver.Listen(ctx)

	ip := resolver.IPAddress("testing-app.com")
	assert.NotEmpty(t, ip)

	res, err := http.Get(fmt.Sprintf("http://%s", ip))
	assert.NoError(t, err)

	body, err := ioutil.ReadAll(res.Body)
	assert.NoError(t, err)

	log.Println(string(body))

	assert.Equal(t, "hello world", string(body))

	r2 := testutils.GetServerContainer()
	defer r2.Close()
	r2.Expire(30)

	r.Close()
	<-time.After(10 * time.Second)

	newIP := resolver.IPAddress("testing-app.com")
	assert.NotEqual(t, newIP, ip)

	res, err = http.Get(fmt.Sprintf("http://%s", newIP))
	assert.NoError(t, err)

	body, err = ioutil.ReadAll(res.Body)
	assert.NoError(t, err)

	log.Println(string(body))

	assert.Equal(t, "hello world", string(body))

}
