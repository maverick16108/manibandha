package web

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"manibandha/internal/models"
)

func checklistItemOut(c *models.ChecklistItem) map[string]any {
	return map[string]any{
		"id": c.ID, "disciple_id": c.DiscipleID, "title": c.Title,
		"is_done": c.IsDone, "note": c.Note, "target": c.Target,
	}
}

func (s *Server) discipleAccessible(r *http.Request, u *models.User, id int) bool {
	var d models.Disciple
	return scopeDisciples(s.db(r).Model(&models.Disciple{}), u).Where("id = ?", id).Select("id").First(&d).Error == nil
}

func canEditChecklist(u *models.User) bool {
	return u.Role == "guru" || u.Role == "secretary" || u.Role == "curator"
}

// GET /disciples/{id}/checklist
func (s *Server) listChecklist(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if !s.discipleAccessible(r, currentUser(r), id) {
		httpErr(w, http.StatusNotFound, "Ученик не найден")
		return
	}
	var items []models.ChecklistItem
	s.DB.Where("disciple_id = ?", id).Order("id").Find(&items)
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		out = append(out, checklistItemOut(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /disciples/{id}/checklist
func (s *Server) addChecklist(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	if !canEditChecklist(u) {
		httpErr(w, http.StatusForbidden, "Недостаточно прав")
		return
	}
	if !s.discipleAccessible(r, u, id) {
		httpErr(w, http.StatusNotFound, "Ученик не найден")
		return
	}
	var p struct {
		Title  string  `json:"title"`
		IsDone *bool   `json:"is_done"`
		Note   *string `json:"note"`
		Target *string `json:"target"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	item := models.ChecklistItem{DiscipleID: id, Title: p.Title, Target: "harinama", Note: p.Note}
	if p.IsDone != nil {
		item.IsDone = *p.IsDone
	}
	if p.Target != nil && *p.Target != "" {
		item.Target = *p.Target
	}
	s.DB.Create(&item)
	writeJSON(w, http.StatusCreated, checklistItemOut(&item))
}

// PATCH /disciples/{id}/checklist/{itemId}
func (s *Server) updateChecklist(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	itemID, _ := strconv.Atoi(chi.URLParam(r, "itemId"))
	u := currentUser(r)
	if !canEditChecklist(u) {
		httpErr(w, http.StatusForbidden, "Недостаточно прав")
		return
	}
	if !s.discipleAccessible(r, u, id) {
		httpErr(w, http.StatusNotFound, "Ученик не найден")
		return
	}
	var item models.ChecklistItem
	if err := s.DB.First(&item, itemID).Error; err != nil || item.DiscipleID != id {
		httpErr(w, http.StatusNotFound, "Пункт не найден")
		return
	}
	var p struct {
		Title  *string `json:"title"`
		IsDone *bool   `json:"is_done"`
		Note   *string `json:"note"`
		Target *string `json:"target"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	upd := map[string]any{}
	if p.Title != nil {
		upd["title"] = *p.Title
	}
	if p.IsDone != nil {
		upd["is_done"] = *p.IsDone
	}
	if p.Note != nil {
		upd["note"] = *p.Note
	}
	if p.Target != nil {
		upd["target"] = *p.Target
	}
	if len(upd) > 0 {
		s.DB.Model(&models.ChecklistItem{}).Where("id = ?", itemID).Updates(upd)
		s.DB.First(&item, itemID)
	}
	writeJSON(w, http.StatusOK, checklistItemOut(&item))
}

// DELETE /disciples/{id}/checklist/{itemId} (staff)
func (s *Server) deleteChecklist(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	itemID, _ := strconv.Atoi(chi.URLParam(r, "itemId"))
	var item models.ChecklistItem
	if err := s.DB.First(&item, itemID).Error; err != nil || item.DiscipleID != id {
		httpErr(w, http.StatusNotFound, "Пункт не найден")
		return
	}
	s.DB.Delete(&models.ChecklistItem{}, itemID)
	w.WriteHeader(http.StatusNoContent)
}
