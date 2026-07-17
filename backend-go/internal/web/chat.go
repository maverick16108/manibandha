package web

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"manibandha/internal/models"
)

// доступные реакции чата (те же 6, что и threadReactions)
var chatReactionSet = map[string]bool{"❤️": true, "👍": true, "🙏": true, "🔥": true, "😂": true, "🎉": true}

// ── доступ ────────────────────────────────────────────────────────────────

func (s *Server) isPending(u *models.User) bool {
	if u.DiscipleID == nil {
		return false
	}
	var d models.Disciple
	if err := s.DB.Select("is_approved").First(&d, *u.DiscipleID).Error; err != nil {
		return false
	}
	return !d.IsApproved
}

// chatGate — незаапрувленный кандидат в мессенджер не допускается.
func (s *Server) chatGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.isPending(currentUser(r)) {
			httpErr(w, http.StatusForbidden, "Чат станет доступен после одобрения заявки")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) myChatIDs(userID int) []int {
	var ids []int
	s.DB.Model(&models.ChatMember{}).Where("user_id = ?", userID).Pluck("chat_id", &ids)
	return ids
}

func (s *Server) memberIDs(chatID int) []int {
	var ids []int
	s.DB.Model(&models.ChatMember{}).Where("chat_id = ?", chatID).Pluck("user_id", &ids)
	return ids
}

func (s *Server) requireMembership(userID, chatID int) (*models.Chat, int) {
	var chat models.Chat
	if err := s.DB.First(&chat, chatID).Error; err != nil {
		return nil, http.StatusNotFound
	}
	var cnt int64
	s.DB.Model(&models.ChatMember{}).Where("chat_id = ? AND user_id = ?", chatID, userID).Count(&cnt)
	if cnt == 0 {
		return nil, http.StatusForbidden
	}
	return &chat, http.StatusOK
}

func (s *Server) nextChatSeq() int64 {
	var seq int64
	s.DB.Raw("SELECT nextval('chat_message_seq')").Scan(&seq)
	return seq
}

// ── сериализация ──────────────────────────────────────────────────────────

func chatSnippet(body string) string {
	x := reAudio.ReplaceAllString(body, "🎤 Голосовое сообщение")
	x = rePhoto.ReplaceAllString(x, "🖼 Фото")
	x = strings.TrimSpace(reWS.ReplaceAllString(x, " "))
	return truncRunes(x, 120)
}

func chatReactionsAgg(m *models.ChatMessage) []map[string]any {
	groups := map[string][]models.ChatMessageReaction{}
	var order []string
	for _, r := range m.Reactions {
		if _, ok := groups[r.Emoji]; !ok {
			order = append(order, r.Emoji)
		}
		groups[r.Emoji] = append(groups[r.Emoji], r)
	}
	sort.SliceStable(order, func(i, j int) bool {
		if len(groups[order[i]]) != len(groups[order[j]]) {
			return len(groups[order[i]]) > len(groups[order[j]])
		}
		return reactionIdx(order[i]) < reactionIdx(order[j])
	})
	out := make([]map[string]any, 0, len(order))
	for _, e := range order {
		who := make([]map[string]any, 0, len(groups[e]))
		for _, x := range groups[e] {
			who = append(who, participant(x.User))
		}
		out = append(out, map[string]any{"emoji": e, "count": len(groups[e]), "who": who})
	}
	return out
}

func myReaction(m *models.ChatMessage, userID int) any {
	for _, r := range m.Reactions {
		if r.UserID == userID {
			return r.Emoji
		}
	}
	return nil
}

func chatMsgOut(m *models.ChatMessage, userID int) map[string]any {
	var replyPreview any
	if m.ReplyQuote != nil {
		replyPreview = truncRunes(*m.ReplyQuote, 200)
	} else if m.ReplyToID != nil && m.ReplyTo != nil {
		replyPreview = chatSnippet(m.ReplyTo.Body)
	}
	body := m.Body
	if m.Deleted {
		body = ""
	}
	var an any
	if m.Author != nil {
		an = m.Author.FullName
	}
	var editedAt any
	if m.EditedAt != nil {
		editedAt = tsUTC(*m.EditedAt)
	}
	return map[string]any{
		"id": m.ID, "chat_id": m.ChatID, "seq": m.Seq, "client_uuid": m.ClientUUID,
		"author_id": m.AuthorID, "author_name": an, "body": body,
		"reply_to_id": m.ReplyToID, "reply_preview": replyPreview,
		"created_at": tsUTC(m.CreatedAt), "edited_at": editedAt, "edit_count": m.EditCount,
		"deleted": m.Deleted, "reactions": chatReactionsAgg(m), "my_reaction": myReaction(m, userID),
	}
}

