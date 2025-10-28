package web

import (
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// tryGorillaUpgrade upgrades using gorilla/websocket and returns a lightweight
// writer interface so callers don't need to import the concrete type.
func tryGorillaUpgrade(w http.ResponseWriter, r *http.Request) (interface {
	WriteMessage(int, []byte) error
	Close() error
}, interface{ Close() error }, bool) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, nil, false
	}
	return conn, conn, true
}
