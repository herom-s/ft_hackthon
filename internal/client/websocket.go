package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	baseURL string
	token   string
	conn    *websocket.Conn
}

func NewWSClient(baseURL, token string) *WSClient {
	return &WSClient{baseURL: baseURL, token: token}
}

func (w *WSClient) ListenStatus(jobID string, onUpdate func(*StatusResponse)) error {
	u, err := url.Parse(w.baseURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}

	wsURL := fmt.Sprintf("%s://%s/ws/grade/status/%s?token=%s", scheme, u.Host, jobID, w.token)

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	w.conn = c

	go func() {
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}

			var resp StatusResponse
			if err := json.Unmarshal(msg, &resp); err != nil {
				continue
			}

			if onUpdate != nil {
				onUpdate(&resp)
			}

			if resp.Status == "completed" || resp.Status == "failed" || resp.Status == "error" {
				c.Close()
				return
			}
		}
	}()

	return nil
}

func (w *WSClient) Close() {
	if w.conn != nil {
		w.conn.Close()
	}
}

var wsDialTimeout = 10 * time.Second
