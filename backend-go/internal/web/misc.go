package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"manibandha/internal/models"
)

// ── События ─────────────────────────────────────────────────────────────────

func eventOut(e *models.Event) map[string]any {
	return map[string]any{
		"id": e.ID, "title": e.Title, "location": e.Location,
		"starts_on": dateStr(&e.StartsOn), "ends_on": dateStr(e.EndsOn),
		"description": e.Description,
	}
}

func eventBrief(e *models.Event) map[string]any {
	return map[string]any{
		"id": e.ID, "title": e.Title, "location": e.Location, "description": e.Description,
		"starts_on": dateStr(&e.StartsOn), "ends_on": dateStr(e.EndsOn),
	}
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	var rows []models.Event
	s.DB.Order("starts_on DESC").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		out = append(out, eventOut(&rows[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) publicUpcoming(w http.ResponseWriter, r *http.Request) {
	today := time.Now().Format("2006-01-02")
	var rows []models.Event
	s.DB.Where("ends_on >= ? OR (ends_on IS NULL AND starts_on >= ?)", today, today).
		Order("starts_on ASC").Limit(8).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		out = append(out, eventBrief(&rows[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) publicList(w http.ResponseWriter, r *http.Request) {
	var rows []models.Event
	s.DB.Order("starts_on DESC").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		out = append(out, eventBrief(&rows[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) publicDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var e models.Event
	if err := s.DB.First(&e, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Событие не найдено")
		return
	}
	writeJSON(w, http.StatusOK, eventBrief(&e))
}

func (s *Server) getEvent(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var e models.Event
	if err := s.DB.First(&e, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Событие не найдено")
		return
	}
	writeJSON(w, http.StatusOK, eventOut(&e))
}

type eventInput struct {
	Title       *string   `json:"title"`
	Location    *string   `json:"location"`
	StartsOn    *jsonDate `json:"starts_on"`
	EndsOn      *jsonDate `json:"ends_on"`
	Description *string   `json:"description"`
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	var in eventInput
	if err := decodeJSON(r, &in); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	e := models.Event{}
	if in.Title != nil {
		e.Title = *in.Title
	}
	e.Location = in.Location
	e.Description = in.Description
	if in.StartsOn != nil && in.StartsOn.t != nil {
		e.StartsOn = *in.StartsOn.t
	}
	if in.EndsOn != nil {
		e.EndsOn = in.EndsOn.t
	}
	if err := s.DB.Create(&e).Error; err != nil {
		httpErr(w, http.StatusBadRequest, "Не удалось создать событие")
		return
	}
	writeJSON(w, http.StatusCreated, eventOut(&e))
}

func (s *Server) updateEvent(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var e models.Event
	if err := s.DB.First(&e, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Событие не найдено")
		return
	}
	var in eventInput
	if err := decodeJSON(r, &in); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	upd := map[string]any{}
	if in.Title != nil {
		upd["title"] = *in.Title
	}
	if in.Location != nil {
		upd["location"] = *in.Location
	}
	if in.Description != nil {
		upd["description"] = *in.Description
	}
	if in.StartsOn != nil {
		upd["starts_on"] = in.StartsOn.t
	}
	if in.EndsOn != nil {
		upd["ends_on"] = in.EndsOn.t
	}
	if len(upd) > 0 {
		s.DB.Model(&models.Event{}).Where("id = ?", id).Updates(upd)
		s.DB.First(&e, id)
	}
	writeJSON(w, http.StatusOK, eventOut(&e))
}

func (s *Server) deleteEvent(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var e models.Event
	if err := s.DB.First(&e, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Событие не найдено")
		return
	}
	s.DB.Delete(&models.Event{}, id)
	w.WriteHeader(http.StatusNoContent)
}

// ── Черновики ───────────────────────────────────────────────────────────────

func (s *Server) getDraft(w http.ResponseWriter, r *http.Request) {
	scope := chi.URLParam(r, "scope")
	u := currentUser(r)
	var d models.Draft
	body := ""
	if err := s.DB.Where("user_id = ? AND scope = ?", u.ID, scope).First(&d).Error; err == nil {
		body = d.Body
	}
	writeJSON(w, http.StatusOK, map[string]any{"body": body})
}

func (s *Server) saveDraft(w http.ResponseWriter, r *http.Request) {
	scope := chi.URLParam(r, "scope")
	u := currentUser(r)
	var p struct {
		Body string `json:"body"`
	}
	_ = decodeJSON(r, &p)
	var d models.Draft
	if err := s.DB.Where("user_id = ? AND scope = ?", u.ID, scope).First(&d).Error; err == nil {
		s.DB.Model(&models.Draft{}).Where("id = ?", d.ID).Update("body", p.Body)
	} else {
		s.DB.Create(&models.Draft{UserID: u.ID, Scope: scope, Body: p.Body})
	}
	writeJSON(w, http.StatusOK, map[string]any{"body": p.Body})
}

func (s *Server) deleteDraft(w http.ResponseWriter, r *http.Request) {
	scope := chi.URLParam(r, "scope")
	u := currentUser(r)
	s.DB.Where("user_id = ? AND scope = ?", u.ID, scope).Delete(&models.Draft{})
	w.WriteHeader(http.StatusNoContent)
}

// ── Настройки ───────────────────────────────────────────────────────────────

func (s *Server) setSetting(key, value string) {
	var st models.AppSetting
	if err := s.DB.Where("key = ?", key).First(&st).Error; err == nil {
		s.DB.Model(&models.AppSetting{}).Where("key = ?", key).Update("value", value)
	} else {
		s.DB.Create(&models.AppSetting{Key: key, Value: value})
	}
}

func (s *Server) settingsBody() map[string]any {
	return map[string]any{
		"forum_edit_window_minutes": s.getIntSetting("forum_edit_window_minutes", 60),
		"auth_expire_days":          s.getIntSetting("auth_expire_days", 30),
		"recording_enabled":         s.getIntSetting("recording_enabled", 1) != 0,
		"recording_height":          s.getIntSetting("recording_height", 720),
	}
}

func (s *Server) readSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.settingsBody())
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var p map[string]any
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	applyInt := func(key string, lo, hi int) bool {
		v, ok := p[key]
		if !ok || v == nil {
			return true
		}
		n, ok := toInt(v)
		if !ok {
			httpErr(w, http.StatusBadRequest, key+" должно быть числом")
			return false
		}
		if n < lo || n > hi {
			httpErr(w, http.StatusBadRequest, "Недопустимое значение для "+key)
			return false
		}
		s.setSetting(key, strconv.Itoa(n))
		return true
	}
	if !applyInt("forum_edit_window_minutes", 0, 100000) {
		return
	}
	if !applyInt("auth_expire_days", 1, 3650) {
		return
	}
	if v, ok := p["recording_enabled"]; ok {
		if truthy(v) {
			s.setSetting("recording_enabled", "1")
		} else {
			s.setSetting("recording_enabled", "0")
		}
	}
	if n, ok := toInt(p["recording_height"]); ok && (n == 480 || n == 720 || n == 1080) {
		s.setSetting("recording_height", strconv.Itoa(n))
	}
	writeJSON(w, http.StatusOK, s.settingsBody())
}

func toInt(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case string:
		n, err := strconv.Atoi(t)
		return n, err == nil
	}
	return 0, false
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		return t != "" && t != "0" && t != "false"
	case nil:
		return false
	}
	return true
}
