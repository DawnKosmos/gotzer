package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, Gotzer! Running on Hetzner Cloud 🚀\n")
		fmt.Fprintf(w, "Project: %s\n", os.Getenv("APP_NAME"))
		fmt.Fprintf(w, "Environment: %s\n", os.Getenv("APP_ENV"))
	})

	fmt.Printf("Server starting on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
		os.Exit(1)
	}
}
