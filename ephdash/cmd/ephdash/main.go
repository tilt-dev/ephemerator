package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/tilt-dev/ephemerator/ephdash/pkg/server"
)

func main() {
	fmt.Printf("Starting server at port 8080\n")

	handler, err := server.NewServer()
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", handler)

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