func membersOut(chat *models.Chat) []map[string]any {
	out := make([]map[string]any, 0, len(chat.Members))
	for i := range chat.Members {
		mem := &chat.Members[i]
		var fn, av any
		if mem.User != nil {
			fn = mem.User.FullName
			av = mem.User.AvatarURL
		}
		out = append(out, map[string]any{
			"user_id": mem.UserID, "full_name": fn, "avatar_url": av,
			"role": mem.Role, "last_read_seq": mem.LastReadSeq,
		})
	}
	return out
}

func (s *Server) chatOut(chat *models.Chat, userID int) map[string]any {
	var last models.ChatMessage
	var lastOut any
	if err := s.DB.Preload("Author").Preload("ReplyTo").Preload("Reactions").Preload("Reactions.User").
		Where("chat_id = ?", chat.ID).Order("seq DESC").First(&last).Error; err == nil {
		lastOut = chatMsgOut(&last, userID)
	}
	var me *models.ChatMember
	for i := range chat.Members {
		if chat.Members[i].UserID == userID {
			me = &chat.Members[i]
			break
		}
	}
	var lastRead int64
	pinned := false
	if me != nil {
		lastRead = me.LastReadSeq
		pinned = me.Pinned
	}
	var unread int64
	s.DB.Model(&models.ChatMessage{}).
		Where("chat_id = ? AND seq > ? AND author_id <> ? AND deleted = ?", chat.ID, lastRead, userID, false).
		Count(&unread)
	return map[string]any{
		"id": chat.ID, "type": chat.Type, "title": chat.Title, "photo_url": chat.PhotoURL,
		"created_by": chat.CreatedBy, "created_at": tsUTC(chat.CreatedAt), "updated_at": tsUTC(chat.UpdatedAt),
		"members": membersOut(chat), "last_message": lastOut, "unread": unread, "pinned": pinned,
	}
}

func membersOrdered(db *gorm.DB) *gorm.DB { return db.Order("id") }

func (s *Server) loadChat(id int) *models.Chat {
	var c models.Chat
	s.DB.Preload("Members", membersOrdered).Preload("Members.User").First(&c, id)
	return &c
}

// ── чаты ──────────────────────────────────────────────────────────────────

