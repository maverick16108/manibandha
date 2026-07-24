package web

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"manibandha/internal/models"
)

//go:embed fonts/DejaVuSans.ttf
var dejavuTTF []byte

var reportStatusLabels = map[string]string{
	"aspirant":    "Кандидат",
	"pranama":     "Пранама-мантра",
	"recommended": "Зарегистрирован",
	"harinama":    "Харинама",
	"brahman":     "Брахман",
}

func statusLabel(s string) string {
	if l, ok := reportStatusLabels[s]; ok {
		return l
	}
	return s
}

// applyReportFilters — те же фильтры, что в reports.py (без status при group по status передаётся отдельно).
func applyReportFilters(q *gorm.DB, qp map[string]string) *gorm.DB {
	if v := qp["status"]; v != "" {
		q = q.Where("initiation_status = ?", v)
	}
	if v := qp["country"]; v != "" {
		q = q.Where("country ILIKE ?", v)
	}
	if v := qp["region"]; v != "" {
		q = q.Where("region ILIKE ?", v)
	}
	if v := qp["city"]; v != "" {
		q = q.Where("city ILIKE ?", v)
	}
	if v := qp["temple_id"]; v != "" {
		q = q.Where("temple_id = ?", v)
	}
	if v := qp["mentor_id"]; v != "" {
		q = q.Where("mentor_id = ?", v)
	}
	if v := qp["ready"]; v != "" {
		q = q.Where("ready_for_initiation = ?", v == "true")
	}
	if v := strings.TrimSpace(qp["q"]); v != "" {
		like := "%" + v + "%"
		q = q.Where("material_name ILIKE ? OR spiritual_name ILIKE ?", like, like)
	}
	return q
}

// GET /reports/summary
func (s *Server) reportSummary(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	base := func() *gorm.DB { return scopeDisciples(s.db(r).Model(&models.Disciple{}), u) }

	var total int64
	base().Count(&total)

	type sc struct {
		K string
		C int64
	}
	var statusRows []sc
	base().Select("initiation_status as k, count(*) as c").Group("initiation_status").Scan(&statusRows)
	byStatus := make([]map[string]any, 0, len(statusRows))
	for _, x := range statusRows {
		byStatus = append(byStatus, map[string]any{"key": statusLabel(x.K), "count": x.C})
	}

	type cc struct {
		K *string
		C int64
	}
	var countryRows []cc
	base().Select("country as k, count(*) as c").Group("country").Order("count(*) DESC").Scan(&countryRows)
	byCountry := make([]map[string]any, 0, len(countryRows))
	for _, x := range countryRows {
		byCountry = append(byCountry, map[string]any{"key": strOrDash(x.K), "count": x.C})
	}

	type tc struct {
		K *int
		C int64
	}
	var templeRows []tc
	base().Select("temple_id as k, count(*) as c").Group("temple_id").Scan(&templeRows)
	tnames := s.templeNames(r)
	byTemple := make([]map[string]any, 0, len(templeRows))
	for _, x := range templeRows {
		name := "—"
		if x.K != nil {
			if n, ok := tnames[*x.K]; ok {
				name = n
			}
		}
		byTemple = append(byTemple, map[string]any{"key": name, "count": x.C})
	}

	var ready, readyPr int64
	base().Where("ready_for_initiation = ?", true).Count(&ready)
	base().Where("ready_for_pranama = ?", true).Count(&readyPr)

	writeJSON(w, http.StatusOK, map[string]any{
		"total": total, "by_status": byStatus, "by_country": byCountry, "by_temple": byTemple,
		"ready_for_pranama": readyPr, "ready_for_initiation": ready,
	})
}

func strOrDash(p *string) string {
	if p == nil || *p == "" {
		return "—"
	}
	return *p
}

func (s *Server) templeNames(r *http.Request) map[int]string {
	var temples []models.Temple
	s.db(r).Find(&temples)
	m := map[int]string{}
	for _, t := range temples {
		m[t.ID] = t.Name
	}
	return m
}

func (s *Server) discipleNames(r *http.Request) map[int]string {
	var ds []models.Disciple
	s.db(r).Select("id, spiritual_name, material_name").Find(&ds)
	m := map[int]string{}
	for i := range ds {
		m[ds[i].ID] = ds[i].Name()
	}
	return m
}

