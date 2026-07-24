package web

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"manibandha/internal/caps"
	"manibandha/internal/models"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify — латиница/цифры/дефис из имени (кириллица транслитерируется грубо).
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var tr = map[rune]string{
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh",
		'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o",
		'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "ts",
		'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
	}
	var b strings.Builder
	for _, r := range s {
		if t, ok := tr[r]; ok {
			b.WriteString(t)
		} else {
			b.WriteRune(r)
		}
	}
	out := slugRe.ReplaceAllString(b.String(), "-")
	return strings.Trim(out, "-")
}

func (s *Server) spaceOut(sp models.Space, userID int, myStatus string, memberCount int) map[string]any {
	return map[string]any{
		"id":            sp.ID,
		"slug":          sp.Slug,
		"name":          sp.Name,
		"type":          sp.Type,
		"join_mode":     sp.JoinMode,
		"custom_domain": sp.CustomDomain,
		"owner_user_id": sp.OwnerUserID,
		"is_owner":      sp.OwnerUserID != nil && *sp.OwnerUserID == userID,
		"my_status":     myStatus, // "" | active | pending | rejected
		"member_count":  memberCount,
	}
}

func (s *Server) memberStatuses(userID int) map[int]string {
	var mems []models.SpaceMember
	s.DB.Where("user_id = ?", userID).Find(&mems)
	out := map[int]string{}
	for _, m := range mems {
		out[m.SpaceID] = m.Status
	}
	return out
}

func (s *Server) activeMemberCounts() map[int]int {
	type row struct {
		SpaceID int
		N       int
	}
	var rows []row
	s.DB.Model(&models.SpaceMember{}).
		Select("space_id, count(*) as n").Where("status = ?", "active").
		Group("space_id").Scan(&rows)
	out := map[int]int{}
	for _, r := range rows {
		out[r.SpaceID] = r.N
	}
	return out
}

