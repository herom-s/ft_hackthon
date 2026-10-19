package client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

var wsDialTimeout = 10 * time.Second

type WSClient struct {
	baseURL  string
	token    string
	insecure bool
	conn     *websocket.Conn
}

func (ac *APIClient) newWSClient() *WSClient {
	return &WSClient{baseURL: ac.baseURL, token: ac.token, insecure: ac.insecure}
}

// Watch streams status updates for a job. The channel closes when the job
// reaches a terminal state, the server sends something that is not a status,
// the connection drops, or done is closed; callers fall back to HTTP polling
// when it closes before a terminal status arrives.
func (w *WSClient) Watch(jobID string, done <-chan struct{}) (<-chan *StatusResponse, error) {
	u, err := url.Parse(w.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s/ws/grade/status/%s", scheme, u.Host, url.PathEscape(jobID))

	dialer := websocket.Dialer{
		HandshakeTimeout: wsDialTimeout,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: w.insecure},
	}
	// The token goes in a header, not the query string, so it stays out of
	// proxy and server access logs.
	c, _, err := dialer.Dial(wsURL, http.Header{"Authorization": {"Bearer " + w.token}})
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	w.conn = c

	updates := make(chan *StatusResponse)
	go func() {
		defer close(updates)
		defer c.Close()
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			var resp StatusResponse
			if err := json.Unmarshal(msg, &resp); err != nil || resp.Status == "" {
				return
			}
			select {
			case updates <- &resp:
			case <-done:
				return
			}
			if isTerminalStatus(resp.Status) {
				return
			}
		}
	}()
	return updates, nil
}

func (w *WSClient) Close() {
	if w.conn != nil {
		w.conn.Close()
	}
}

func isTerminalStatus(status string) bool {
	return status == "completed" || status == "failed" || status == "error"
}
