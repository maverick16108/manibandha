package web

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"manibandha/internal/caps"
	"manibandha/internal/models"
)

// ── сериализация ────────────────────────────────────────────────────────────

func dateStr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format("2006-01-02")
}

func mentorBrief(m *models.Disciple) any {
	if m == nil {
		return nil
	}
	return map[string]any{"id": m.ID, "name": m.Name()}
}

func templeOrNil(t *models.Temple) any {
	if t == nil {
		return nil
	}
	return t
}

func checklistOut(items []models.ChecklistItem) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		c := &items[i]
		out = append(out, map[string]any{
			"id": c.ID, "disciple_id": c.DiscipleID, "title": c.Title,
			"is_done": c.IsDone, "note": c.Note, "target": c.Target,
		})
	}
	return out
}

func discipleOut(d *models.Disciple) map[string]any {
	return map[string]any{
		"id":                   d.ID,
		"spiritual_name":       d.SpiritualName,
		"material_name":        d.MaterialName,
		"photo_url":            d.PhotoURL,
		"phone":                d.Phone,
		"email":                d.Email,
		"messenger":            d.Messenger,
		"country":              d.Country,
		"region":               d.Region,
		"city":                 d.City,
		"temple_id":            d.TempleID,
		"gender":               d.Gender,
		"marital_status":       d.MaritalStatus,
		"date_of_birth":        dateStr(d.DateOfBirth),
		"initiation_status":    d.InitiationStatus,
		"pranama_date":         dateStr(d.PranamaDate),
		"harinama_date":        dateStr(d.HarinamaDate),
		"harinama_name":        d.HarinamaName,
		"brahman_date":         dateStr(d.BrahmanDate),
		"seva":                 d.Seva,
		"current_activity":     d.CurrentActivity,
		"mentor_id":            d.MentorID,
		"mentor_name":          d.MentorName,
		"is_mentor":            d.IsMentor,
		"recommended_by":       d.RecommendedBy,
		"application_date":     dateStr(d.ApplicationDate),
		"ready_for_pranama":    d.ReadyForPranama,
		"ready_for_initiation": d.ReadyForInitiation,
		"notes":                d.Notes,
		"is_approved":          d.IsApproved,
		"created_at":           tsUTC(d.CreatedAt),
		"updated_at":           tsUTC(d.UpdatedAt),
		"temple":               templeOrNil(d.Temple),
		"mentor":               mentorBrief(d.Mentor),
		"checklist":            checklistOut(d.Checklist),
	}
}

func discipleListItem(d *models.Disciple) map[string]any {
	return map[string]any{
		"id":                d.ID,
		"spiritual_name":    d.SpiritualName,
		"material_name":     d.MaterialName,
		"photo_url":         d.PhotoURL,
		"phone":             d.Phone,
		"country":           d.Country,
		"region":            d.Region,
		"city":              d.City,
		"initiation_status": d.InitiationStatus,
		"is_mentor":         d.IsMentor,
		"is_approved":       d.IsApproved,
		"profile_filled":    d.ProfileFilled(),
		"pranama_date":      dateStr(d.PranamaDate),
		"harinama_date":     dateStr(d.HarinamaDate),
		"brahman_date":      dateStr(d.BrahmanDate),
		"created_at":        tsUTC(d.CreatedAt),
		"temple":            templeOrNil(d.Temple),
		"mentor":            mentorBrief(d.Mentor),
	}
}

// ── доступ по ролям ─────────────────────────────────────────────────────────

func scopeDisciples(q *gorm.DB, u *models.User) *gorm.DB {
	id := -1
	if u.DiscipleID != nil {
		id = *u.DiscipleID
	}
	switch u.Role {
	case "guru", "secretary":
		return q
	case "curator":
		return q.Where("mentor_id = ?", id)
	case "student":
		return q.Where("id = ?", id)
	}
	return q.Where("1 = 0")
}

