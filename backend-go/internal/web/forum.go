package web

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"manibandha/internal/caps"
	"manibandha/internal/models"
)

func (s *Server) isMod(userID int) bool { return caps.HasCap(s.DB, userID, "forum.moderate") }

func (s *Server) withinForumEdit(p *models.ForumPost) bool {
	mins := s.getIntSetting("forum_edit_window_minutes", 60)
	return time.Since(p.CreatedAt) <= time.Duration(mins)*time.Minute
}

func participant(u *models.User) map[string]any {
	if u == nil {
		return map[string]any{"name": nil, "avatar": nil}
	}
	return map[string]any{"name": u.FullName, "avatar": u.AvatarURL}
}

func forumReactions(likes []models.ForumPostLike, userID int) []map[string]any {
	groups := map[string][]models.ForumPostLike{}
	var order []string
	for _, l := range likes {
		e := l.Emoji
		if e == "" {
			e = "❤️"
		}
		if _, ok := groups[e]; !ok {
			order = append(order, e)
		}
		groups[e] = append(groups[e], l)
	}
	sort.SliceStable(order, func(i, j int) bool { return len(groups[order[i]]) > len(groups[order[j]]) })
	out := make([]map[string]any, 0, len(order))
	for _, e := range order {
		ls := groups[e]
		mine := false
		who := make([]map[string]any, 0, len(ls))
		for _, l := range ls {
			if l.UserID == userID {
				mine = true
			}
			who = append(who, participant(l.User))
		}
		out = append(out, map[string]any{"emoji": e, "count": len(ls), "mine": mine, "who": who})
	}
	return out
}

func postOut(p *models.ForumPost, userID int) map[string]any {
	likers := make([]map[string]any, 0, len(p.Likes))
	liked := false
	for _, l := range p.Likes {
		likers = append(likers, participant(l.User))
		if l.UserID == userID {
			liked = true
		}
	}
	var an, aav any
	if p.Author != nil {
		an = p.Author.FullName
		aav = p.Author.AvatarURL
	}
	return map[string]any{
		"id": p.ID, "author_id": p.AuthorID, "author_name": an, "author_avatar": aav,
		"body": p.Body, "created_at": tsUTC(p.CreatedAt), "edit_count": p.EditCount,
		"likes": len(p.Likes), "liked": liked, "likers": likers,
		"reactions": forumReactions(p.Likes, userID),
	}
}

func sectionOut(sec *models.ForumSection, userID int, isMod bool, count int) map[string]any {
	var an any
	if sec.Author != nil {
		an = sec.Author.FullName
	}
	canEdit := (sec.AuthorID != nil && *sec.AuthorID == userID) || isMod
	return map[string]any{
		"id": sec.ID, "title": sec.Title, "description": sec.Description, "color": sec.Color,
		"cover_url": sec.CoverURL, "author_id": sec.AuthorID, "author_name": an,
		"topics_count": count, "can_edit": canEdit, "created_at": tsUTC(sec.CreatedAt),
	}
}

