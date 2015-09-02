package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/websocket"
)

var (
	apiServer   string
	nginxServer string
)

type Object struct {
	Object Endpoints `json:"object"`
}

type Endpoints struct {
	Kind       string   `json:"kind"`
	ApiVersion string   `json:"apiVersion"`
	Metadata   Metadata `json:"metadata"`
	Subsets    []Subset `json:"subsets"`
}

type Metadata struct {
	Name string `json:"name"`
}

type Subset struct {
	Addresses []Address `json:"addresses"`
	Ports     []Port    `json:"ports"`
}

type Address struct {
	IP string `json:"ip"`
}

type Port struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

type NginxResponse struct {
	Upstreams map[string][]Backend `json:"upstreams"`
}

type Backend struct {
	ID     int    `json:"id"`
	Server string `json:"server"`
}

func NginxStatus() (*NginxResponse, error) {
	resp, err := http.Get("http://104.154.85.118:9090/status")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Non 200 OK")
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var er NginxResponse
	if err := json.Unmarshal(data, &er); err != nil {
		return nil, err
	}
	return &er, nil
}

func init() {
	flag.StringVar(&apiServer, "api-server", "127.0.0.1:8080", "Kubernetes API server for watching endpoints. (ip:port)")
	flag.StringVar(&nginxServer, "nginx-server", "", "Nginx server for managing backends. (ip:port)")
}

func main() {

	flag.Parse()

	origin := "http://localhost"
	url := fmt.Sprintf("ws://%s/api/v1/watch/endpoints", apiServer)
	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		log.Fatal(err)
	}

	for {
		var ep Object
		if err := websocket.JSON.Receive(ws, &ep); err != nil {
			log.Println(err)
			time.Sleep(time.Duration(2 * time.Second))
		}

		nr, err := NginxStatus()
		if err != nil {
			log.Println(err)
			time.Sleep(time.Duration(2 * time.Second))
		}

		upstreamName := ep.Object.Metadata.Name
		upstreams, ok := nr.Upstreams[upstreamName]
		if !ok {
			log.Printf("no matching upstream for service %s skipping...", upstreamName)
			continue
		}

		for _, subset := range ep.Object.Subsets {
		RowLoop:
			for _, address := range subset.Addresses {
				server := net.JoinHostPort(address.IP, "80")

				for _, backend := range upstreams {
					if server == backend.Server {
						continue RowLoop
					}
				}

				log.Printf("registering backend %s with %s ...", server, upstreamName)
				url := fmt.Sprintf("http://%s/upstream_conf?add=&upstream=%s&server=%s", nginxServer, upstreamName, server)
				resp, err := http.Get(url)
				if err != nil {
					log.Fatal(err)
				}
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					log.Fatal(errors.New("Non 200 OK"))
				}
			}
		}

		// Remove old backends
		for _, backend := range upstreams {
			for _, subset := range ep.Object.Subsets {

				var server string
				hasBackend := false
				for _, address := range subset.Addresses {
					server = net.JoinHostPort(address.IP, "80")
					if server == backend.Server {
						hasBackend = true
						break
					}
				}

				if !hasBackend {
					log.Printf("removing backend %s [#%d] from %s ...", backend.Server, backend.ID, upstreamName)
					url := fmt.Sprintf("http://104.154.85.118:32771/upstream_conf?remove=&upstream=%s&id=%d", upstreamName, backend.ID)
					resp, err := http.Get(url)
					if err != nil {
						log.Println(err)
					}
					data, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						log.Println(err)
					}
					if resp.StatusCode != http.StatusOK {
						log.Println(string(data))
						log.Println(errors.New("Non 200 OK"), resp.StatusCode)
					}
				}
			}
		}
	}
}