// GET /disciples — список с фильтрами, скоупом и сортировкой.
func (s *Server) listDisciples(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	qp := r.URL.Query()

	apply := func(q *gorm.DB) *gorm.DB {
		q = scopeDisciples(q, u)
		if v := strings.TrimSpace(qp.Get("q")); v != "" {
			like := "%" + v + "%"
			q = q.Where("material_name ILIKE ? OR spiritual_name ILIKE ?", like, like)
		}
		if v := qp.Get("status"); v != "" {
			q = q.Where("initiation_status = ?", v)
		}
		if v := qp.Get("country"); v != "" {
			q = q.Where("country ILIKE ?", v)
		}
		if v := qp.Get("region"); v != "" {
			q = q.Where("region ILIKE ?", v)
		}
		if v := qp.Get("city"); v != "" {
			q = q.Where("city ILIKE ?", v)
		}
		if v := qp.Get("temple_id"); v != "" {
			q = q.Where("temple_id = ?", v)
		}
		if v := qp.Get("mentor_id"); v != "" {
			q = q.Where("mentor_id = ?", v)
		}
		if v := qp.Get("ready"); v != "" {
			q = q.Where("ready_for_initiation = ?", v == "true")
		}
		if v := qp.Get("ready_pranama"); v != "" {
			q = q.Where("ready_for_pranama = ?", v == "true")
		}
		if v := qp.Get("is_mentor"); v != "" {
			q = q.Where("is_mentor = ?", v == "true")
		}
		if qp.Get("named") == "true" {
			q = q.Where("(material_name IS NOT NULL AND material_name <> '') OR (spiritual_name IS NOT NULL AND spiritual_name <> '')")
		}
		if v := qp.Get("pending"); v != "" {
			q = q.Where("is_approved = ?", v != "true")
		}
		if v := qp.Get("event_month"); v != "" {
			if start, end, ok := monthRange(v); ok {
				q = q.Where(
					"(pranama_date >= ? AND pranama_date < ?) OR (harinama_date >= ? AND harinama_date < ?) OR (brahman_date >= ? AND brahman_date < ?)",
					start, end, start, end, start, end)
			}
		}
		return q
	}

	var total int64
	apply(s.DB.Model(&models.Disciple{})).Count(&total)

	order := map[string]string{
		"material_name":     "material_name",
		"spiritual_name":    "spiritual_name",
		"created_at":        "created_at DESC",
		"initiation_status": "initiation_status",
	}[qp.Get("sort")]
	if order == "" {
		order = "material_name"
	}

	skip, _ := strconv.Atoi(qp.Get("skip"))
	limit := 50
	if v, err := strconv.Atoi(qp.Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if limit > 500 {
		limit = 500
	}

	var items []models.Disciple
	apply(s.DB.Preload("Temple").Preload("Mentor")).Order(order).Offset(skip).Limit(limit).Find(&items)

	rows := make([]map[string]any, 0, len(items))
	for i := range items {
		rows = append(rows, discipleListItem(&items[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "items": rows})
}

func monthRange(s string) (time.Time, time.Time, bool) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, false
	}
	y, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || m < 1 || m > 12 {
		return time.Time{}, time.Time{}, false
	}
	start := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return start, end, true
}

func (s *Server) discipleFull() *gorm.DB {
	return s.DB.Preload("Temple").Preload("Mentor").Preload("Checklist")
}

func (s *Server) getScoped(u *models.User, id int) (*models.Disciple, bool) {
	var d models.Disciple
	if err := scopeDisciples(s.discipleFull(), u).Where("id = ?", id).First(&d).Error; err != nil {
		return nil, false
	}
	return &d, true
}

// доступ к карточке: view_all/note видят любого, иначе — по скоупу
func (s *Server) getViewable(u *models.User, id int) (*models.Disciple, bool) {
	if caps.HasCap(s.DB, u.ID, "disciples.view_all") || caps.HasCap(s.DB, u.ID, "disciples.note") {
		var d models.Disciple
		if err := s.discipleFull().First(&d, id).Error; err != nil {
			return nil, false
		}
		return &d, true
	}
	return s.getScoped(u, id)
}

// GET /disciples/{id}
func (s *Server) getDisciple(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	d, ok := s.getViewable(currentUser(r), id)
	if !ok {
		httpErr(w, http.StatusNotFound, "Ученик не найден")
		return
	}
	writeJSON(w, http.StatusOK, discipleOut(d))
}

// POST /disciples (staff)
func (s *Server) createDisciple(w http.ResponseWriter, r *http.Request) {
	var in discipleInput
	if err := decodeJSON(r, &in); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	d := models.Disciple{InitiationStatus: "aspirant", IsApproved: true}
	in.applyTo(&d)
	if err := s.DB.Create(&d).Error; err != nil {
		httpErr(w, http.StatusBadRequest, "Не удалось создать анкету")
		return
	}
	var full models.Disciple
	s.discipleFull().First(&full, d.ID)
	writeJSON(w, http.StatusCreated, discipleOut(&full))
}

// PATCH /disciples/{id}
func (s *Server) updateDisciple(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)

	var d models.Disciple
	switch u.Role {
	case "curator":
		if _, ok := s.getScoped(u, id); !ok {
			httpErr(w, http.StatusNotFound, "Ученик не найден")
			return
		}
	case "student":
		if u.DiscipleID == nil || *u.DiscipleID != id {
			httpErr(w, http.StatusForbidden, "Можно редактировать только свою анкету")
			return
		}
	case "guru", "secretary":
		// любой
	default:
		httpErr(w, http.StatusForbidden, "Недостаточно прав")
		return
	}
	if err := s.DB.First(&d, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Ученик не найден")
		return
	}

	var in discipleInput
	if err := decodeJSON(r, &in); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	upd := in.updateMap()
	if len(upd) > 0 {
		s.DB.Model(&models.Disciple{}).Where("id = ?", id).Updates(upd)
	}
	s.discipleFull().First(&d, id)

	// синхронизировать имя связанного пользователя
	name := d.MaterialName
	if d.SpiritualName != nil && *d.SpiritualName != "" {
		name = *d.SpiritualName
	}
	if name != "" {
		s.DB.Model(&models.User{}).Where("disciple_id = ?", id).Update("full_name", name)
	}
	writeJSON(w, http.StatusOK, discipleOut(&d))
}

