package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	"github.com/tilt-dev/ephemerator/ephconfig"
	"github.com/tilt-dev/ephemerator/ephdash/pkg/env"
	"github.com/tilt-dev/ephemerator/ephdash/pkg/server"
)

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("kubernetes connection setup failed: %v", err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("kubernetes connection setup failed: %v", err)
	}

	envClient := env.NewClient(clientset, os.Getenv("NAMESPACE"))

	allowlist, err := ephconfig.ReadAllowlist()
	if err != nil {
		log.Fatal("server setup failed")
	}

	handler, err := server.NewServer(envClient, allowlist)
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", handler)

	fmt.Printf("Starting server at port 8080\n")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
