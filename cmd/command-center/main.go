package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	commandcenter "github.com/kenny/command-center"
	"github.com/kenny/command-center/internal/server"
)

const defaultAddr = ":8080"

func main() {
	dev := flag.Bool("dev", false, "proxy frontend requests to Vite dev server")
	flag.Parse()

	mux := http.NewServeMux()

	if *dev {
		viteURL := "http://localhost:5173"
		proxy, err := server.NewDevProxyHandler(viteURL)
		if err != nil {
			log.Fatalf("failed to create dev proxy: %v", err)
		}
		mux.Handle("/", proxy)
		fmt.Printf("Dev mode: proxying to Vite at %s\n", viteURL)
	} else {
		handler, err := server.NewSPAHandler(commandcenter.WebFS, "web/build")
		if err != nil {
			log.Fatalf("failed to create SPA handler: %v", err)
		}
		mux.Handle("/", handler)
	}

	fmt.Printf("Listening on %s\n", defaultAddr)
	log.Fatal(http.ListenAndServe(defaultAddr, mux))
}
