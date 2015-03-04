package controllers

import (
	"log"
	"time"
	"container/list"


	"github.com/gorilla/websocket"
	"net/http"
	"sync/atomic"
	"sync"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool { return true },
}

type connection struct {
	// The websocket connection.
	ws *websocket.Conn
	// Buffered channel of outbound messages.
	send chan []byte
}

var alldata *list.List =list.New()
var datalock sync.RWMutex
// readPump pumps messages from the websocket connection to the hub.
func (c *connection) readPump() {
	defer func() {
		h.unregister <- c
		c.ws.Close()
	}()
	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })


	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		datalock.Lock()
		alldata.PushBack(message)
		datalock.Unlock()
		go handle(string(message))
	}
}
// write writes a message with the given message type and payload.
func (c *connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}
// writePump pumps messages from the hub to the websocket connection.
func (c *connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

var sessionNum int32=0
var sessionId int32=0
func EchoServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	c := &connection{send: make(chan []byte, 256), ws: ws}
	h.register <- c
	id:=atomic.AddInt32(&sessionId,1)
	num:=atomic.AddInt32(&sessionNum,1)
	log.Printf("session join, id:%d\n",id)
	log.Printf("%d active sessions\n",num)
	defer func(){
		num:=atomic.AddInt32(&sessionNum,-1)
		log.Printf("session leave, id:%d\n%d active sessions",id,num)
	}()
	go c.writePump()
	datalock.RLock()
	for e := alldata.Front(); e != nil; e = e.Next() {
		b := e.Value.([]byte)
		c.send<-b
	}
	datalock.RUnlock()
	c.readPump()
}

type hub struct {
	// Registered connections.
	connections map[*connection]bool
	// Inbound messages from the connections.
	broadcast chan []byte
	// Register requests from the connections.
	register chan *connection
	// Unregister requests from connections.
	unregister chan *connection
}

var h = hub{
	broadcast:   make(chan []byte),
	register:    make(chan *connection),
	unregister:  make(chan *connection),
	connections: make(map[*connection]bool),
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
		case c := <-h.unregister:
			if _, ok := h.connections[c]; ok {
				delete(h.connections, c)
				close(c.send)
			}
		case m := <-h.broadcast:
		for c := range h.connections {
			select {
			case c.send <- m:
			default:
				close(c.send)
				delete(h.connections, c)
			}
		}
		}
	}
}

func init() {

	go h.run()
}
