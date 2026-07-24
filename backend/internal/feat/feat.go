// Package feat — модули (фичи) пространства: каталог, дефолты по типу пространства и
// эффективное состояние (дефолт, переопределённый строками space_features).
package feat

import (
	"sort"

	"gorm.io/gorm"

	"manibandha/internal/models"
)

// Feature — переключаемый модуль пространства.
type Feature struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// Catalog — порядок отображения модулей в управлении пространством.
var Catalog = []Feature{
	{"dashboard", "Обзор"},
	{"calendar", "События"},
	{"disciples", "Ученики"},
	{"questions", "Вопросы"},
	{"reports", "Отчёты о служении"},
	{"forum", "Форум"},
	{"conference", "Конференция"},
	{"gallery", "Галерея"},
}

var known = func() map[string]bool {
	m := map[string]bool{}
	for _, f := range Catalog {
		m[f.Key] = true
	}
	return m
}()

// defaultsByType — какие модули включены по умолчанию для типа пространства.
// «guru» (Манибандха) — все. «obuchenie» (Курсы, Ф5) — свой набор.
var defaultsByType = map[string]map[string]bool{
	"guru": {
		"dashboard": true, "calendar": true, "disciples": true, "questions": true,
		"reports": true, "forum": true, "conference": true, "gallery": true,
	},
	"obuchenie": {
		"dashboard": false, "calendar": true, "disciples": false, "questions": true,
		"reports": false, "forum": true, "conference": true, "gallery": true,
	},
}

// DefaultsFor — карта дефолтов для типа пространства (неизвестный тип → всё включено).
func DefaultsFor(spaceType string) map[string]bool {
	if d, ok := defaultsByType[spaceType]; ok {
		out := make(map[string]bool, len(d))
		for k, v := range d {
			out[k] = v
		}
		return out
	}
	out := map[string]bool{}
	for _, f := range Catalog {
		out[f.Key] = true
	}
	return out
}

// EnabledFeatures — эффективное состояние модулей пространства: дефолт по типу, переопределённый
// явными строками space_features.
func EnabledFeatures(db *gorm.DB, spaceID int) map[string]bool {
	var sp models.Space
	spaceType := "guru"
	if err := db.Select("type").First(&sp, spaceID).Error; err == nil && sp.Type != "" {
		spaceType = sp.Type
	}
	eff := DefaultsFor(spaceType)
	var rows []models.SpaceFeature
	db.Where("space_id = ?", spaceID).Find(&rows)
	for _, r := range rows {
		if known[r.Feature] {
			eff[r.Feature] = r.Enabled
		}
	}
	return eff
}

// EnabledList — отсортированный список включённых модулей (для фронта/навигации).
func EnabledList(db *gorm.DB, spaceID int) []string {
	eff := EnabledFeatures(db, spaceID)
	out := make([]string, 0, len(eff))
	for k, on := range eff {
		if on {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// IsEnabled — включён ли конкретный модуль в пространстве.
func IsEnabled(db *gorm.DB, spaceID int, feature string) bool {
	if !known[feature] {
		return true // не-модульные разделы (чат, управление) не гейтятся фичами
	}
	return EnabledFeatures(db, spaceID)[feature]
}

// IsKnown — есть ли такой модуль в каталоге.
func IsKnown(feature string) bool { return known[feature] }

// Set — включить/выключить модуль (upsert строки space_features).
func Set(db *gorm.DB, spaceID int, feature string, enabled bool) error {
	row := models.SpaceFeature{SpaceID: spaceID, Feature: feature, Enabled: enabled}
	return db.Save(&row).Error
}
