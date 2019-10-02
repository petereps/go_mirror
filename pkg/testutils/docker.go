package testutils

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/ory/dockertest"
)

var pool *dockertest.Pool

var containers int

// GetServerContainer returns an ephemeral test http server docker container,
// that responds "hello world" from GET requests on any path
func GetServerContainer() *dockertest.Resource {
	var err error
	if pool == nil {
		pool, err = dockertest.NewPool("")
	}
	if err != nil {
		panic(err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:       fmt.Sprintf("test-hello-world-%d", containers),
		Repository: "thelonix/http-hello-world",
		Tag:        "latest",
		Env:        []string{"VIRTUAL_HOST=testing-app.com"},
	})

	containers++

	if err != nil {
		panic(err)
	}

	var port string
	if err = pool.Retry(func() (err error) {
		defer func() {
			if err == nil {
				return
			}
			logrus.WithError(err).Errorln("error")
		}()

		port = resource.GetPort("80/tcp")
		r, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s", port))
		if err != nil {
			return err
		}
		_, err = ioutil.ReadAll(r.Body)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	return resource
}
