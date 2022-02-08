package main

import (
	"context"
	"flag"
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

var authFakeUser = flag.String(
	"auth-fake-user", "",
	"When specified, we'll use a fake default user instead of requesting a user from the oauth proxy.")

var authProxy = flag.String(
	"auth-proxy", "",
	"URL of the oauth2-proxy inside the cluster, e.g., 'http://oauth-proxy'. Must not end in a slash.")

func main() {
	flag.Parse()

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("kubernetes connection setup failed: %v", err)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("kubernetes connection setup failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	envClient := env.NewClient(ctx, clientset, os.Getenv("NAMESPACE"), os.Getenv("EPH_SLACK_WEBHOOK"))

	allowlist, err := ephconfig.ReadAllowlist()
	if err != nil {
		log.Fatal("server setup failed")
	}

	gatewayHost, err := ephconfig.ReadGatewayHost()
	if err != nil {
		log.Fatal("server setup failed")
	}

	authSettings := server.AuthSettings{
		FakeUser: *authFakeUser,
		Proxy:    *authProxy,
	}

	err = authSettings.Validate()
	if err != nil {
		log.Fatalf("server setup failed: %v", err)
	}

	handler, err := server.NewServer(envClient, allowlist, gatewayHost, authSettings)
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", handler)

	fmt.Printf("Starting server at port 8080\n")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
