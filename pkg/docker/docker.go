package docker

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/sirupsen/logrus"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

// DNSResolver uses the local docker sock to resolve ip addresses,
// based on the identifier provided
type DNSResolver struct {
	currentIP      string
	cli            *client.Client
	hostToIP       map[string]string
	mux            sync.Mutex
	debounce       *time.Timer
	hostIdentifier string
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")

	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

// ReverseProxy will return a httputil.ReverseProxy that uses the IPAddress
// call of DNSResolver instead of the systems dns call
func (d *DNSResolver) ReverseProxy(target *url.URL) *httputil.ReverseProxy {
	targetQuery := target.RawQuery

	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		lookup := target.Hostname()

		address := d.IPAddress(lookup)
		if address == "" {
			logrus.Errorf("no host found in docker for request host %s. Defering to system dns", lookup)
			address = lookup
		} else {
			logrus.WithField("ip", address).Infoln("found")
		}

		if target.Port() != "" {
			address = address + ":" + target.Port()
		}

		req.URL.Host = address

		req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}

		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}

	}

	return &httputil.ReverseProxy{Director: director}
}

// NewDNSResolver returns an initialized DNSResolver, with ip addresses
// filled in at the time of creation
func NewDNSResolver(client *client.Client, hostIdentifier string) *DNSResolver {
	d := &DNSResolver{cli: client, hostToIP: make(map[string]string), hostIdentifier: hostIdentifier}
	d.lookupIPs(nil)
	go d.Listen(context.Background())
	return d
}

// IPAddress gets an ip address by host name
func (d *DNSResolver) IPAddress(host string) string {
	return d.hostToIP[host]
}

func (d *DNSResolver) lookupIPs(event interface{}) {
	if d.debounce == nil {
		d.debounce = time.NewTimer(500 * time.Millisecond)
		<-d.debounce.C
	} else {
		d.debounce.Reset(500 * time.Millisecond)
		return
	}

	logrus.WithField("event", fmt.Sprintf("%+v", event)).Debugln("looking up ip")

	d.debounce = nil

	cli := d.cli

	args := filters.NewArgs()
	args.Add("status", "running")
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{Filters: args})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		c, err := cli.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			continue
		}

		for _, env := range c.Config.Env {
			kv := strings.SplitN(env, "=", 2)
			if len(kv) < 2 {
				continue
			}

			if key := kv[0]; key == d.hostIdentifier {
				host := kv[1]
				for _, n := range c.NetworkSettings.Networks {
					if n.IPAddress != "" {
						logrus.WithField("host", host).WithField("ip_address", n.IPAddress).Infoln()
						d.mux.Lock()
						d.hostToIP[host] = n.IPAddress
						d.mux.Unlock()
						break
					}
				}
			}
		}

	}
}

// Listen listens for docker events, and resolves ips when they occur
func (d *DNSResolver) Listen(ctx context.Context) error {
	cli := d.cli

	args := filters.NewArgs()

	messages, errC := cli.Events(ctx, types.EventsOptions{Filters: args})

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errC:
			return err
		case event, ok := <-messages:
			if !ok {
				return nil
			}
			go d.lookupIPs(event)
		}
	}
}
