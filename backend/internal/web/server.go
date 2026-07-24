package web

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"manibandha/internal/caps"
	"manibandha/internal/config"
	"manibandha/internal/models"
	"manibandha/internal/security"
)

type Server struct {
	DB  *gorm.DB
	Cfg *config.Config
	JWT *security.JWT
}

type ctxKey int

const (
	userKey  ctxKey = 0
	spaceKey ctxKey = 1
)

// ── ответы в стиле FastAPI ──────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// httpErr — {"detail": "..."} как в FastAPI (фронт читает e.response.data.detail).
func httpErr(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}

// tsUTC форматирует момент в UTC с суффиксом Z (как pydantic):
// 6 знаков микросекунд, если они есть, иначе без дробной части.
func tsUTC(t time.Time) string {
	u := t.UTC()
	if u.Nanosecond() == 0 {
		return u.Format("2006-01-02T15:04:05Z07:00")
	}
	return u.Format("2006-01-02T15:04:05.000000Z07:00")
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(dst)
}

// ── аутентификация ──────────────────────────────────────────────────────────

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// auth — middleware: Bearer-токен → пользователь по email (sub), проверка is_active.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const msg = "Не удалось проверить учётные данные"
		tok := bearer(r)
		if tok == "" {
			httpErr(w, http.StatusUnauthorized, msg)
			return
		}
		claims, err := s.JWT.Parse(tok)
		if err != nil {
			httpErr(w, http.StatusUnauthorized, msg)
			return
		}
		email, _ := claims["sub"].(string)
		if email == "" {
			httpErr(w, http.StatusUnauthorized, msg)
			return
		}
		var u models.User
		if err := s.DB.Where("email = ?", email).First(&u).Error; err != nil || !u.IsActive {
			httpErr(w, http.StatusUnauthorized, msg)
			return
		}
		ctx := context.WithValue(r.Context(), userKey, &u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func currentUser(r *http.Request) *models.User {
	u, _ := r.Context().Value(userKey).(*models.User)
	return u
}

// requireCap — middleware-гейт по capability.
func (s *Server) requireCap(cap string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := currentUser(r)
			if u == nil || !caps.HasCap(s.DB, u.ID, cap) {
				httpErr(w, http.StatusForbidden, "Недостаточно прав")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// spaceCtx — определяет активное пространство запроса: заголовок X-Space-Id (если задан фронтом),
// иначе кастомный домен из Host, иначе домашнее пространство (Манибандха). Не требует авторизации.
func (s *Server) spaceCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := caps.HomeSpaceID
		if h := strings.TrimSpace(r.Header.Get("X-Space-Id")); h != "" {
			if n, err := strconv.Atoi(h); err == nil && n > 0 {
				id = n
			}
		} else {
			host := r.Host
			if i := strings.IndexByte(host, ':'); i >= 0 {
				host = host[:i]
			}
			host = strings.TrimPrefix(host, "www.")
			if host != "" {
				var sp models.Space
				if err := s.DB.Select("id").Where("custom_domain = ?", host).First(&sp).Error; err == nil {
					id = sp.ID
				}
			}
		}
		ctx := context.WithValue(r.Context(), spaceKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// activeSpaceID — активное пространство запроса (по умолчанию домашнее).
func activeSpaceID(r *http.Request) int {
	if v, ok := r.Context().Value(spaceKey).(int); ok && v > 0 {
		return v
	}
	return caps.HomeSpaceID
}

// requireModerator — гейт для модератора активного пространства (владелец или супер-админ).
func (s *Server) requireModerator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := currentUser(r)
		if u == nil || !caps.IsModerator(s.DB, u.ID, activeSpaceID(r)) {
			httpErr(w, http.StatusForbidden, "Только модератор пространства")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── токен ───────────────────────────────────────────────────────────────────

func (s *Server) tokenFor(u *models.User) (string, error) {
	days := s.getIntSetting("auth_expire_days", 30)
	return s.JWT.Create(u.Email, u.Role, days*1440)
}

func (s *Server) getStrSetting(key, def string) string {
	var st models.AppSetting
	if err := s.DB.Where("key = ?", key).First(&st).Error; err != nil {
		return def
	}
	return st.Value
}

func (s *Server) getIntSetting(key string, def int) int {
	var st models.AppSetting
	if err := s.DB.Where("key = ?", key).First(&st).Error; err != nil {
		return def
	}
	if n, err := strconv.Atoi(strings.TrimSpace(st.Value)); err == nil {
		return n
	}
	return def
}

// ── SMS ─────────────────────────────────────────────────────────────────────

// normalizePhone → 7XXXXXXXXXX (11 цифр, РФ) — как core/sms.py.
func normalizePhone(raw string) string {
	var d strings.Builder
	for _, ch := range raw {
		if ch >= '0' && ch <= '9' {
			d.WriteRune(ch)
		}
	}
	s := d.String()
	if len(s) == 11 && s[0] == '8' {
		s = "7" + s[1:]
	}
	if len(s) == 10 {
		s = "7" + s
	}
	return s
}

func (s *Server) sendSMS(phone, message string) {
	if !s.Cfg.SMSCEnabled || s.Cfg.SMSCLogin == "" {
		log.Printf("[sms] %s: %s", phone, message)
		return
	}
	q := url.Values{}
	q.Set("login", s.Cfg.SMSCLogin)
	q.Set("psw", s.Cfg.SMSCPassword)
	q.Set("phones", phone)
	q.Set("mes", message)
	q.Set("fmt", "3")
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Get("https://smsc.ru/sys/send.php?" + q.Encode())
	if err != nil {
		log.Printf("[sms] send error: %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	log.Printf("[sms] sent to %s: %s", phone, string(body))
}