// POST /disciples/{id}/approve
func (s *Server) approveDisciple(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var d models.Disciple
	if err := s.DB.First(&d, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Ученик не найден")
		return
	}
	upd := map[string]any{"is_approved": true}
	if d.InitiationStatus == "recommended" {
		upd["initiation_status"] = "aspirant"
	}
	s.DB.Model(&models.Disciple{}).Where("id = ?", id).Updates(upd)

	// выдать роль по умолчанию связанному пользователю
	var linked models.User
	if err := s.DB.Where("disciple_id = ?", id).First(&linked).Error; err == nil {
		var def models.Role
		if err := s.DB.Where("is_default = ?", true).First(&def).Error; err == nil {
			var cnt int64
			s.DB.Model(&models.UserRole{}).Where("user_id = ? AND role_id = ?", linked.ID, def.ID).Count(&cnt)
			if cnt == 0 {
				s.DB.Create(&models.UserRole{UserID: linked.ID, RoleID: def.ID})
			}
		}
	}
	s.discipleFull().First(&d, id)
	writeJSON(w, http.StatusOK, discipleOut(&d))
}

// DELETE /disciples/{id} (staff)
func (s *Server) deleteDisciple(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var d models.Disciple
	if err := s.DB.First(&d, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Ученик не найден")
		return
	}
	s.DB.Delete(&models.Disciple{}, id)
	w.WriteHeader(http.StatusNoContent)
}

// ── Заметки ─────────────────────────────────────────────────────────────────

func (s *Server) listNotes(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var cnt int64
	s.DB.Model(&models.Disciple{}).Where("id = ?", id).Count(&cnt)
	if cnt == 0 {
		httpErr(w, http.StatusNotFound, "Ученик не найден")
		return
	}
	var rows []models.DiscipleNote
	s.DB.Preload("Author").Where("disciple_id = ?", id).Order("created_at DESC").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		n := &rows[i]
		var an any
		if n.Author != nil {
			an = n.Author.FullName
		}
		out = append(out, map[string]any{
			"id": n.ID, "author_id": n.AuthorID, "author_name": an, "text": n.Text, "created_at": tsUTC(n.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) addNote(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var cnt int64
	s.DB.Model(&models.Disciple{}).Where("id = ?", id).Count(&cnt)
	if cnt == 0 {
		httpErr(w, http.StatusNotFound, "Ученик не найден")
		return
	}
	var p struct {
		Text string `json:"text"`
	}
	_ = decodeJSON(r, &p)
	text := strings.TrimSpace(p.Text)
	if text == "" {
		httpErr(w, http.StatusBadRequest, "Пустая заметка")
		return
	}
	n := models.DiscipleNote{DiscipleID: id, AuthorID: &u.ID, Text: text}
	s.DB.Create(&n)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": n.ID, "author_id": n.AuthorID, "author_name": u.FullName, "text": n.Text, "created_at": tsUTC(n.CreatedAt),
	})
}

func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	noteID, _ := strconv.Atoi(chi.URLParam(r, "noteId"))
	u := currentUser(r)
	var n models.DiscipleNote
	if err := s.DB.First(&n, noteID).Error; err != nil || n.DiscipleID != id {
		httpErr(w, http.StatusNotFound, "Заметка не найдена")
		return
	}
	if (n.AuthorID == nil || *n.AuthorID != u.ID) && !caps.HasCap(s.DB, u.ID, "disciples.edit") {
		httpErr(w, http.StatusForbidden, "Удалять можно свои заметки")
		return
	}
	s.DB.Delete(&models.DiscipleNote{}, noteID)
	w.WriteHeader(http.StatusNoContent)
}

