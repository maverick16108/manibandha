package web

import (
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"manibandha/internal/caps"
	"manibandha/internal/models"
)

var threadReactions = []string{"❤️", "👍", "🙏", "🔥", "😂", "🎉"}

func reactionIdx(e string) int {
	for i, x := range threadReactions {
		if x == e {
			return i
		}
	}
	return 99
}

func isReaction(e string) bool { return reactionIdx(e) < len(threadReactions) }

var reAudio = regexp.MustCompile(`@\[audio\]\([^)]*\)`)
var rePhoto = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
var reWS = regexp.MustCompile(`\s+`)

func threadSnippet(body string) string {
	s := reAudio.ReplaceAllString(body, "🎤 Голосовое сообщение")
	s = rePhoto.ReplaceAllString(s, "🖼 Фото")
	s = strings.TrimSpace(reWS.ReplaceAllString(s, " "))
	return truncRunes(s, 100)
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func (s *Server) capSet(userID int) map[string]bool {
	m := map[string]bool{}
	for _, c := range caps.UserCapabilities(s.DB, userID) {
		m[c] = true
	}
	return m
}

// accessibleThreads — ветки, видимые пользователю (OR по правам).
func (s *Server) accessibleThreads(u *models.User) *gorm.DB {
	cs := s.capSet(u.ID)
	own := -1
	if u.DiscipleID != nil {
		own = *u.DiscipleID
	}
	conds := s.DB.Where("threads.disciple_id = ?", own)
	if cs["questions.answer"] || cs["questions.view_all"] {
		conds = conds.Or("threads.kind = ?", "question")
	}
	if cs["reports.read_all"] {
		if cs["disciples.view_all"] {
			conds = conds.Or("threads.kind = ?", "report")
		} else {
			conds = conds.Or("threads.kind = ? AND disciples.mentor_id = ?", "report", own)
		}
	}
	if cs["disciples.approve"] {
		conds = conds.Or("threads.kind = ?", "approval")
	}
	return s.DB.Model(&models.Thread{}).
		Joins("JOIN disciples ON disciples.id = threads.disciple_id").
		Where(conds)
}

func isRecipient(cs map[string]bool, kind string) bool {
	switch kind {
	case "question":
		return cs["questions.answer"] || cs["questions.view_all"]
	case "report":
		return cs["reports.read_all"]
	case "approval":
		return cs["disciples.approve"]
	}
	return false
}

func (s *Server) markRead(userID, threadID int) {
	var tr models.ThreadRead
	if err := s.DB.Where("thread_id = ? AND user_id = ?", threadID, userID).First(&tr).Error; err == nil {
		s.DB.Model(&models.ThreadRead{}).Where("id = ?", tr.ID).Update("last_seen_at", gorm.Expr("now()"))
	} else {
		s.DB.Create(&models.ThreadRead{ThreadID: threadID, UserID: userID})
	}
}

func (s *Server) markStaffSeen(u *models.User, t *models.Thread) {
	if isRecipient(s.capSet(u.ID), t.Kind) {
		// В Python updated_at имеет onupdate=now(), поэтому запись staff_seen_at
		// заодно поднимает updated_at — воспроизводим это поведение точно.
		s.DB.Model(&models.Thread{}).Where("id = ?", t.ID).Updates(map[string]any{
			"staff_seen_at": gorm.Expr("now()"),
			"updated_at":    gorm.Expr("now()"),
		})
	}
}

func withinEditWindow(m *models.ThreadMessage) bool {
	return time.Since(m.CreatedAt) <= time.Hour
}

func discipleName(d *models.Disciple) string {
	if d == nil {
		return "—"
	}
	return d.Name()
}

func reactionsOf(m *models.ThreadMessage, userID int) []map[string]any {
	counts := map[string]int{}
	var order []string
	mine := ""
	for _, l := range m.Likes {
		if _, ok := counts[l.Emoji]; !ok {
			order = append(order, l.Emoji)
		}
		counts[l.Emoji]++
		if l.UserID == userID {
			mine = l.Emoji
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		if counts[order[i]] != counts[order[j]] {
			return counts[order[i]] > counts[order[j]]
		}
		return reactionIdx(order[i]) < reactionIdx(order[j])
	})
	out := make([]map[string]any, 0, len(order))
	for _, e := range order {
		out = append(out, map[string]any{"emoji": e, "count": counts[e], "mine": e == mine})
	}
	return out
}

func replyDict(m *models.ThreadMessage) any {
	if m.ReplyTo == nil {
		return nil
	}
	var an any
	if m.ReplyTo.Author != nil {
		an = m.ReplyTo.Author.FullName
	}
	return map[string]any{"id": m.ReplyTo.ID, "author_name": an, "body": threadSnippet(m.ReplyTo.Body)}
}

func (s *Server) msgOut(m *models.ThreadMessage, userID int) map[string]any {
	var an any
	if m.Author != nil {
		an = m.Author.FullName
	}
	return map[string]any{
		"id": m.ID, "author_id": m.AuthorID, "author_name": an,
		"body": m.Body, "created_at": tsUTC(m.CreatedAt), "edit_count": m.EditCount,
		"reactions": reactionsOf(m, userID), "reply_to": replyDict(m),
	}
}

// GET /threads
func (s *Server) listThreads(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	qp := r.URL.Query()
	q := s.accessibleThreads(u).
		Preload("Disciple").
		Preload("Messages", func(d *gorm.DB) *gorm.DB { return d.Order("created_at") })
	if v := qp.Get("kind"); v != "" {
		q = q.Where("threads.kind = ?", v)
	}
	if v := qp.Get("disciple_id"); v != "" {
		q = q.Where("threads.disciple_id = ?", v)
	}
	if v := qp.Get("mentor_id"); v != "" {
		q = q.Where("disciples.mentor_id = ?", v)
	}
	if v := qp.Get("period"); v != "" {
		q = q.Where("threads.period = ?", v)
	}
	var rows []models.Thread
	q.Order("threads.updated_at DESC").Find(&rows)

	seen := s.readMap(u.ID)
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		t := &rows[i]
		var lastPreview any
		if n := len(t.Messages); n > 0 {
			lastPreview = truncRunes(t.Messages[n-1].Body, 120)
		}
		ls, ok := seen[t.ID]
		unread := !ok || t.UpdatedAt.After(ls)
		out = append(out, map[string]any{
			"id": t.ID, "kind": t.Kind, "disciple_id": t.DiscipleID,
			"disciple_name": discipleName(t.Disciple),
			"subject":       t.Subject, "period": t.Period, "updated_at": tsUTC(t.UpdatedAt),
			"messages_count": len(t.Messages), "last_preview": lastPreview, "unread": unread,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) readMap(userID int) map[int]time.Time {
	var reads []models.ThreadRead
	s.DB.Where("user_id = ?", userID).Find(&reads)
	m := make(map[int]time.Time, len(reads))
	for _, r := range reads {
		m[r.ThreadID] = r.LastSeenAt
	}
	return m
}

// GET /threads/nav-counts
func (s *Server) navCounts(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	cs := s.capSet(u.ID)
	seen := s.readMap(u.ID)

	countUnread := func(kind string) int {
		recipient := isRecipient(cs, kind)
		var threads []models.Thread
		s.accessibleThreads(u).Where("threads.kind = ?", kind).Find(&threads)
		c := 0
		for i := range threads {
			t := &threads[i]
			var unread bool
			if recipient {
				unread = t.StaffSeenAt == nil || t.UpdatedAt.After(*t.StaffSeenAt)
			} else {
				ls, ok := seen[t.ID]
				unread = !ok || t.UpdatedAt.After(ls)
			}
			if unread {
				c++
			}
		}
		return c
	}

	res := map[string]any{
		"questions": countUnread("question"),
		"reports":   countUnread("report"),
		"approvals": 0,
	}
	if cs["disciples.approve"] {
		var n int64
		s.DB.Model(&models.Disciple{}).Where("is_approved = ?", false).Count(&n)
		res["approvals"] = int(n)
	}
	res["forum"] = 0
	if cs["forum.view"] {
		freads := map[int]time.Time{}
		var fr []models.ForumTopicRead
		s.DB.Where("user_id = ?", u.ID).Find(&fr)
		for _, x := range fr {
			freads[x.TopicID] = x.LastSeenAt
		}
		var topics []models.ForumTopic
		s.DB.Find(&topics)
		fc := 0
		for _, t := range topics {
			ls, ok := freads[t.ID]
			if !ok || t.UpdatedAt.After(ls) {
				fc++
			}
		}
		res["forum"] = fc
	}
	res["conference"] = 0
	if cs["conference.view"] {
		var n int64
		s.DB.Model(&models.Conference{}).Where("status IN ?", []string{"live", "scheduled"}).Count(&n)
		res["conference"] = int(n)
	}
	writeJSON(w, http.StatusOK, res)
}

// GET /threads/stats?disciple_id=
func (s *Server) threadStats(w http.ResponseWriter, r *http.Request) {
	did, _ := strconv.Atoi(r.URL.Query().Get("disciple_id"))
	var questions, reports int64
	s.DB.Model(&models.Thread{}).Where("disciple_id = ? AND kind = ?", did, "question").Count(&questions)
	s.DB.Model(&models.Thread{}).Where("disciple_id = ? AND kind = ?", did, "report").Count(&reports)
	var messages int64
	var student models.User
	if err := s.DB.Where("disciple_id = ?", did).First(&student).Error; err == nil {
		s.DB.Model(&models.ThreadMessage{}).
			Joins("JOIN threads ON threads.id = thread_messages.thread_id").
			Where("threads.disciple_id = ? AND thread_messages.author_id = ?", did, student.ID).
			Count(&messages)
	}
	writeJSON(w, http.StatusOK, map[string]any{"questions": questions, "reports": reports, "messages": messages})
}

// GET /threads/{id}
func (s *Server) getThread(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	out, code := s.buildThreadOut(currentUser(r), id)
	if code != http.StatusOK {
		httpErr(w, code, "Ветка не найдена")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) buildThreadOut(u *models.User, threadID int) (map[string]any, int) {
	var t models.Thread
	if err := s.accessibleThreads(u).
		Preload("Disciple").
		Preload("Messages", func(d *gorm.DB) *gorm.DB { return d.Order("created_at") }).
		Preload("Messages.Author").Preload("Messages.Likes").
		Preload("Messages.ReplyTo").Preload("Messages.ReplyTo.Author").
		Where("threads.id = ?", threadID).First(&t).Error; err != nil {
		return nil, http.StatusNotFound
	}
	var lastRead any
	var prev models.ThreadRead
	if err := s.DB.Where("thread_id = ? AND user_id = ?", t.ID, u.ID).First(&prev).Error; err == nil {
		lastRead = tsUTC(prev.LastSeenAt)
	}
	s.markStaffSeen(u, &t)
	s.markRead(u.ID, t.ID)
	// updated_at мог измениться из-за markStaffSeen — перечитываем (как Python после commit)
	var reload models.Thread
	if s.DB.Select("updated_at").First(&reload, t.ID).Error == nil {
		t.UpdatedAt = reload.UpdatedAt
	}

	msgs := make([]map[string]any, 0, len(t.Messages))
	for i := range t.Messages {
		msgs = append(msgs, s.msgOut(&t.Messages[i], u.ID))
	}
	return map[string]any{
		"id": t.ID, "kind": t.Kind, "disciple_id": t.DiscipleID,
		"disciple_name": discipleName(t.Disciple),
		"subject":       t.Subject, "period": t.Period,
		"created_at": tsUTC(t.CreatedAt), "updated_at": tsUTC(t.UpdatedAt),
		"last_read_at": lastRead, "messages": msgs,
	}, http.StatusOK
}

// POST /threads
func (s *Server) createThread(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var p struct {
		Kind       string  `json:"kind"`
		Body       string  `json:"body"`
		DiscipleID *int    `json:"disciple_id"`
		Subject    *string `json:"subject"`
		Period     *string `json:"period"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	var discipleID int
	switch {
	case u.DiscipleID != nil:
		discipleID = *u.DiscipleID
	case u.Role == "guru" && p.DiscipleID != nil:
		discipleID = *p.DiscipleID
	default:
		httpErr(w, http.StatusForbidden, "Нельзя создать ветку без ученика")
		return
	}
	var cnt int64
	s.DB.Model(&models.Disciple{}).Where("id = ?", discipleID).Count(&cnt)
	if cnt == 0 {
		httpErr(w, http.StatusNotFound, "Ученик не найден")
		return
	}
	if p.Kind == "report" && (p.Period == nil || *p.Period == "") {
		httpErr(w, http.StatusBadRequest, "Для отчёта нужен месяц (period)")
		return
	}

	var thread models.Thread
	found := false
	if p.Kind == "report" {
		if err := s.DB.Where("kind = ? AND disciple_id = ? AND period = ?", "report", discipleID, p.Period).First(&thread).Error; err == nil {
			found = true
		}
	}
	if !found {
		thread = models.Thread{Kind: p.Kind, DiscipleID: discipleID, Subject: p.Subject, Period: p.Period}
		s.DB.Create(&thread)
	}
	s.DB.Create(&models.ThreadMessage{ThreadID: thread.ID, AuthorID: &u.ID, Body: p.Body})

	out, code := s.buildThreadOut(u, thread.ID)
	if code != http.StatusOK {
		httpErr(w, code, "Ветка не найдена")
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// POST /threads/{id}/messages
func (s *Server) addThreadMessage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	t, ok := s.accessibleThread(u, id)
	if !ok {
		httpErr(w, http.StatusNotFound, "Ветка не найдена")
		return
	}
	var p struct {
		Body      string `json:"body"`
		ReplyToID *int   `json:"reply_to_id"`
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
	var replyTo *int
	if p.ReplyToID != nil {
		var parent models.ThreadMessage
		if err := s.DB.First(&parent, *p.ReplyToID).Error; err == nil && parent.ThreadID == t.ID {
			replyTo = &parent.ID
		}
	}
	msg := models.ThreadMessage{ThreadID: t.ID, AuthorID: &u.ID, Body: body, ReplyToID: replyTo}
	s.DB.Create(&msg)
	s.DB.Model(&models.Thread{}).Where("id = ?", t.ID).Update("updated_at", gorm.Expr("now()"))
	s.markStaffSeen(u, t)
	s.markRead(u.ID, t.ID)

	var full models.ThreadMessage
	s.DB.Preload("Author").Preload("Likes").Preload("ReplyTo").Preload("ReplyTo.Author").First(&full, msg.ID)
	writeJSON(w, http.StatusCreated, s.msgOut(&full, u.ID))
}

func (s *Server) accessibleThread(u *models.User, id int) (*models.Thread, bool) {
	var t models.Thread
	if err := s.accessibleThreads(u).Where("threads.id = ?", id).First(&t).Error; err != nil {
		return nil, false
	}
	return &t, true
}

func (s *Server) ownEditable(u *models.User, threadID, messageID int) (*models.ThreadMessage, int, string) {
	if _, ok := s.accessibleThread(u, threadID); !ok {
		return nil, http.StatusNotFound, "Ветка не найдена"
	}
	var msg models.ThreadMessage
	if err := s.DB.First(&msg, messageID).Error; err != nil || msg.ThreadID != threadID {
		return nil, http.StatusNotFound, "Сообщение не найдено"
	}
	if msg.AuthorID == nil || *msg.AuthorID != u.ID {
		return nil, http.StatusForbidden, "Можно менять только свои сообщения"
	}
	if !withinEditWindow(&msg) {
		return nil, http.StatusForbidden, "Прошёл час — сообщение больше нельзя изменить"
	}
	return &msg, http.StatusOK, ""
}

// PATCH /threads/{id}/messages/{mid}
func (s *Server) editThreadMessage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	mid, _ := strconv.Atoi(chi.URLParam(r, "mid"))
	u := currentUser(r)
	msg, code, msgErr := s.ownEditable(u, id, mid)
	if code != http.StatusOK {
		httpErr(w, code, msgErr)
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
	s.DB.Model(&models.ThreadMessage{}).Where("id = ?", msg.ID).Updates(map[string]any{
		"body": body, "edited_at": gorm.Expr("now()"), "edit_count": msg.EditCount + 1,
	})
	var full models.ThreadMessage
	s.DB.Preload("Author").Preload("Likes").Preload("ReplyTo").Preload("ReplyTo.Author").First(&full, msg.ID)
	writeJSON(w, http.StatusOK, s.msgOut(&full, u.ID))
}

// DELETE /threads/{id}/messages/{mid}
func (s *Server) deleteThreadMessage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	mid, _ := strconv.Atoi(chi.URLParam(r, "mid"))
	u := currentUser(r)
	msg, code, msgErr := s.ownEditable(u, id, mid)
	if code != http.StatusOK {
		httpErr(w, code, msgErr)
		return
	}
	s.DB.Delete(&models.ThreadMessage{}, msg.ID)
	w.WriteHeader(http.StatusNoContent)
}

// POST /threads/{id}/messages/{mid}/react
func (s *Server) reactThreadMessage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	mid, _ := strconv.Atoi(chi.URLParam(r, "mid"))
	u := currentUser(r)
	var p struct {
		Emoji string `json:"emoji"`
	}
	_ = decodeJSON(r, &p)
	if !isReaction(p.Emoji) {
		httpErr(w, http.StatusBadRequest, "Недопустимая реакция")
		return
	}
	if _, ok := s.accessibleThread(u, id); !ok {
		httpErr(w, http.StatusNotFound, "Ветка не найдена")
		return
	}
	var msg models.ThreadMessage
	if err := s.DB.First(&msg, mid).Error; err != nil || msg.ThreadID != id {
		httpErr(w, http.StatusNotFound, "Сообщение не найдено")
		return
	}
	var existing models.MessageLike
	if err := s.DB.Where("message_id = ? AND user_id = ?", mid, u.ID).First(&existing).Error; err == nil {
		if existing.Emoji == p.Emoji {
			s.DB.Delete(&models.MessageLike{}, existing.ID)
		} else {
			s.DB.Model(&models.MessageLike{}).Where("id = ?", existing.ID).Update("emoji", p.Emoji)
		}
	} else {
		s.DB.Create(&models.MessageLike{MessageID: mid, UserID: u.ID, Emoji: p.Emoji})
	}
	var full models.ThreadMessage
	s.DB.Preload("Likes").First(&full, mid)
	writeJSON(w, http.StatusOK, map[string]any{"reactions": reactionsOf(&full, u.ID)})
}
