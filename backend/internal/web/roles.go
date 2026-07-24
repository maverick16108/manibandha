package web

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"manibandha/internal/caps"
	"manibandha/internal/models"
)

func roleOut(r *models.Role) map[string]any {
	c := r.Capabilities
	if c == nil {
		c = models.StringList{}
	}
	return map[string]any{
		"id": r.ID, "key": r.Key, "name": r.Name,
		"is_system": r.IsSystem, "is_superadmin": r.IsSuperadmin, "is_default": r.IsDefault,
		"capabilities": c,
	}
}

func filterCaps(in []string) models.StringList {
	out := models.StringList{}
	for _, c := range in {
		if caps.IsCap(c) {
			out = append(out, c)
		}
	}
	return out
}

// GET /roles
func (s *Server) listRoles(w http.ResponseWriter, r *http.Request) {
	var roles []models.Role
	s.db(r).Order("is_system DESC, id").Find(&roles)
	out := make([]map[string]any, 0, len(roles))
	for i := range roles {
		out = append(out, roleOut(&roles[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /roles
func (s *Server) createRole(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Key          string   `json:"key"`
		Name         string   `json:"name"`
		IsDefault    bool     `json:"is_default"`
		Capabilities []string `json:"capabilities"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	key := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(p.Key)), " ", "_")
	name := strings.TrimSpace(p.Name)
	if name == "" {
		httpErr(w, http.StatusBadRequest, "Укажите название роли")
		return
	}
	if key == "" {
		key = strings.ReplaceAll(strings.ToLower(name), " ", "_")
	}
	var cnt int64
	s.db(r).Model(&models.Role{}).Where("key = ?", key).Count(&cnt)
	if cnt > 0 {
		httpErr(w, http.StatusBadRequest, "Роль с таким ключом уже существует")
		return
	}
	role := models.Role{
		Key: key, Name: name, IsSystem: false, IsSuperadmin: false,
		IsDefault: p.IsDefault, Capabilities: filterCaps(p.Capabilities),
	}
	if role.IsDefault {
		s.db(r).Model(&models.Role{}).Where("1 = 1").Update("is_default", false)
	}
	s.db(r).Create(&role)
	writeJSON(w, http.StatusCreated, roleOut(&role))
}

// PUT /roles/{id}
func (s *Server) updateRole(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var role models.Role
	if err := s.db(r).First(&role, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Роль не найдена")
		return
	}
	if role.IsSuperadmin {
		httpErr(w, http.StatusBadRequest, "Роль гуру не редактируется")
		return
	}
	var p map[string]any
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	upd := map[string]any{}
	if v, ok := p["name"].(string); ok && strings.TrimSpace(v) != "" {
		upd["name"] = strings.TrimSpace(v)
	}
	if raw, ok := p["capabilities"].([]any); ok {
		list := make([]string, 0, len(raw))
		for _, x := range raw {
			if s, ok := x.(string); ok {
				list = append(list, s)
			}
		}
		upd["capabilities"] = filterCaps(list)
	}
	if v, ok := p["is_default"]; ok {
		def, _ := v.(bool)
		upd["is_default"] = def
		if def {
			s.db(r).Model(&models.Role{}).Where("id <> ?", role.ID).Update("is_default", false)
		}
	}
	if len(upd) > 0 {
		s.db(r).Model(&models.Role{}).Where("id = ?", role.ID).Updates(upd)
		s.db(r).First(&role, role.ID)
	}
	writeJSON(w, http.StatusOK, roleOut(&role))
}

// DELETE /roles/{id}
func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var role models.Role
	if err := s.db(r).First(&role, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Роль не найдена")
		return
	}
	if role.IsSystem {
		httpErr(w, http.StatusBadRequest, "Системную роль удалить нельзя")
		return
	}
	s.DB.Where("role_id = ?", role.ID).Delete(&models.UserRole{})
	s.db(r).Delete(&models.Role{}, role.ID)
	w.WriteHeader(http.StatusNoContent)
}

// GET /users/{id}/roles
func (s *Server) getUserRoles(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var urs []models.UserRole
	s.DB.Where("user_id = ?", id).Find(&urs)
	ids := make([]int, 0, len(urs))
	for _, ur := range urs {
		ids = append(ids, ur.RoleID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"role_ids": ids})
}

// PUT /users/{id}/roles
func (s *Server) setUserRoles(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var u models.User
	if err := s.DB.First(&u, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Пользователь не найден")
		return
	}
	var p struct {
		RoleIDs []int `json:"role_ids"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	valid := []int{}
	if len(p.RoleIDs) > 0 {
		var roles []models.Role
		s.db(r).Where("id IN ?", p.RoleIDs).Find(&roles)
		for _, r := range roles {
			valid = append(valid, r.ID)
		}
	}
	s.DB.Where("user_id = ?", id).Delete(&models.UserRole{})
	for _, rid := range valid {
		s.DB.Create(&models.UserRole{UserID: id, RoleID: rid})
	}
	sort.Ints(valid)
	writeJSON(w, http.StatusOK, map[string]any{"role_ids": valid})
}
