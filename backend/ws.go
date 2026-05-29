package backend

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]string // conn → username ("" when auth disabled)
}

var wsHub = &hub{clients: map[*websocket.Conn]string{}}

func (h *hub) register(conn *websocket.Conn, username string) {
	h.mu.Lock()
	h.clients[conn] = username
	h.mu.Unlock()
}

func (h *hub) unregister(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}

// Broadcast sends an "update" reload signal to all connected clients.
func Broadcast() {
	wsHub.broadcast("update", "")
}

// BroadcastAll sends msg to every connected client regardless of username.
func BroadcastAll(msg string) {
	wsHub.broadcast(msg, "")
}

// BroadcastToUser sends msg only to connections owned by username.
func BroadcastToUser(username, msg string) {
	wsHub.broadcast(msg, username)
}

func (h *hub) broadcast(msg, username string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sent := 0
	for conn, u := range h.clients {
		if username != "" && u != username {
			continue
		}
		conn.WriteMessage(websocket.TextMessage, []byte(msg)) //nolint:errcheck
		sent++
	}
	Log.Infow("ws broadcast", "msg", msg, "targetUser", username, "totalClients", len(h.clients), "sentTo", sent)
}

func WsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	username := usernameFromRequest(r)
	wsHub.register(conn, username)
	defer wsHub.unregister(conn)

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// usernameFromRequest extracts the username from the request.
// Checks query param "token" first (used by WebSocket, which can't set headers),
// then Authorization header, then auth_token cookie.
// Returns "" if auth is disabled or no valid token is present.
func usernameFromRequest(r *http.Request) string {
	if ValidateToken == nil {
		return ""
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	if token == "" {
		if c, err := r.Cookie("auth_token"); err == nil {
			token = c.Value
		}
	}
	if token == "" {
		return ""
	}
	username, err := ValidateToken(token)
	if err != nil {
		if cfg, cfgErr := loadGlobalConfig(); cfgErr == nil && cfg.AllowLocalUsernameLogin {
			return token
		}
		return ""
	}
	return username
}