// ── Файлы ───────────────────────────────────────────────────────────────────

const maxDiscipleFileBytes = 25 * 1024 * 1024

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	if _, ok := s.getViewable(currentUser(r), id); !ok {
		httpErr(w, http.StatusNotFound, "Ученик не найден")
		return
	}
	var rows []models.DiscipleFile
	s.DB.Preload("Uploader").Where("disciple_id = ?", id).Order("created_at DESC").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		f := &rows[i]
		var un any
		if f.Uploader != nil {
			un = f.Uploader.FullName
		}
		out = append(out, map[string]any{
			"id": f.ID, "name": f.Name, "url": f.URL, "size": f.Size,
			"content_type": f.ContentType, "uploaded_by_name": un, "created_at": tsUTC(f.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) uploadDiscipleFile(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var cnt int64
	s.DB.Model(&models.Disciple{}).Where("id = ?", id).Count(&cnt)
	if cnt == 0 {
		httpErr(w, http.StatusNotFound, "Ученик не найден")
		return
	}
	if err := r.ParseMultipartForm(maxDiscipleFileBytes + 1024); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректная форма")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		httpErr(w, http.StatusBadRequest, "Файл не передан")
		return
	}
	defer file.Close()
	data, _ := io.ReadAll(io.LimitReader(file, maxDiscipleFileBytes+1))
	if len(data) > maxDiscipleFileBytes {
		httpErr(w, http.StatusBadRequest, "Файл больше 25 МБ")
		return
	}
	orig := hdr.Filename
	if orig == "" {
		orig = "file"
	}
	ext := filepath.Ext(orig)
	if len(ext) > 12 {
		ext = ext[:12]
	}
	_ = os.MkdirAll(s.Cfg.UploadDir, 0o755)
	name := randHex() + ext
	if err := os.WriteFile(filepath.Join(s.Cfg.UploadDir, name), data, 0o644); err != nil {
		httpErr(w, http.StatusInternalServerError, "Не удалось сохранить файл")
		return
	}
	ct := strings.Split(hdr.Header.Get("Content-Type"), ";")[0]
	var ctp *string
	if ct != "" {
		if len(ct) > 120 {
			ct = ct[:120]
		}
		ctp = &ct
	}
	if len(orig) > 255 {
		orig = orig[:255]
	}
	sz := len(data)
	rec := models.DiscipleFile{
		DiscipleID: id, UploadedBy: &u.ID, Name: orig,
		URL: "/uploads/" + name, Size: &sz, ContentType: ctp,
	}
	s.DB.Create(&rec)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": rec.ID, "name": rec.Name, "url": rec.URL, "size": rec.Size,
		"content_type": rec.ContentType, "uploaded_by_name": u.FullName, "created_at": tsUTC(rec.CreatedAt),
	})
}

func (s *Server) deleteDiscipleFile(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	fileID, _ := strconv.Atoi(chi.URLParam(r, "fileId"))
	var rec models.DiscipleFile
	if err := s.DB.First(&rec, fileID).Error; err != nil || rec.DiscipleID != id {
		httpErr(w, http.StatusNotFound, "Файл не найден")
		return
	}
	s.DB.Delete(&models.DiscipleFile{}, fileID)
	w.WriteHeader(http.StatusNoContent)
}