// GET /spaces — каталог всех пространств со статусом моего участия.
func (s *Server) listSpaces(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var spaces []models.Space
	s.DB.Order("id").Find(&spaces)
	statuses := s.memberStatuses(u.ID)
	counts := s.activeMemberCounts()
	out := make([]map[string]any, 0, len(spaces))
	for _, sp := range spaces {
		out = append(out, s.spaceOut(sp, u.ID, statuses[sp.ID], counts[sp.ID]))
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /spaces/{slug} — одно пространство по slug.
func (s *Server) getSpace(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	slug := chi.URLParam(r, "slug")
	var sp models.Space
	if err := s.DB.Where("slug = ?", slug).First(&sp).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Пространство не найдено")
		return
	}
	statuses := s.memberStatuses(u.ID)
	counts := s.activeMemberCounts()
	writeJSON(w, http.StatusOK, s.spaceOut(sp, u.ID, statuses[sp.ID], counts[sp.ID]))
}

// POST /spaces — создать пространство. Создатель = владелец (модератор) + активный участник.
func (s *Server) createSpace(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var p struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
		Type string `json:"type"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		httpErr(w, http.StatusBadRequest, "Укажите название пространства")
		return
	}
	spaceType := strings.TrimSpace(p.Type)
	if spaceType == "" {
		spaceType = "guru"
	}
	slug := slugify(p.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	if slug == "" {
		httpErr(w, http.StatusBadRequest, "Не удалось построить адрес (slug) — задайте латиницей")
		return
	}
	// уникальный slug: при коллизии добавляем -2, -3, …
	base := slug
	for i := 2; ; i++ {
		var cnt int64
		s.DB.Model(&models.Space{}).Where("slug = ?", slug).Count(&cnt)
		if cnt == 0 {
			break
		}
		slug = base + "-" + strconv.Itoa(i)
	}
	owner := u.ID
	sp := models.Space{Slug: slug, Name: name, Type: spaceType, OwnerUserID: &owner, JoinMode: "request"}
	if err := s.DB.Create(&sp).Error; err != nil {
		httpErr(w, http.StatusInternalServerError, "Не удалось создать пространство")
		return
	}
	s.DB.Save(&models.SpaceMember{SpaceID: sp.ID, UserID: u.ID, Status: "active"})
	writeJSON(w, http.StatusOK, s.spaceOut(sp, u.ID, "active", 1))
}

// POST /spaces/{id}/join — вступить (по заявке → pending; открытое → active).
func (s *Server) joinSpace(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var sp models.Space
	if err := s.DB.First(&sp, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Пространство не найдено")
		return
	}
	// уже участник — вернуть текущий статус
	var existing models.SpaceMember
	if err := s.DB.Where("space_id = ? AND user_id = ?", id, u.ID).First(&existing).Error; err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": existing.Status})
		return
	}
	status := "active"
	if sp.JoinMode == "request" {
		status = "pending"
	}
	s.DB.Save(&models.SpaceMember{SpaceID: id, UserID: u.ID, Status: status})
	writeJSON(w, http.StatusOK, map[string]any{"status": status})
}

// DELETE /spaces/{id}/join — выйти из пространства (владелец выйти не может).
func (s *Server) leaveSpace(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var sp models.Space
	if err := s.DB.First(&sp, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Пространство не найдено")
		return
	}
	if sp.OwnerUserID != nil && *sp.OwnerUserID == u.ID {
		httpErr(w, http.StatusBadRequest, "Владелец не может выйти из своего пространства")
		return
	}
	s.DB.Where("space_id = ? AND user_id = ?", id, u.ID).Delete(&models.SpaceMember{})
	writeJSON(w, http.StatusOK, map[string]any{"status": ""})
}

// GET /spaces/{id}/members — участники пространства (модератор): для управления заявками/правами.
func (s *Server) listSpaceMembers(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var mems []models.SpaceMember
	s.DB.Where("space_id = ?", id).Order("status, joined_at").Find(&mems)
	ids := make([]int, 0, len(mems))
	for _, m := range mems {
		ids = append(ids, m.UserID)
	}
	users := map[int]models.User{}
	if len(ids) > 0 {
		var us []models.User
		s.DB.Where("id IN ?", ids).Find(&us)
		for _, x := range us {
			users[x.ID] = x
		}
	}
	out := make([]map[string]any, 0, len(mems))
	for _, m := range mems {
		usr := users[m.UserID]
		out = append(out, map[string]any{
			"user_id": m.UserID,
			"name":    usr.FullName,
			"email":   usr.Email,
			"status":  m.Status,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// PUT /spaces/{id}/members/{uid} — сменить статус участника (модератор): approve/reject.
func (s *Server) setSpaceMemberStatus(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	uid, _ := strconv.Atoi(chi.URLParam(r, "uid"))
	var p struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	if p.Status != "active" && p.Status != "rejected" {
		httpErr(w, http.StatusBadRequest, "Недопустимый статус")
		return
	}
	s.DB.Model(&models.SpaceMember{}).
		Where("space_id = ? AND user_id = ?", id, uid).Update("status", p.Status)
	writeJSON(w, http.StatusOK, map[string]any{"user_id": uid, "status": p.Status})
}

// DELETE /spaces/{id}/members/{uid} — удалить участника (модератор; владельца нельзя).
func (s *Server) removeSpaceMember(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	uid, _ := strconv.Atoi(chi.URLParam(r, "uid"))
	var sp models.Space
	if err := s.DB.First(&sp, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Пространство не найдено")
		return
	}
	if sp.OwnerUserID != nil && *sp.OwnerUserID == uid {
		httpErr(w, http.StatusBadRequest, "Нельзя удалить владельца пространства")
		return
	}
	s.DB.Where("space_id = ? AND user_id = ?", id, uid).Delete(&models.SpaceMember{})
	writeJSON(w, http.StatusOK, map[string]any{"user_id": uid, "status": ""})
}

// requireSpaceModerator — гейт «модератор пространства из URL {id}» (для управления участниками).
func (s *Server) requireSpaceModerator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := currentUser(r)
		id, _ := strconv.Atoi(chi.URLParam(r, "id"))
		if u == nil || !caps.IsModerator(s.DB, u.ID, id) {
			httpErr(w, http.StatusForbidden, "Только модератор пространства")
			return
		}
		next.ServeHTTP(w, r)
	})
}
