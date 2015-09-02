package main

import (
	"log"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
)

func main() {
	config := &client.Config{
		Host: "http://127.0.0.1:8080",
	}
	c, err := client.New(config)
	if err != nil {
		log.Println(err)
	}

	endpoints := c.Endpoints(api.NamespaceDefault)
	watch, err := endpoints.Watch(labels.Everything(), fields.Everything(), "")
	if err != nil {
		log.Println(err)
	}

	for result := range watch.ResultChan() {
		log.Println(result.Object)
	}
}
