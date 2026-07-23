package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"manibandha/internal/models"
)

// GET /gallery/albums — список альбомов (с обложкой и числом фото). Право gallery.view.
func (s *Server) listGalleryAlbums(w http.ResponseWriter, r *http.Request) {
	var albums []models.GalleryAlbum
	s.DB.Order("is_home DESC, sort_order ASC, id ASC").Find(&albums)
	out := []map[string]any{}
	for i := range albums {
		a := &albums[i]
		var cnt int64
		s.DB.Model(&models.GalleryPhoto{}).Where("album_id = ?", a.ID).Count(&cnt)
		var cover models.GalleryPhoto
		var coverURL any
		if s.DB.Where("album_id = ?", a.ID).Order("sort_order ASC, id ASC").First(&cover).Error == nil {
			coverURL = cover.URL
		}
		out = append(out, map[string]any{
			"id": a.ID, "title": a.Title, "description": a.Description,
			"is_home": a.IsHome, "photo_count": cnt, "cover": coverURL,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"albums": out})
}

// GET /gallery/albums/{id} — альбом с фотографиями. Право gallery.view.
func (s *Server) getGalleryAlbum(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var a models.GalleryAlbum
	if err := s.DB.First(&a, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Альбом не найден")
		return
	}
	var photos []models.GalleryPhoto
	s.DB.Where("album_id = ?", id).Order("sort_order ASC, id ASC").Find(&photos)
	resp := map[string]any{
		"id": a.ID, "title": a.Title, "description": a.Description,
		"is_home": a.IsHome, "photos": photos,
	}
	if a.IsHome {
		resp["bw"] = s.getIntSetting("gallery_home_bw", 0) != 0
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /gallery/albums — создать альбом. Право gallery.manage.
func (s *Server) createGalleryAlbum(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Title       string  `json:"title"`
		Description *string `json:"description"`
	}
	if err := decodeJSON(r, &p); err != nil || strings.TrimSpace(p.Title) == "" {
		httpErr(w, http.StatusBadRequest, "Укажите название альбома")
		return
	}
	var maxOrder int
	s.DB.Model(&models.GalleryAlbum{}).Select("COALESCE(MAX(sort_order),0)").Scan(&maxOrder)
	a := models.GalleryAlbum{Title: truncRunes(strings.TrimSpace(p.Title), 255), Description: p.Description, SortOrder: maxOrder + 1}
	if err := s.DB.Create(&a).Error; err != nil {
		httpErr(w, http.StatusBadRequest, "Не удалось создать альбом")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": a.ID})
}

// PATCH /gallery/albums/{id} — изменить альбом (название/описание; для «Главной» — ч/б-флаг). Право gallery.manage.
func (s *Server) updateGalleryAlbum(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var a models.GalleryAlbum
	if err := s.DB.First(&a, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Альбом не найден")
		return
	}
	var p map[string]any
	_ = decodeJSON(r, &p)
	upd := map[string]any{}
	if v, ok := p["title"]; ok {
		if t := truncRunes(strings.TrimSpace(anyStr(v)), 255); t != "" {
			upd["title"] = t
		}
	}
	if v, ok := p["description"]; ok {
		if d := strings.TrimSpace(anyStr(v)); d == "" {
			upd["description"] = nil
		} else {
			upd["description"] = d
		}
	}
	if len(upd) > 0 {
		upd["updated_at"] = gorm.Expr("now()")
		s.DB.Model(&models.GalleryAlbum{}).Where("id = ?", id).Updates(upd)
	}
	if a.IsHome {
		if v, ok := p["bw"]; ok {
			val := "0"
			if b, _ := v.(bool); b {
				val = "1"
			}
			s.setSetting("gallery_home_bw", val)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DELETE /gallery/albums/{id} — удалить альбом (кроме «Главной»). Право gallery.manage.
func (s *Server) deleteGalleryAlbum(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var a models.GalleryAlbum
	if err := s.DB.First(&a, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Альбом не найден")
		return
	}
	if a.IsHome {
		httpErr(w, http.StatusBadRequest, "Нельзя удалить альбом главной страницы")
		return
	}
	s.DB.Delete(&models.GalleryAlbum{}, id) // фото удалятся каскадом
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// POST /gallery/albums/{id}/photos — добавить фотографии (urls). Право gallery.manage.
func (s *Server) addGalleryPhotos(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var a models.GalleryAlbum
	if err := s.DB.First(&a, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Альбом не найден")
		return
	}
	var p struct {
		URLs []string `json:"urls"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	var maxOrder int
	s.DB.Model(&models.GalleryPhoto{}).Where("album_id = ?", id).Select("COALESCE(MAX(sort_order),0)").Scan(&maxOrder)
	for _, u := range p.URLs {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		maxOrder++
		s.DB.Create(&models.GalleryPhoto{AlbumID: id, URL: truncRunes(u, 500), SortOrder: maxOrder})
	}
	s.DB.Model(&models.GalleryAlbum{}).Where("id = ?", id).Update("updated_at", gorm.Expr("now()"))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DELETE /gallery/photos/{id} — удалить фото. Право gallery.manage.
func (s *Server) deleteGalleryPhoto(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	s.DB.Delete(&models.GalleryPhoto{}, id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GET /gallery/public/home — фото альбома «Главная» + флаг ч/б (для лендинга, без авторизации).
func (s *Server) publicHomeGallery(w http.ResponseWriter, r *http.Request) {
	var a models.GalleryAlbum
	if err := s.DB.Where("is_home").First(&a).Error; err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"photos": []any{}, "bw": false})
		return
	}
	var photos []models.GalleryPhoto
	s.DB.Where("album_id = ?", a.ID).Order("sort_order ASC, id ASC").Find(&photos)
	urls := []string{}
	for i := range photos {
		urls = append(urls, photos[i].URL)
	}
	writeJSON(w, http.StatusOK, map[string]any{"photos": urls, "bw": s.getIntSetting("gallery_home_bw", 0) != 0})
}