func (s *Server) listChats(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var chats []models.Chat
	s.DB.Preload("Members", membersOrdered).Preload("Members.User").
		Where("id IN ?", s.myChatIDs(u.ID)).Order("updated_at DESC").Find(&chats)
	out := make([]map[string]any, 0, len(chats))
	for i := range chats {
		out = append(out, s.chatOut(&chats[i], u.ID))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) listContacts(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var users []models.User
	s.DB.Where("is_active = ? AND id <> ? AND (disciple_id IS NULL OR disciple_id NOT IN (?))",
		true, u.ID, s.DB.Model(&models.Disciple{}).Select("id").Where("is_approved = ?", false)).
		Order("full_name").Find(&users)
	out := make([]map[string]any, 0, len(users))
	for _, x := range users {
		out = append(out, map[string]any{"id": x.ID, "full_name": x.FullName, "avatar_url": x.AvatarURL, "role": x.Role})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getUpdates(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	limit := 300
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if limit > 500 {
		limit = 500
	}
	if limit < 1 {
		limit = 1
	}
	var rows []models.ChatMessage
	s.DB.Preload("Author").Preload("ReplyTo").Preload("Reactions").Preload("Reactions.User").
		Where("chat_id IN ? AND seq > ?", s.myChatIDs(u.ID), since).
		Order("seq ASC").Limit(limit + 1).Find(&rows)
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	updates := make([]map[string]any, 0, len(rows))
	pts := since
	for i := range rows {
		updates = append(updates, map[string]any{
			"type": "message", "seq": rows[i].Seq, "chat_id": rows[i].ChatID,
			"message": chatMsgOut(&rows[i], u.ID), "message_id": nil,
		})
		pts = rows[i].Seq
	}
	writeJSON(w, http.StatusOK, map[string]any{"updates": updates, "pts": pts, "has_more": hasMore})
}

func (s *Server) createChat(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var p struct {
		Type      string `json:"type"`
		PeerID    *int   `json:"peer_id"`
		Title     string `json:"title"`
		MemberIDs []int  `json:"member_ids"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	if p.Type == "direct" {
		if p.PeerID == nil || *p.PeerID == u.ID {
			httpErr(w, http.StatusBadRequest, "Укажите собеседника")
			return
		}
		var peer models.User
		if err := s.DB.First(&peer, *p.PeerID).Error; err != nil || !peer.IsActive || s.isPending(&peer) {
			httpErr(w, http.StatusNotFound, "Собеседник недоступен")
			return
		}
		// дедуп существующего личного чата
		var existing models.Chat
		err := s.DB.Where("type = ? AND id IN (?) AND id IN (?)", "direct",
			s.DB.Model(&models.ChatMember{}).Select("chat_id").Where("user_id = ?", u.ID),
			s.DB.Model(&models.ChatMember{}).Select("chat_id").Where("user_id = ?", peer.ID)).First(&existing).Error
		if err == nil {
			writeJSON(w, http.StatusCreated, s.chatOut(s.loadChat(existing.ID), u.ID))
			return
		}
		chat := models.Chat{Type: "direct", CreatedBy: &u.ID}
		s.DB.Create(&chat)
		s.DB.Create(&models.ChatMember{ChatID: chat.ID, UserID: u.ID, Role: "member"})
		s.DB.Create(&models.ChatMember{ChatID: chat.ID, UserID: peer.ID, Role: "member"})
		s.broadcastChat(chat.ID, map[string]any{"type": "chat", "chat_id": chat.ID})
		writeJSON(w, http.StatusCreated, s.chatOut(s.loadChat(chat.ID), u.ID))
		return
	}

	// группа
	title := strings.TrimSpace(p.Title)
	if title == "" {
		httpErr(w, http.StatusBadRequest, "Укажите название группы")
		return
	}
	ids := map[int]bool{}
	for _, id := range p.MemberIDs {
		if id != u.ID {
			ids[id] = true
		}
	}
	idList := make([]int, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	var valid []models.User
	if len(idList) > 0 {
		s.DB.Where("id IN ? AND is_active = ?", idList, true).Find(&valid)
	}
	chat := models.Chat{Type: "group", Title: &title, CreatedBy: &u.ID}
	s.DB.Create(&chat)
	s.DB.Create(&models.ChatMember{ChatID: chat.ID, UserID: u.ID, Role: "owner"})
	for i := range valid {
		if !s.isPending(&valid[i]) {
			s.DB.Create(&models.ChatMember{ChatID: chat.ID, UserID: valid[i].ID, Role: "member"})
		}
	}
	s.broadcastChat(chat.ID, map[string]any{"type": "chat", "chat_id": chat.ID})
	writeJSON(w, http.StatusCreated, s.chatOut(s.loadChat(chat.ID), u.ID))
}

func (s *Server) getChat(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	if _, code := s.requireMembership(u.ID, id); code != http.StatusOK {
		httpErr(w, code, membershipMsg(code))
		return
	}
	writeJSON(w, http.StatusOK, s.chatOut(s.loadChat(id), u.ID))
}

func membershipMsg(code int) string {
	if code == http.StatusNotFound {
		return "Чат не найден"
	}
	return "Нет доступа к чату"
}

func (s *Server) updateChatHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	chat, code := s.requireMembership(u.ID, id)
	if code != http.StatusOK {
		httpErr(w, code, membershipMsg(code))
		return
	}
	if chat.Type != "group" {
		httpErr(w, http.StatusBadRequest, "Настройки доступны только для групп")
		return
	}
	var p struct {
		Title    *string `json:"title"`
		PhotoURL *string `json:"photo_url"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	upd := map[string]any{}
	if p.Title != nil {
		t := strings.TrimSpace(*p.Title)
		if t == "" {
			httpErr(w, http.StatusBadRequest, "Название не может быть пустым")
			return
		}
		upd["title"] = t
	}
	if p.PhotoURL != nil {
		if *p.PhotoURL == "" {
			upd["photo_url"] = nil
		} else {
			upd["photo_url"] = *p.PhotoURL
		}
	}
	if len(upd) > 0 {
		s.DB.Model(&models.Chat{}).Where("id = ?", id).Updates(upd)
	}
	s.broadcastChat(id, map[string]any{"type": "chat", "chat_id": id})
	writeJSON(w, http.StatusOK, s.chatOut(s.loadChat(id), u.ID))
}

func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	if _, code := s.requireMembership(u.ID, id); code != http.StatusOK {
		httpErr(w, code, membershipMsg(code))
		return
	}
	limit := 50
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if limit > 100 {
		limit = 100
	}
	q := s.DB.Preload("Author").Preload("ReplyTo").Preload("Reactions").Preload("Reactions.User").
		Where("chat_id = ?", id)
	if v := r.URL.Query().Get("before_seq"); v != "" {
		q = q.Where("seq < ?", v)
	}
	var rows []models.ChatMessage
	q.Order("seq DESC").Limit(limit).Find(&rows)
	// реверс для отображения по возрастанию
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		out = append(out, chatMsgOut(&rows[i], u.ID))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) sendMessage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	if _, code := s.requireMembership(u.ID, id); code != http.StatusOK {
		httpErr(w, code, membershipMsg(code))
		return
	}
	var p struct {
		ClientUUID string  `json:"client_uuid"`
		Body       string  `json:"body"`
		ReplyToID  *int    `json:"reply_to_id"`
		ReplyQuote *string `json:"reply_quote"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	body := strings.TrimSpace(p.Body)
	if body == "" {
		httpErr(w, http.StatusBadRequest, "Пустое сообщение")
		return
	}
	if p.ClientUUID == "" {
		httpErr(w, http.StatusBadRequest, "Нужен client_uuid")
		return
	}
	if existing := s.findByUUID(id, p.ClientUUID); existing != nil {
		writeJSON(w, http.StatusCreated, chatMsgOut(existing, u.ID))
		return
	}
	var replyTo *int
	if p.ReplyToID != nil {
		var parent models.ChatMessage
		if err := s.DB.First(&parent, *p.ReplyToID).Error; err == nil && parent.ChatID == id {
			replyTo = &parent.ID
		}
	}
	quote := p.ReplyQuote
	if quote != nil && *quote == "" {
		quote = nil
	}
	msg := models.ChatMessage{
		ChatID: id, Seq: s.nextChatSeq(), ClientUUID: &p.ClientUUID,
		AuthorID: &u.ID, Body: body, ReplyToID: replyTo, ReplyQuote: quote,
	}
	if err := s.DB.Create(&msg).Error; err != nil {
		if existing := s.findByUUID(id, p.ClientUUID); existing != nil {
			writeJSON(w, http.StatusCreated, chatMsgOut(existing, u.ID))
			return
		}
		httpErr(w, http.StatusBadRequest, "Не удалось отправить")
		return
	}
	s.DB.Model(&models.Chat{}).Where("id = ?", id).Update("updated_at", gorm.Expr("now()"))
	s.DB.Model(&models.ChatMember{}).Where("chat_id = ? AND user_id = ? AND last_read_seq < ?", id, u.ID, msg.Seq).
		Update("last_read_seq", msg.Seq)
	full := s.loadMessage(msg.ID)
	out := chatMsgOut(full, u.ID)
	s.broadcastChat(id, map[string]any{"type": "message", "seq": msg.Seq, "chat_id": id, "message": out})
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) findByUUID(chatID int, uuid string) *models.ChatMessage {
	var m models.ChatMessage
	if err := s.DB.Preload("Author").Preload("ReplyTo").Preload("Reactions").Preload("Reactions.User").
		Where("chat_id = ? AND client_uuid = ?", chatID, uuid).First(&m).Error; err != nil {
		return nil
	}
	return &m
}

func (s *Server) loadMessage(id int) *models.ChatMessage {
	var m models.ChatMessage
	s.DB.Preload("Author").Preload("ReplyTo").Preload("Reactions").Preload("Reactions.User").First(&m, id)
	return &m
}

func (s *Server) editChatMessage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	mid, _ := strconv.Atoi(chi.URLParam(r, "mid"))
	u := currentUser(r)
	if _, code := s.requireMembership(u.ID, id); code != http.StatusOK {
		httpErr(w, code, membershipMsg(code))
		return
	}
	var msg models.ChatMessage
	if err := s.DB.First(&msg, mid).Error; err != nil || msg.ChatID != id || msg.Deleted {
		httpErr(w, http.StatusNotFound, "Сообщение не найдено")
		return
	}
	if msg.AuthorID == nil || *msg.AuthorID != u.ID {
		httpErr(w, http.StatusForbidden, "Можно менять только свои сообщения")
		return
	}
	if time.Since(msg.CreatedAt) > 24*time.Hour {
		httpErr(w, http.StatusForbidden, "Срок редактирования истёк")
		return
	}
	var p struct {
		Body string `json:"body"`
	}
	_ = decodeJSON(r, &p)
	body := strings.TrimSpace(p.Body)
	if body == "" {
		httpErr(w, http.StatusBadRequest, "Пустое сообщение")
		return
	}
	s.DB.Model(&models.ChatMessage{}).Where("id = ?", mid).Updates(map[string]any{
		"body": body, "edited_at": gorm.Expr("now()"), "edit_count": msg.EditCount + 1,
	})
	full := s.loadMessage(mid)
	out := chatMsgOut(full, u.ID)
	s.broadcastChat(id, map[string]any{"type": "edit", "chat_id": id, "message": out})
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) deleteChatMessage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	mid, _ := strconv.Atoi(chi.URLParam(r, "mid"))
	u := currentUser(r)
	if _, code := s.requireMembership(u.ID, id); code != http.StatusOK {
		httpErr(w, code, membershipMsg(code))
		return
	}
	var msg models.ChatMessage
	if err := s.DB.First(&msg, mid).Error; err != nil || msg.ChatID != id {
		httpErr(w, http.StatusNotFound, "Сообщение не найдено")
		return
	}
	if msg.AuthorID == nil || *msg.AuthorID != u.ID {
		httpErr(w, http.StatusForbidden, "Можно удалять только свои сообщения")
		return
	}
	s.DB.Model(&models.ChatMessage{}).Where("id = ?", mid).Updates(map[string]any{"deleted": true, "body": ""})
	s.broadcastChat(id, map[string]any{"type": "delete", "chat_id": id, "message_id": mid})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) markChatRead(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	if _, code := s.requireMembership(u.ID, id); code != http.StatusOK {
		httpErr(w, code, membershipMsg(code))
		return
	}
	var p struct {
		Seq int64 `json:"seq"`
	}
	_ = decodeJSON(r, &p)
	var me models.ChatMember
	if err := s.DB.Where("chat_id = ? AND user_id = ?", id, u.ID).First(&me).Error; err == nil && p.Seq > me.LastReadSeq {
		s.DB.Model(&models.ChatMember{}).Where("id = ?", me.ID).Update("last_read_seq", p.Seq)
		s.broadcastChat(id, map[string]any{"type": "read", "chat_id": id, "user_id": u.ID, "last_read_seq": p.Seq})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) pinChat(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var me models.ChatMember
	if err := s.DB.Where("chat_id = ? AND user_id = ?", id, u.ID).First(&me).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Чат не найден")
		return
	}
	var p struct {
		Pinned bool `json:"pinned"`
	}
	_ = decodeJSON(r, &p)
	s.DB.Model(&models.ChatMember{}).Where("id = ?", me.ID).Update("pinned", p.Pinned)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) leaveChat(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var me models.ChatMember
	if err := s.DB.Where("chat_id = ? AND user_id = ?", id, u.ID).First(&me).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Чат не найден")
		return
	}
	s.DB.Delete(&models.ChatMember{}, me.ID)
	s.broadcastChat(id, map[string]any{"type": "chat", "chat_id": id})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reactChatMessage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	mid, _ := strconv.Atoi(chi.URLParam(r, "mid"))
	u := currentUser(r)
	var p struct {
		Emoji string `json:"emoji"`
	}
	_ = decodeJSON(r, &p)
	if !chatReactionSet[p.Emoji] {
		httpErr(w, http.StatusBadRequest, "Недопустимая реакция")
		return
	}
	if _, code := s.requireMembership(u.ID, id); code != http.StatusOK {
		httpErr(w, code, membershipMsg(code))
		return
	}
	var msg models.ChatMessage
	if err := s.DB.First(&msg, mid).Error; err != nil || msg.ChatID != id {
		httpErr(w, http.StatusNotFound, "Сообщение не найдено")
		return
	}
	var my any
	var existing models.ChatMessageReaction
	if err := s.DB.Where("message_id = ? AND user_id = ?", mid, u.ID).First(&existing).Error; err == nil {
		if existing.Emoji == p.Emoji {
			s.DB.Delete(&models.ChatMessageReaction{}, existing.ID)
			my = nil
		} else {
			s.DB.Model(&models.ChatMessageReaction{}).Where("id = ?", existing.ID).Update("emoji", p.Emoji)
			my = p.Emoji
		}
	} else {
		s.DB.Create(&models.ChatMessageReaction{MessageID: mid, UserID: u.ID, Emoji: p.Emoji})
		my = p.Emoji
	}
	full := s.loadMessage(mid)
	agg := chatReactionsAgg(full)
	s.broadcastChat(id, map[string]any{
		"type": "react", "chat_id": id, "message_id": mid,
		"user_id": u.ID, "reactions": agg, "emoji": my,
	})
	writeJSON(w, http.StatusOK, map[string]any{"reactions": agg, "my_reaction": my})
}
