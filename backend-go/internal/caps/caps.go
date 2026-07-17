package caps

import (
	"sort"

	"gorm.io/gorm"

	"manibandha/internal/models"
)

// Каталог прав-действий (key, label, group) — из app/core/capabilities.py.
type Capability struct {
	Key   string
	Label string
	Group string
}

var Catalog = []Capability{
	{"dashboard.view", "Смотреть обзор", "Обзор"},
	{"calendar.view", "Смотреть календарь", "Календарь"},
	{"calendar.manage", "Управлять событиями", "Календарь"},
	{"disciples.view_all", "Видеть всех учеников", "Ученики"},
	{"disciples.view_own", "Видеть своих учеников", "Ученики"},
	{"disciples.create", "Добавлять учеников", "Ученики"},
	{"disciples.edit", "Редактировать анкеты", "Ученики"},
	{"disciples.delete", "Удалять учеников", "Ученики"},
	{"disciples.approve", "Апрувить регистрации", "Ученики"},
	{"disciples.note", "Делать заметки об учениках", "Ученики"},
	{"questions.ask", "Задавать вопросы", "Вопросы"},
	{"questions.answer", "Отвечать на вопросы", "Вопросы"},
	{"questions.view_all", "Видеть все вопросы", "Вопросы"},
	{"reports.write", "Писать отчёты", "Отчёты о служении"},
	{"reports.read_all", "Читать отчёты учеников", "Отчёты о служении"},
	{"reports.like", "Ставить лайки отчётам", "Отчёты о служении"},
	{"forum.view", "Читать форум", "Форум"},
	{"forum.post", "Писать на форуме", "Форум"},
	{"forum.moderate", "Модерировать форум", "Форум"},
	{"conference.view", "Участвовать в конференциях", "Конференция"},
	{"conference.host", "Проводить конференции", "Конференция"},
	{"dictionaries.manage", "Управлять справочниками", "Справочники"},
	{"users.manage", "Управлять пользователями", "Пользователи"},
	{"roles.manage", "Управлять ролями", "Роли"},
	{"settings.manage", "Управлять настройками", "Настройки"},
}

var allCaps []string
var capSet = map[string]bool{}

func init() {
	for _, c := range Catalog {
		allCaps = append(allCaps, c.Key)
		capSet[c.Key] = true
	}
}

func AllCaps() []string { return append([]string(nil), allCaps...) }

// Grouped — каталог для редактора ролей, сгруппированный по фичам (сохраняет порядок).
func Grouped() []map[string]any {
	var order []string
	items := map[string][]map[string]string{}
	for _, c := range Catalog {
		if _, ok := items[c.Group]; !ok {
			order = append(order, c.Group)
		}
		items[c.Group] = append(items[c.Group], map[string]string{"key": c.Key, "label": c.Label})
	}
	out := make([]map[string]any, 0, len(order))
	for _, g := range order {
		out = append(out, map[string]any{"group": g, "items": items[g]})
	}
	return out
}

// UserRoles — роли пользователя.
func UserRoles(db *gorm.DB, userID int) []models.Role {
	var roles []models.Role
	db.Joins("JOIN user_roles ur ON ur.role_id = roles.id").
		Where("ur.user_id = ?", userID).Find(&roles)
	return roles
}

// UserCapabilities — объединение прав всех ролей (superadmin → все).
func UserCapabilities(db *gorm.DB, userID int) []string {
	roles := UserRoles(db, userID)
	for _, r := range roles {
		if r.IsSuperadmin {
			out := AllCaps()
			sort.Strings(out)
			return out
		}
	}
	set := map[string]bool{}
	for _, r := range roles {
		for _, c := range r.Capabilities {
			if capSet[c] {
				set[c] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func HasCap(db *gorm.DB, userID int, cap string) bool {
	for _, c := range UserCapabilities(db, userID) {
		if c == cap {
			return true
		}
	}
	return false
}

func RoleKeys(db *gorm.DB, userID int) []string {
	roles := UserRoles(db, userID)
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		out = append(out, r.Key)
	}
	return out
}
