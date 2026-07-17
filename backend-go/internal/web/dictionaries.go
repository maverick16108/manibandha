package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"manibandha/internal/models"
)

// ── Города ─────────────────────────────────────────────────────────────────

func (s *Server) listCities(w http.ResponseWriter, r *http.Request) {
	var v []models.City
	s.DB.Order("name").Find(&v)
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) createCity(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Name    string  `json:"name"`
		Country *string `json:"country"`
		Region  *string `json:"region"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		httpErr(w, http.StatusBadRequest, "Название города обязательно")
		return
	}
	var cnt int64
	s.DB.Model(&models.City{}).Where("name ILIKE ?", name).Count(&cnt)
	if cnt > 0 {
		httpErr(w, http.StatusBadRequest, "Такой город уже есть")
		return
	}
	c := models.City{Name: name, Country: emptyToNil(p.Country), Region: emptyToNil(p.Region)}
	s.DB.Create(&c)
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) updateCity(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var c models.City
	if err := s.DB.First(&c, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Город не найден")
		return
	}
	var p map[string]any
	_ = decodeJSON(r, &p)
	upd := map[string]any{}
	if v, ok := p["name"]; ok {
		upd["name"] = v
	}
	if v, ok := p["country"]; ok {
		upd["country"] = v
	}
	if v, ok := p["region"]; ok {
		upd["region"] = v
	}
	if len(upd) > 0 {
		s.DB.Model(&models.City{}).Where("id = ?", c.ID).Updates(upd)
		s.DB.First(&c, c.ID)
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) deleteCity(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var c models.City
	if err := s.DB.First(&c, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Город не найден")
		return
	}
	s.DB.Delete(&models.City{}, c.ID)
	w.WriteHeader(http.StatusNoContent)
}

// ── Регионы / Страны (только name) ──────────────────────────────────────────

func (s *Server) listRegions(w http.ResponseWriter, r *http.Request) {
	var v []models.Region
	s.DB.Order("name").Find(&v)
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) createRegion(w http.ResponseWriter, r *http.Request) {
	name, ok := s.readName(w, r, "Название региона обязательно")
	if !ok {
		return
	}
	var cnt int64
	s.DB.Model(&models.Region{}).Where("name ILIKE ?", name).Count(&cnt)
	if cnt > 0 {
		httpErr(w, http.StatusBadRequest, "Такой регион уже есть")
		return
	}
	v := models.Region{Name: name}
	s.DB.Create(&v)
	writeJSON(w, http.StatusCreated, v)
}

func (s *Server) updateRegion(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var v models.Region
	if err := s.DB.First(&v, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Регион не найден")
		return
	}
	var p struct {
		Name *string `json:"name"`
	}
	_ = decodeJSON(r, &p)
	if p.Name != nil {
		s.DB.Model(&models.Region{}).Where("id = ?", v.ID).Update("name", *p.Name)
		s.DB.First(&v, v.ID)
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) deleteRegion(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var v models.Region
	if err := s.DB.First(&v, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Регион не найден")
		return
	}
	s.DB.Delete(&models.Region{}, v.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listCountries(w http.ResponseWriter, r *http.Request) {
	var v []models.Country
	s.DB.Order("name").Find(&v)
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) createCountry(w http.ResponseWriter, r *http.Request) {
	name, ok := s.readName(w, r, "Название страны обязательно")
	if !ok {
		return
	}
	var cnt int64
	s.DB.Model(&models.Country{}).Where("name ILIKE ?", name).Count(&cnt)
	if cnt > 0 {
		httpErr(w, http.StatusBadRequest, "Такая страна уже есть")
		return
	}
	v := models.Country{Name: name}
	s.DB.Create(&v)
	writeJSON(w, http.StatusCreated, v)
}

func (s *Server) updateCountry(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var v models.Country
	if err := s.DB.First(&v, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Страна не найдена")
		return
	}
	var p struct {
		Name *string `json:"name"`
	}
	_ = decodeJSON(r, &p)
	if p.Name != nil {
		s.DB.Model(&models.Country{}).Where("id = ?", v.ID).Update("name", *p.Name)
		s.DB.First(&v, v.ID)
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) deleteCountry(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var v models.Country
	if err := s.DB.First(&v, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Страна не найдена")
		return
	}
	s.DB.Delete(&models.Country{}, v.ID)
	w.WriteHeader(http.StatusNoContent)
}

// ── Храмы ───────────────────────────────────────────────────────────────────

func (s *Server) listTemples(w http.ResponseWriter, r *http.Request) {
	var v []models.Temple
	s.DB.Order("name").Find(&v)
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) createTemple(w http.ResponseWriter, r *http.Request) {
	var t models.Temple
	if err := decodeJSON(r, &t); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	t.ID = 0
	s.DB.Create(&t)
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) updateTemple(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var t models.Temple
	if err := s.DB.First(&t, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Храм не найден")
		return
	}
	var p map[string]any
	_ = decodeJSON(r, &p)
	upd := map[string]any{}
	for _, k := range []string{"name", "city", "country", "president_name", "notes"} {
		if v, ok := p[k]; ok {
			upd[k] = v
		}
	}
	if len(upd) > 0 {
		s.DB.Model(&models.Temple{}).Where("id = ?", t.ID).Updates(upd)
		s.DB.First(&t, t.ID)
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) deleteTemple(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var t models.Temple
	if err := s.DB.First(&t, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Храм не найден")
		return
	}
	s.DB.Delete(&models.Temple{}, t.ID)
	w.WriteHeader(http.StatusNoContent)
}

// ── helpers ─────────────────────────────────────────────────────────────────

func (s *Server) readName(w http.ResponseWriter, r *http.Request, emptyMsg string) (string, bool) {
	var p struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return "", false
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		httpErr(w, http.StatusBadRequest, emptyMsg)
		return "", false
	}
	return name, true
}

func emptyToNil(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}
