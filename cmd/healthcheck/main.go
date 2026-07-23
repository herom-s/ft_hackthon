package main

import (
	"net/http"
	"os"
)

func main() {
	url := "http://localhost:8000/api/v1/health"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}
	resp, err := http.Get(url)
	if err != nil {
		os.Exit(1)
	}
	if resp.StatusCode != 200 {
		os.Exit(1)
	}
	os.Exit(0)
}
