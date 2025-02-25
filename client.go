package main

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	writeWait      = 100 * time.Second
	pongWait       = 600 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       *chan []byte
	stopSignal chan interface{}
	receiver   *ReceiverHub
	mutex      *sync.Mutex
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		log.Printf("message:", message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		var m Message
		err = json.Unmarshal(message, &m)
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		if err != nil {
			log.Printf("error: %v", err)
			c.hub.broadcast <- message
		} else {
			switch m.Command {
			case "list":
				*c.send <- screenCaptureDevices()
			case "activate":
				udid, err := formatUdid(m.UDID)
				if err != nil {
					*c.send <- toErrJSON(err, "Failed to activate")
				} else {
					*c.send <- activate(udid)
				}
			case "stream":
				log.Info("command: \"stream\"")
				udid, err := formatUdid(m.UDID)
				if err != nil {
					*c.send <- toErrJSON(err, "Failed to activate")
				} else {
					c.stream(udid)
				}
			default:
				c.hub.broadcast <- message
			}
		}
	}
}
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-*c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) stop() {
	if c.send == nil {
		log.Warn("Client.stop() called more then once")
		return
	}
	c.mutex.Lock()
	close(*c.send)
	c.send = nil
	if c.stopSignal != nil {
		select {
		case c.stopSignal <- nil:
		default:
		}
	}
	c.mutex.Unlock()
}

func (c *Client) stream(udid string) {
	log.Debugf("create receiver hub for %s", udid)
	c.receiver = c.hub.getOrCreateReceiver(udid)
	go func() {
		c.receiver.AddClient(c)
	}()
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	send := make(chan []byte, 256)
	client := &Client{hub: hub, conn: conn, send: &send, mutex: &sync.Mutex{}}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()

	if r.URL.RawQuery != "" {
		m, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			log.Errorln("Failed to parse query string:" + r.URL.RawQuery)
			return
		}

		rawUdid := m.Get("stream")
		if rawUdid != "" {
			udid, err := formatUdid(rawUdid)
			if err != nil {
				*client.send <- toErrJSON(err, "Failed to start stream")
			} else {
				client.stream(udid)
			}
		}
	}
}