func forumParticipants(posts []models.ForumPost, limit int) []map[string]any {
	seen := map[int]bool{}
	out := []map[string]any{}
	for i := len(posts) - 1; i >= 0; i-- {
		a := posts[i].Author
		if a == nil || seen[a.ID] {
			continue
		}
		seen[a.ID] = true
		out = append(out, map[string]any{"name": a.FullName, "avatar": a.AvatarURL})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func topicSection(t *models.ForumTopic) (any, any) {
	if t.Section != nil {
		return t.Section.Title, t.Section.Color
	}
	return nil, "#c8742a"
}

func topicListItem(t *models.ForumTopic, reads map[int]time.Time) map[string]any {
	pc := len(t.Posts)
	ls, ok := reads[t.ID]
	unread := !ok || t.UpdatedAt.After(ls)
	st, sc := topicSection(t)
	var an any
	if t.Author != nil {
		an = t.Author.FullName
	}
	replies := pc - 1
	if replies < 0 {
		replies = 0
	}
	return map[string]any{
		"id": t.ID, "title": t.Title, "cover_url": t.CoverURL, "section_id": t.SectionID,
		"section_title": st, "section_color": sc, "author_name": an,
		"pinned": t.Pinned, "replies": replies, "views": t.Views, "posts_count": pc,
		"participants": forumParticipants(t.Posts, 5), "unread": unread,
		"last_activity": tsUTC(t.UpdatedAt), "created_at": tsUTC(t.CreatedAt),
	}
}

func topicOut(t *models.ForumTopic, userID int) map[string]any {
	st, sc := topicSection(t)
	var an any
	if t.Author != nil {
		an = t.Author.FullName
	}
	posts := make([]map[string]any, 0, len(t.Posts))
	for i := range t.Posts {
		posts = append(posts, postOut(&t.Posts[i], userID))
	}
	return map[string]any{
		"id": t.ID, "title": t.Title, "cover_url": t.CoverURL, "section_id": t.SectionID,
		"section_title": st, "section_color": sc, "author_name": an,
		"pinned": t.Pinned, "created_at": tsUTC(t.CreatedAt), "posts": posts,
	}
}

// GET /forum/users/{id}
func (s *Server) forumUserCard(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var u models.User
	if err := s.DB.First(&u, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Пользователь не найден")
		return
	}
	var photo, country, region, city any
	photo = u.AvatarURL
	if u.DiscipleID != nil {
		var d models.Disciple
		if err := s.DB.First(&d, *u.DiscipleID).Error; err == nil {
			if u.AvatarURL == nil {
				photo = d.PhotoURL
			}
			country, region, city = d.Country, d.Region, d.City
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name": u.FullName, "photo": photo, "country": country, "region": region, "city": city,
	})
}

// GET /forum/sections
func (s *Server) listSections(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	isMod := s.isMod(u.ID)
	var sections []models.ForumSection
	s.DB.Preload("Author").Preload("Topics").Order("title ASC").Find(&sections)
	out := make([]map[string]any, 0, len(sections))
	for i := range sections {
		out = append(out, sectionOut(&sections[i], u.ID, isMod, len(sections[i].Topics)))
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /forum/sections
func (s *Server) createSection(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var p struct {
		Title       string  `json:"title"`
		Description *string `json:"description"`
		Color       *string `json:"color"`
		CoverURL    *string `json:"cover_url"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	title := strings.TrimSpace(p.Title)
	if title == "" {
		httpErr(w, http.StatusBadRequest, "Нужно название раздела")
		return
	}
	color := "#c8742a"
	if p.Color != nil {
		if c := truncRunes(strings.TrimSpace(*p.Color), 16); c != "" {
			color = c
		}
	}
	sec := models.ForumSection{
		Title: truncRunes(title, 160), Color: color, AuthorID: &u.ID,
	}
	if p.Description != nil {
		if d := truncRunes(strings.TrimSpace(*p.Description), 500); d != "" {
			sec.Description = &d
		}
	}
	if p.CoverURL != nil && *p.CoverURL != "" {
		sec.CoverURL = p.CoverURL
	}
	s.DB.Create(&sec)
	s.DB.Preload("Author").First(&sec, sec.ID)
	writeJSON(w, http.StatusCreated, sectionOut(&sec, u.ID, true, 0))
}

func (s *Server) sectionEditable(u *models.User, sec *models.ForumSection) bool {
	return (sec.AuthorID != nil && *sec.AuthorID == u.ID) || s.isMod(u.ID)
}

// PATCH /forum/sections/{id}
func (s *Server) updateSection(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var sec models.ForumSection
	if err := s.DB.Preload("Author").First(&sec, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Раздел не найден")
		return
	}
	if !s.sectionEditable(u, &sec) {
		httpErr(w, http.StatusForbidden, "Менять раздел может создатель или модератор")
		return
	}
	var p struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Color       *string `json:"color"`
		CoverURL    *string `json:"cover_url"`
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
		upd["title"] = truncRunes(t, 160)
	}
	if p.Description != nil {
		d := truncRunes(strings.TrimSpace(*p.Description), 500)
		if d == "" {
			upd["description"] = nil
		} else {
			upd["description"] = d
		}
	}
	if p.Color != nil {
		if c := truncRunes(strings.TrimSpace(*p.Color), 16); c != "" {
			upd["color"] = c
		}
	}
	if p.CoverURL != nil {
		if *p.CoverURL == "" {
			upd["cover_url"] = nil
		} else {
			upd["cover_url"] = *p.CoverURL
		}
	}
	if len(upd) > 0 {
		s.DB.Model(&models.ForumSection{}).Where("id = ?", id).Updates(upd)
		s.DB.Preload("Author").First(&sec, id)
	}
	var n int64
	s.DB.Model(&models.ForumTopic{}).Where("section_id = ?", id).Count(&n)
	writeJSON(w, http.StatusOK, sectionOut(&sec, u.ID, s.isMod(u.ID), int(n)))
}

// DELETE /forum/sections/{id}
func (s *Server) deleteSection(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var sec models.ForumSection
	if err := s.DB.First(&sec, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Раздел не найден")
		return
	}
	if !s.sectionEditable(u, &sec) {
		httpErr(w, http.StatusForbidden, "Менять раздел может создатель или модератор")
		return
	}
	s.DB.Delete(&models.ForumSection{}, id)
	w.WriteHeader(http.StatusNoContent)
}

// GET /forum/topics
func (s *Server) listTopics(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	q := s.DB.Preload("Posts", func(d *gorm.DB) *gorm.DB { return d.Order("created_at") }).
		Preload("Posts.Author").Preload("Section").Preload("Author")
	if v := r.URL.Query().Get("section_id"); v != "" {
		q = q.Where("section_id = ?", v)
	}
	var topics []models.ForumTopic
	q.Order("pinned DESC, updated_at DESC").Find(&topics)

	reads := map[int]time.Time{}
	var rr []models.ForumTopicRead
	s.DB.Where("user_id = ?", u.ID).Find(&rr)
	for _, x := range rr {
		reads[x.TopicID] = x.LastSeenAt
	}
	out := make([]map[string]any, 0, len(topics))
	for i := range topics {
		out = append(out, topicListItem(&topics[i], reads))
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /forum/topics
func (s *Server) createTopic(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var p struct {
		SectionID int     `json:"section_id"`
		Title     string  `json:"title"`
		Body      string  `json:"body"`
		CoverURL  *string `json:"cover_url"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	title := strings.TrimSpace(p.Title)
	body := strings.TrimSpace(p.Body)
	if title == "" {
		httpErr(w, http.StatusBadRequest, "Нужен заголовок темы")
		return
	}
	if body == "" {
		httpErr(w, http.StatusBadRequest, "Нужно первое сообщение")
		return
	}
	var cnt int64
	s.DB.Model(&models.ForumSection{}).Where("id = ?", p.SectionID).Count(&cnt)
	if cnt == 0 {
		httpErr(w, http.StatusBadRequest, "Выберите раздел")
		return
	}
	topic := models.ForumTopic{SectionID: &p.SectionID, Title: truncRunes(title, 255), AuthorID: &u.ID, CoverURL: p.CoverURL}
	s.DB.Create(&topic)
	s.DB.Create(&models.ForumPost{TopicID: topic.ID, AuthorID: &u.ID, Body: body})
	full := s.loadTopic(topic.ID)
	writeJSON(w, http.StatusCreated, topicOut(full, u.ID))
}

func (s *Server) loadTopic(id int) *models.ForumTopic {
	var t models.ForumTopic
	s.DB.Preload("Posts", func(d *gorm.DB) *gorm.DB { return d.Order("created_at") }).
		Preload("Posts.Author").Preload("Posts.Likes").Preload("Posts.Likes.User").
		Preload("Section").Preload("Author").First(&t, id)
	return &t
}

// GET /forum/topics/{id}
func (s *Server) getTopic(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var exists int64
	s.DB.Model(&models.ForumTopic{}).Where("id = ?", id).Count(&exists)
	if exists == 0 {
		httpErr(w, http.StatusNotFound, "Тема не найдена")
		return
	}
	count := r.URL.Query().Get("count") != "false"
	if count {
		// views++ → в Python onupdate поднимает updated_at, воспроизводим
		s.DB.Model(&models.ForumTopic{}).Where("id = ?", id).Updates(map[string]any{
			"views": gorm.Expr("views + 1"), "updated_at": gorm.Expr("now()"),
		})
	}
	s.markTopicRead(u.ID, id)
	t := s.loadTopic(id)
	writeJSON(w, http.StatusOK, topicOut(t, u.ID))
}

func (s *Server) markTopicRead(userID, topicID int) {
	var tr models.ForumTopicRead
	if err := s.DB.Where("topic_id = ? AND user_id = ?", topicID, userID).First(&tr).Error; err == nil {
		s.DB.Model(&models.ForumTopicRead{}).Where("id = ?", tr.ID).Update("last_seen_at", gorm.Expr("now()"))
	} else {
		s.DB.Create(&models.ForumTopicRead{TopicID: topicID, UserID: userID})
	}
}

// POST /forum/topics/{id}/posts
func (s *Server) addForumPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var cnt int64
	s.DB.Model(&models.ForumTopic{}).Where("id = ?", id).Count(&cnt)
	if cnt == 0 {
		httpErr(w, http.StatusNotFound, "Тема не найдена")
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
	post := models.ForumPost{TopicID: id, AuthorID: &u.ID, Body: body}
	s.DB.Create(&post)
	s.DB.Model(&models.ForumTopic{}).Where("id = ?", id).Update("updated_at", gorm.Expr("now()"))
	s.markTopicRead(u.ID, id)
	var full models.ForumPost
	s.DB.Preload("Author").Preload("Likes").Preload("Likes.User").First(&full, post.ID)
	writeJSON(w, http.StatusCreated, postOut(&full, u.ID))
}

func (s *Server) ownOrMod(u *models.User, p *models.ForumPost, needWindow bool) (int, string) {
	mod := s.isMod(u.ID)
	if (p.AuthorID == nil || *p.AuthorID != u.ID) && !mod {
		return http.StatusForbidden, "Можно менять только свои сообщения"
	}
	if !mod && needWindow && !s.withinForumEdit(p) {
		return http.StatusForbidden, "Время на изменение сообщения истекло"
	}
	return http.StatusOK, ""
}

// PATCH /forum/posts/{id}
func (s *Server) editForumPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var post models.ForumPost
	if err := s.DB.First(&post, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Сообщение не найдено")
		return
	}
	if code, msg := s.ownOrMod(u, &post, true); code != http.StatusOK {
		httpErr(w, code, msg)
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
	s.DB.Model(&models.ForumPost{}).Where("id = ?", id).Updates(map[string]any{
		"body": body, "edited_at": gorm.Expr("now()"), "edit_count": post.EditCount + 1,
	})
	var full models.ForumPost
	s.DB.Preload("Author").Preload("Likes").Preload("Likes.User").First(&full, id)
	writeJSON(w, http.StatusOK, postOut(&full, u.ID))
}

// POST /forum/posts/{id}/like
func (s *Server) toggleLike(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var p struct {
		Emoji string `json:"emoji"`
	}
	_ = decodeJSON(r, &p)
	emoji := truncRunes(strings.TrimSpace(p.Emoji), 16)
	if emoji == "" {
		emoji = "❤️"
	}
	var cnt int64
	s.DB.Model(&models.ForumPost{}).Where("id = ?", id).Count(&cnt)
	if cnt == 0 {
		httpErr(w, http.StatusNotFound, "Сообщение не найдено")
		return
	}
	var existing models.ForumPostLike
	if err := s.DB.Where("post_id = ? AND user_id = ?", id, u.ID).First(&existing).Error; err == nil {
		if existing.Emoji == emoji {
			s.DB.Delete(&models.ForumPostLike{}, existing.ID)
		} else {
			s.DB.Model(&models.ForumPostLike{}).Where("id = ?", existing.ID).Update("emoji", emoji)
		}
	} else {
		s.DB.Create(&models.ForumPostLike{PostID: id, UserID: u.ID, Emoji: emoji})
	}
	var likes []models.ForumPostLike
	s.DB.Preload("User").Where("post_id = ?", id).Find(&likes)
	likers := make([]map[string]any, 0, len(likes))
	liked := false
	for _, l := range likes {
		likers = append(likers, participant(l.User))
		if l.UserID == u.ID {
			liked = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"likes": len(likes), "liked": liked, "likers": likers,
		"reactions": forumReactions(likes, u.ID),
	})
}

// DELETE /forum/posts/{id}
func (s *Server) deleteForumPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var post models.ForumPost
	if err := s.DB.First(&post, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Сообщение не найдено")
		return
	}
	if code, msg := s.ownOrMod(u, &post, true); code != http.StatusOK {
		httpErr(w, code, msg)
		return
	}
	topicID := post.TopicID
	s.DB.Delete(&models.ForumPost{}, id)
	var remaining int64
	s.DB.Model(&models.ForumPost{}).Where("topic_id = ?", topicID).Count(&remaining)
	if remaining == 0 {
		s.DB.Delete(&models.ForumTopic{}, topicID)
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /forum/topics/{id}
func (s *Server) deleteTopic(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var t models.ForumTopic
	if err := s.DB.First(&t, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Тема не найдена")
		return
	}
	if (t.AuthorID == nil || *t.AuthorID != u.ID) && !s.isMod(u.ID) {
		httpErr(w, http.StatusForbidden, "Удалять тему может автор или модератор")
		return
	}
	s.DB.Delete(&models.ForumTopic{}, id)
	w.WriteHeader(http.StatusNoContent)
}
