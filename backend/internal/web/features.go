package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"manibandha/internal/caps"
	"manibandha/internal/feat"
)

// GET /space/features — каталог модулей пространства с эффективным состоянием (для управления).
func (s *Server) listSpaceFeatures(w http.ResponseWriter, r *http.Request) {
	eff := feat.EnabledFeatures(s.DB, caps.HomeSpaceID)
	items := make([]map[string]any, 0, len(feat.Catalog))
	for _, f := range feat.Catalog {
		items = append(items, map[string]any{"key": f.Key, "label": f.Label, "enabled": eff[f.Key]})
	}
	u := currentUser(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"items":      items,
		"can_manage": u != nil && caps.IsModerator(s.DB, u.ID, caps.HomeSpaceID),
	})
}

// PUT /space/features/{key} — включить/выключить модуль (только модератор пространства).
func (s *Server) setSpaceFeature(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if !feat.IsKnown(key) {
		httpErr(w, http.StatusNotFound, "Неизвестный модуль")
		return
	}
	var p struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	if err := feat.Set(s.DB, caps.HomeSpaceID, key, p.Enabled); err != nil {
		httpErr(w, http.StatusInternalServerError, "Не удалось сохранить")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key, "enabled": p.Enabled})
}
