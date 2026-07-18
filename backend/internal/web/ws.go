package web

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"manibandha/internal/models"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // токен в query аутентифицирует
}

// wsConn — обёртка с мьютексом (конкурентная запись в gorilla-сокет небезопасна).
type wsConn struct {
	ws *websocket.Conn
	mu sync.Mutex
}

func (c *wsConn) writeJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.WriteJSON(v)
}

// ── Хаб чата: один пользователь → набор сокетов ──────────────────────────────

type chatHubT struct {
	mu      sync.RWMutex
	sockets map[int]map[*wsConn]bool
}

var chatH = &chatHubT{sockets: map[int]map[*wsConn]bool{}}

func (h *chatHubT) add(uid int, c *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sockets[uid] == nil {
		h.sockets[uid] = map[*wsConn]bool{}
	}
	h.sockets[uid][c] = true
}

func (h *chatHubT) remove(uid int, c *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.sockets[uid]; m != nil {
		delete(m, c)
		if len(m) == 0 {
			delete(h.sockets, uid)
		}
	}
}

func (h *chatHubT) sendToUsers(uids []int, data any) {
	h.mu.RLock()
	seen := map[*wsConn]bool{}
	var targets []*wsConn
	for _, uid := range uids {
		for c := range h.sockets[uid] {
			if !seen[c] {
				seen[c] = true
				targets = append(targets, c)
			}
		}
	}
	h.mu.RUnlock()
	for _, c := range targets {
		_ = c.writeJSON(data)
	}
}

func (s *Server) broadcastChat(chatID int, data any) {
	ids := s.memberIDs(chatID)
	go chatH.sendToUsers(ids, data)
}

// онлайн, если у пользователя есть хотя бы один активный чат-сокет
func (h *chatHubT) isOnline(uid int) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sockets[uid]) > 0
}

// ── Хаб веток: тема → набор сокетов ──────────────────────────────────────────

type threadHubT struct {
	mu    sync.RWMutex
	rooms map[int]map[*wsConn]bool
}

var threadH = &threadHubT{rooms: map[int]map[*wsConn]bool{}}

func (h *threadHubT) add(tid int, c *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[tid] == nil {
		h.rooms[tid] = map[*wsConn]bool{}
	}
	h.rooms[tid][c] = true
}

func (h *threadHubT) remove(tid int, c *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.rooms[tid]; m != nil {
		delete(m, c)
		if len(m) == 0 {
			delete(h.rooms, tid)
		}
	}
}

func (h *threadHubT) broadcast(tid int, data any) {
	h.mu.RLock()
	var targets []*wsConn
	for c := range h.rooms[tid] {
		targets = append(targets, c)
	}
	h.mu.RUnlock()
	for _, c := range targets {
		_ = c.writeJSON(data)
	}
}

func (s *Server) broadcastThread(threadID int, data any) {
	go threadH.broadcast(threadID, data)
}

// ── аутентификация WS по токену ──────────────────────────────────────────────

func (s *Server) wsAuth(token string) *models.User {
	claims, err := s.JWT.Parse(token)
	if err != nil {
		return nil
	}
	email, _ := claims["sub"].(string)
	if email == "" {
		return nil
	}
	var u models.User
	if err := s.DB.Where("email = ?", email).First(&u).Error; err != nil || !u.IsActive {
		return nil
	}
	return &u
}

// GET /ws/chat?token=...
func (s *Server) wsChat(w http.ResponseWriter, r *http.Request) {
	u := s.wsAuth(r.URL.Query().Get("token"))
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &wsConn{ws: ws}
	chatH.add(u.ID, c)
	defer func() {
		chatH.remove(u.ID, c)
		ws.Close()
	}()
	for {
		var data map[string]any
		if err := ws.ReadJSON(&data); err != nil {
			return
		}
		if data["type"] == "typing" {
			cid, ok := toInt(data["chat_id"])
			if !ok {
				continue
			}
			var cnt int64
			s.DB.Model(&models.ChatMember{}).Where("chat_id = ? AND user_id = ?", cid, u.ID).Count(&cnt)
			if cnt == 0 {
				continue
			}
			others := []int{}
			for _, id := range s.memberIDs(cid) {
				if id != u.ID {
					others = append(others, id)
				}
			}
			chatH.sendToUsers(others, map[string]any{
				"type": "typing", "chat_id": cid, "user_id": u.ID, "name": u.FullName,
			})
		}
	}
}

// GET /ws/threads/{id}?token=...
func (s *Server) wsThread(w http.ResponseWriter, r *http.Request) {
	u := s.wsAuth(r.URL.Query().Get("token"))
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	tid, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if _, ok := s.accessibleThread(u, tid); !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &wsConn{ws: ws}
	threadH.add(tid, c)
	defer func() {
		threadH.remove(tid, c)
		ws.Close()
	}()
	for {
		var data map[string]any
		if err := ws.ReadJSON(&data); err != nil {
			return
		}
		switch data["type"] {
		case "typing":
			threadH.broadcast(tid, map[string]any{"type": "typing", "user_id": u.ID, "name": u.FullName})
		case "message":
			body, _ := data["body"].(string)
			body = strings.TrimSpace(body)
			if body == "" {
				continue
			}
			if _, ok := s.accessibleThread(u, tid); !ok {
				continue
			}
			var replyTo *int
			if rid, ok := toInt(data["reply_to_id"]); ok && rid > 0 {
				var parent models.ThreadMessage
				if err := s.DB.First(&parent, rid).Error; err == nil && parent.ThreadID == tid {
					replyTo = &parent.ID
				}
			}
			msg := models.ThreadMessage{ThreadID: tid, AuthorID: &u.ID, Body: body, ReplyToID: replyTo}
			s.DB.Create(&msg)
			t := &models.Thread{ID: tid, Kind: threadKind(s, tid)}
			s.DB.Model(&models.Thread{}).Where("id = ?", tid).Update("updated_at", gorm.Expr("now()"))
			s.markStaffSeen(u, t)
			s.markRead(u.ID, tid)
			var full models.ThreadMessage
			s.DB.Preload("ReplyTo").Preload("ReplyTo.Author").First(&full, msg.ID)
			threadH.broadcast(tid, map[string]any{
				"type": "message",
				"message": map[string]any{
					"id": msg.ID, "author_id": u.ID, "author_name": u.FullName,
					"body": msg.Body, "created_at": tsUTC(msg.CreatedAt),
					"edit_count": 0, "reactions": []any{}, "reply_to": replyDict(&full),
				},
			})
		}
	}
}

func threadKind(s *Server, tid int) string {
	var t models.Thread
	s.DB.Select("kind").First(&t, tid)
	return t.Kind
}
