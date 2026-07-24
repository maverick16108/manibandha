package web

import (
	"context"
	"net/http"
	"strings"

	"gorm.io/gorm"
)

// Мультиарендность (Ф4b): изоляция контента по пространствам через GORM-колбэки.
//
// Ключевое свойство безопасности: колбэки НИЧЕГО не делают, когда активное пространство — домашнее
// (Манибандха, id=1). Весь существующий контент имеет space_id=1, а middleware по умолчанию отдаёт 1,
// поэтому для Манибандхи поведение БД не меняется вообще. Скоупинг включается только когда пользователь
// зашёл в НЕдомашнее пространство (id>1) — такие пространства новые и пустые, риск для боевой Манибандхи нулевой.

// scopedTables — «контентные» таблицы арендатора (есть колонка space_id). Служебные таблицы пространств
// (spaces, space_members, space_features), app_settings и users сюда НЕ входят.
var scopedTables = map[string]bool{
	"events": true, "disciples": true, "threads": true, "forum_sections": true,
	"forum_topics": true, "conferences": true, "gallery_albums": true,
	"roles": true, "drafts": true, "temples": true,
}

// db — хэндл БД, несущий контекст запроса (активное пространство). Использовать в обработчиках,
// работающих со скоуп-таблицами, вместо s.DB напрямую.
func (s *Server) db(r *http.Request) *gorm.DB { return s.DB.WithContext(r.Context()) }

// platformHosts — домены самой платформы (не привязаны к конкретному пространству). Регистрация здесь
// создаёт «просто пользователя» (без анкеты ученика Манибандхи), который попадает в чаты.
var platformHosts = map[string]bool{"svistok.io": true, "www.svistok.io": true}

func requestHost(r *http.Request) string {
	host := r.Host
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return host
}

// isPlatformHost — запрос пришёл на домен платформы (svistok.io), а не на домен пространства.
func (s *Server) isPlatformHost(r *http.Request) bool { return platformHosts[requestHost(r)] }

func spaceFromCtx(ctx context.Context) (int, bool) {
	if ctx == nil {
		return 0, false
	}
	if v, ok := ctx.Value(spaceKey).(int); ok && v > 1 { // >1: домашнее (1) не скоупим
		return v, true
	}
	return 0, false
}

// scopedTableName — имя таблицы текущего запроса (парсит модель/назначение при необходимости).
func scopedTableName(d *gorm.DB) string {
	st := d.Statement
	if st.Table != "" {
		return st.Table
	}
	if st.Schema != nil {
		return st.Schema.Table
	}
	if st.Model != nil {
		if err := st.Parse(st.Model); err == nil && st.Schema != nil {
			return st.Schema.Table
		}
	}
	if st.Dest != nil {
		if err := st.Parse(st.Dest); err == nil && st.Schema != nil {
			return st.Schema.Table
		}
	}
	return ""
}

// applyWhereScope добавляет <table>.space_id = ? для скоуп-таблиц (чтение/обновление/удаление).
func applyWhereScope(d *gorm.DB) {
	id, ok := spaceFromCtx(d.Statement.Context)
	if !ok {
		return
	}
	table := scopedTableName(d)
	if table == "" || !scopedTables[table] {
		return
	}
	// квалифицируем колонку именем таблицы — на случай JOIN'ов (threads JOIN disciples и т.п.)
	d.Where(table+".space_id = ?", id)
}

// RegisterSpaceScoping навешивает колбэки чтения/обновления/удаления (фильтр по space_id)
// и вставки (простановка space_id). Все — no-op для домашнего пространства (id=1).
func RegisterSpaceScoping(db *gorm.DB) {
	_ = db.Callback().Query().Before("gorm:query").Register("space:query", applyWhereScope)
	_ = db.Callback().Update().Before("gorm:update").Register("space:update", applyWhereScope)
	_ = db.Callback().Delete().Before("gorm:delete").Register("space:delete", applyWhereScope)

	_ = db.Callback().Create().Before("gorm:create").Register("space:create", func(d *gorm.DB) {
		id, ok := spaceFromCtx(d.Statement.Context)
		if !ok {
			return
		}
		if d.Statement.Schema == nil {
			return
		}
		table := d.Statement.Schema.Table
		if !scopedTables[table] {
			return
		}
		if f := d.Statement.Schema.LookUpField("space_id"); f != nil {
			d.Statement.SetColumn("space_id", id)
		}
	})
}