// GET /reports/timeline
func (s *Server) reportTimeline(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	type dts struct {
		PranamaDate  *time.Time
		HarinamaDate *time.Time
		BrahmanDate  *time.Time
	}
	var rows []dts
	scopeDisciples(s.db(r).Model(&models.Disciple{}), u).
		Select("pranama_date, harinama_date, brahman_date").Scan(&rows)

	buckets := map[string]map[string]int{}
	add := func(t *time.Time, kind string) {
		if t == nil {
			return
		}
		key := t.Format("2006-01")
		if buckets[key] == nil {
			buckets[key] = map[string]int{"pranama": 0, "harinama": 0, "brahman": 0}
		}
		buckets[key][kind]++
	}
	for _, x := range rows {
		add(x.PranamaDate, "pranama")
		add(x.HarinamaDate, "harinama")
		add(x.BrahmanDate, "brahman")
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		b := buckets[k]
		out = append(out, map[string]any{"period": k, "pranama": b["pranama"], "harinama": b["harinama"], "brahman": b["brahman"]})
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /reports/group
func (s *Server) reportGroup(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	qp := queryMap(r)
	groupBy := qp["group_by"]
	if groupBy == "" {
		groupBy = "status"
	}
	col := map[string]string{
		"status": "initiation_status", "country": "country", "region": "region",
		"city": "city", "mentor": "mentor_id", "temple": "temple_id",
	}[groupBy]
	if col == "" {
		col = "temple_id"
	}

	q := scopeDisciples(s.db(r).Model(&models.Disciple{}), u)
	q = applyReportFilters(q, qp)
	type row struct {
		V *string
		C int64
	}
	var rows []row
	q.Select(col + " as v, count(*) as c").Group(col).Order("count(*) DESC").Scan(&rows)

	var label func(*string) string
	switch groupBy {
	case "status":
		label = func(v *string) string {
			if v == nil {
				return "—"
			}
			return statusLabel(*v)
		}
	case "country", "region", "city":
		label = func(v *string) string { return strOrDash(v) }
	case "mentor":
		names := s.discipleNames(r)
		label = func(v *string) string { return lookupName(v, names) }
	default: // temple
		names := s.templeNames(r)
		label = func(v *string) string { return lookupName(v, names) }
	}
	out := make([]map[string]any, 0, len(rows))
	for _, x := range rows {
		out = append(out, map[string]any{"key": label(x.V), "count": x.C})
	}
	writeJSON(w, http.StatusOK, out)
}

func lookupName(v *string, names map[int]string) string {
	if v == nil || *v == "" {
		return "—"
	}
	var id int
	fmt.Sscanf(*v, "%d", &id)
	if n, ok := names[id]; ok {
		return n
	}
	return "—"
}

func queryMap(r *http.Request) map[string]string {
	m := map[string]string{}
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			m[k] = v[0]
		}
	}
	return m
}

// ── экспорт ─────────────────────────────────────────────────────────────────

var reportColumns = []struct {
	Head string
	Val  func(d *models.Disciple) string
}{
	{"Духовное имя", func(d *models.Disciple) string { return deref(d.SpiritualName) }},
	{"Мирское имя", func(d *models.Disciple) string { return d.MaterialName }},
	{"Статус", func(d *models.Disciple) string { return statusLabel(d.InitiationStatus) }},
	{"Страна", func(d *models.Disciple) string { return deref(d.Country) }},
	{"Область", func(d *models.Disciple) string { return deref(d.Region) }},
	{"Город", func(d *models.Disciple) string { return deref(d.City) }},
	{"Храм", func(d *models.Disciple) string {
		if d.Temple != nil {
			return d.Temple.Name
		}
		return ""
	}},
	{"Куратор", func(d *models.Disciple) string {
		if d.Mentor != nil {
			return d.Mentor.Name()
		}
		return ""
	}},
	{"Телефон", func(d *models.Disciple) string { return deref(d.Phone) }},
	{"Email", func(d *models.Disciple) string { return deref(d.Email) }},
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *Server) reportRows(r *http.Request, u *models.User, qp map[string]string) []models.Disciple {
	q := scopeDisciples(s.db(r).Preload("Temple").Preload("Mentor"), u)
	q = applyReportFilters(q, qp)
	var rows []models.Disciple
	q.Order("material_name").Find(&rows)
	return rows
}

// GET /reports/disciples.xlsx
func (s *Server) exportXlsx(w http.ResponseWriter, r *http.Request) {
	rows := s.reportRows(r, currentUser(r), queryMap(r))
	f := excelize.NewFile()
	sheet := "Ученики"
	f.SetSheetName(f.GetSheetName(0), sheet)
	for i, c := range reportColumns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellStr(sheet, cell, c.Head)
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheet, col, col, 22)
	}
	for ri := range rows {
		for ci, c := range reportColumns {
			v := c.Val(&rows[ri])
			if v == "" {
				continue // пустые ячейки не пишем — как openpyxl (None)
			}
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
			f.SetCellStr(sheet, cell, v)
		}
	}
	var buf bytes.Buffer
	f.Write(&buf)
	fname := fmt.Sprintf("disciples_%s.xlsx", time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fname))
	w.Write(buf.Bytes())
}

// GET /reports/disciples.pdf
func (s *Server) exportPdf(w http.ResponseWriter, r *http.Request) {
	rows := s.reportRows(r, currentUser(r), queryMap(r))
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(12, 12, 12)
	pdf.AddPage()

	// Встроенный DejaVuSans (кириллица на любом хосте, без зависимости от системных шрифтов).
	font := "Helvetica"
	if len(dejavuTTF) > 0 {
		pdf.AddUTF8FontFromBytes("app", "", dejavuTTF)
		font = "app"
	}
	pdf.SetFont(font, "", 16)
	pdf.CellFormat(0, 8, "Список учеников — Е.М. Манибандха Прабху", "", 1, "L", false, 0, "")
	pdf.SetFont(font, "", 8)
	pdf.CellFormat(0, 5, fmt.Sprintf("Сформировано: %s · всего: %d", time.Now().Format("02.01.2006 15:04"), len(rows)), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	// ширины колонок (сумма ≈ 273 мм полезной ширины A4 landscape)
	widths := []float64{34, 34, 26, 24, 26, 24, 30, 30, 26, 30}
	pdf.SetFillColor(200, 116, 42)
	pdf.SetTextColor(255, 255, 255)
	for i, c := range reportColumns {
		pdf.CellFormat(widths[i], 7, c.Head, "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(0, 0, 0)
	fill := false
	for ri := range rows {
		if fill {
			pdf.SetFillColor(250, 246, 240)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		for i, c := range reportColumns {
			pdf.CellFormat(widths[i], 6, c.Val(&rows[ri]), "1", 0, "L", true, 0, "")
		}
		pdf.Ln(-1)
		fill = !fill
	}

	var buf bytes.Buffer
	pdf.Output(&buf)
	fname := fmt.Sprintf("disciples_%s.pdf", time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fname))
	w.Write(buf.Bytes())
}
