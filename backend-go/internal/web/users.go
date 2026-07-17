package web

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"manibandha/internal/models"
	"manibandha/internal/security"
)

// staff — гейт «гуру или секретарь» (аналог staff_user в deps.py).
func (s *Server) staff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := currentUser(r)
		if u == nil || (u.Role != "guru" && u.Role != "secretary") {
			httpErr(w, http.StatusForbidden, "Недостаточно прав")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GET /users — все пользователи (staff).
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	var users []models.User
	s.DB.Order("full_name").Find(&users)
	writeJSON(w, http.StatusOK, users)
}

// GET /users/mentors — кураторы/гуру для назначения наставником.
func (s *Server) listMentors(w http.ResponseWriter, r *http.Request) {
	var users []models.User
	s.DB.Where("role IN ? AND is_active = ?", []string{"curator", "guru"}, true).
		Order("full_name").Find(&users)
	// UserBrief: id, full_name, role
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		out = append(out, map[string]any{"id": u.ID, "full_name": u.FullName, "role": u.Role})
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /users — создать пользователя (staff).
func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Email      string  `json:"email"`
		Phone      *string `json:"phone"`
		FullName   string  `json:"full_name"`
		Role       *string `json:"role"`
		IsActive   *bool   `json:"is_active"`
		DiscipleID *int    `json:"disciple_id"`
		Password   string  `json:"password"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	var cnt int64
	s.DB.Model(&models.User{}).Where("email = ?", p.Email).Count(&cnt)
	if cnt > 0 {
		httpErr(w, http.StatusBadRequest, "Пользователь с таким email уже существует")
		return
	}
	var phone *string
	if p.Phone != nil && *p.Phone != "" {
		ph := normalizePhone(*p.Phone)
		if ph != "" {
			var c int64
			s.DB.Model(&models.User{}).Where("phone = ?", ph).Count(&c)
			if c > 0 {
				httpErr(w, http.StatusBadRequest, "Пользователь с таким телефоном уже существует")
				return
			}
			phone = &ph
		}
	}
	role := "secretary"
	if p.Role != nil && *p.Role != "" {
		role = *p.Role
	}
	active := true
	if p.IsActive != nil {
		active = *p.IsActive
	}
	u := models.User{
		Email:          p.Email,
		Phone:          phone,
		FullName:       p.FullName,
		Role:           role,
		IsActive:       active,
		DiscipleID:     p.DiscipleID,
		HashedPassword: security.HashPassword(p.Password),
	}
	if err := s.DB.Create(&u).Error; err != nil {
		httpErr(w, http.StatusBadRequest, "Не удалось создать пользователя")
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

// PATCH /users/{id} — обновить (staff).
func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var u models.User
	if err := s.DB.First(&u, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Пользователь не найден")
		return
	}
	var p struct {
		FullName   *string `json:"full_name"`
		Phone      *string `json:"phone"`
		Role       *string `json:"role"`
		IsActive   *bool   `json:"is_active"`
		Password   *string `json:"password"`
		DiscipleID *int    `json:"disciple_id"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	upd := map[string]any{}
	if p.Password != nil && *p.Password != "" {
		upd["hashed_password"] = security.HashPassword(*p.Password)
	}
	if p.Phone != nil {
		ph := normalizePhone(*p.Phone)
		if ph != "" {
			var c int64
			s.DB.Model(&models.User{}).Where("phone = ? AND id <> ?", ph, u.ID).Count(&c)
			if c > 0 {
				httpErr(w, http.StatusBadRequest, "Пользователь с таким телефоном уже существует")
				return
			}
			upd["phone"] = ph
		} else {
			upd["phone"] = nil
		}
	}
	if p.FullName != nil {
		upd["full_name"] = *p.FullName
	}
	if p.Role != nil {
		upd["role"] = *p.Role
	}
	if p.IsActive != nil {
		upd["is_active"] = *p.IsActive
	}
	if p.DiscipleID != nil {
		upd["disciple_id"] = *p.DiscipleID
	}
	if len(upd) > 0 {
		s.DB.Model(&models.User{}).Where("id = ?", u.ID).Updates(upd)
		s.DB.First(&u, u.ID)
	}
	writeJSON(w, http.StatusOK, u)
}

// DELETE /users/{id} — удалить (нельзя себя).
func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	cur := currentUser(r)
	var u models.User
	if err := s.DB.First(&u, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Пользователь не найден")
		return
	}
	if u.ID == cur.ID {
		httpErr(w, http.StatusBadRequest, "Нельзя удалить самого себя")
		return
	}
	s.DB.Delete(&models.User{}, u.ID)
	w.WriteHeader(http.StatusNoContent)
}
