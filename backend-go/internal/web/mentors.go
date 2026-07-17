package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"manibandha/internal/models"
	"manibandha/internal/security"
)

// GET /mentors — кураторы как справочник наставников.
func (s *Server) listMentorsDict(w http.ResponseWriter, r *http.Request) {
	var rows []models.User
	s.DB.Where("role = ?", "curator").Order("full_name").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, u := range rows {
		out = append(out, map[string]any{"id": u.ID, "name": u.FullName})
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /mentors — создать наставника (инертный аккаунт-куратор).
func (s *Server) createMentor(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		httpErr(w, http.StatusBadRequest, "Имя наставника обязательно")
		return
	}
	u := models.User{
		FullName:       name,
		Email:          "mentor-" + randHex()[:10] + "@manibandha.local",
		Role:           "curator",
		IsActive:       true,
		HashedPassword: security.HashPassword(security.RandToken(16)),
	}
	s.DB.Create(&u)
	writeJSON(w, http.StatusCreated, map[string]any{"id": u.ID, "name": u.FullName})
}

// PATCH /mentors/{id} — переименовать.
func (s *Server) renameMentor(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var u models.User
	if err := s.DB.First(&u, id).Error; err != nil || u.Role != "curator" {
		httpErr(w, http.StatusNotFound, "Наставник не найден")
		return
	}
	var p struct {
		Name string `json:"name"`
	}
	_ = decodeJSON(r, &p)
	if strings.TrimSpace(p.Name) != "" {
		u.FullName = strings.TrimSpace(p.Name)
		s.DB.Model(&models.User{}).Where("id = ?", u.ID).Update("full_name", u.FullName)
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": u.ID, "name": u.FullName})
}

// DELETE /mentors/{id}
func (s *Server) deleteMentor(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var u models.User
	if err := s.DB.First(&u, id).Error; err != nil || u.Role != "curator" {
		httpErr(w, http.StatusNotFound, "Наставник не найден")
		return
	}
	s.DB.Delete(&models.User{}, u.ID)
	w.WriteHeader(http.StatusNoContent)
}
