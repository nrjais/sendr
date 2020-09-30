package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"

	"github.com/pion/webrtc/v3"
)

func signalCandidate(to string, c *webrtc.ICECandidate) error {
	payload := []byte(c.ToJSON().Candidate)
	resp, err := http.Post(fmt.Sprintf("http://localhost:8080/candidate/%s", to),
		"application/json; charset=utf-8", bytes.NewReader(payload))

	if err != nil {
		return err
	}

	if closeErr := resp.Body.Close(); closeErr != nil {
		return closeErr
	}

	return nil
}

func main() {
	cmd := os.Args[1]
	if cmd == "send" {
		server()
	} else if cmd == "client" {
		client()
	} else {
		httpServer()
	}
}
