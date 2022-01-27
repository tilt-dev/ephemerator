package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/tilt-dev/ephemerator/ephconfig"
	"github.com/tilt-dev/ephemerator/ephdash/pkg/server"
)

func main() {
	fmt.Printf("Starting server at port 8080\n")

	allowlist, err := ephconfig.ReadAllowlist()
	if err != nil {
		log.Fatal("server setup failed")
		os.Exit(1)
	}

	handler, err := server.NewServer(allowlist)
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", handler)

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
