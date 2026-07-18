package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"manibandha/internal/caps"
	"manibandha/internal/models"
	"manibandha/internal/security"
)

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "app": s.Cfg.AppName})
}

// POST /auth/login — форма OAuth2 (username=email, password).
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	email := r.FormValue("username")
	password := r.FormValue("password")
	var u models.User
	if err := s.DB.Where("email = ?", email).First(&u).Error; err != nil {
		httpErr(w, http.StatusUnauthorized, "Неверный email или пароль")
		return
	}
	if !security.VerifyPassword(password, u.HashedPassword) {
		httpErr(w, http.StatusUnauthorized, "Неверный email или пароль")
		return
	}
	if !u.IsActive {
		httpErr(w, http.StatusForbidden, "Учётная запись отключена")
		return
	}
	s.issueToken(w, &u)
}

func (s *Server) issueToken(w http.ResponseWriter, u *models.User) {
	tok, err := s.tokenFor(u)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "Не удалось создать токен")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"access_token": tok, "token_type": "bearer"})
}

// POST /auth/phone/request — отправить SMS-код.
func (s *Server) phoneRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Phone   string `json:"phone"`
		Purpose string `json:"purpose"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	if body.Purpose == "" {
		body.Purpose = "auto"
	}
	ph := normalizePhone(body.Phone)
	if len(ph) != 11 {
		httpErr(w, http.StatusBadRequest, "Некорректный номер телефона")
		return
	}
	var cnt int64
	s.DB.Model(&models.User{}).Where("phone = ?", ph).Count(&cnt)
	exists := cnt > 0
	if body.Purpose == "register" && exists {
		httpErr(w, http.StatusBadRequest, "Этот номер уже зарегистрирован — перейдите на «Вход».")
		return
	}
	if body.Purpose == "login" && !exists {
		httpErr(w, http.StatusBadRequest, "Этот номер не зарегистрирован — перейдите на «Регистрация».")
		return
	}
	code := fmt.Sprintf("%04d", randomInt(10000))
	s.DB.Where("phone = ?", ph).Delete(&models.SmsCode{})
	s.DB.Create(&models.SmsCode{
		Phone:     ph,
		Code:      code,
		ExpiresAt: time.Now().Add(time.Duration(s.Cfg.SMSCodeTTLSec) * time.Second),
	})
	s.sendSMS(ph, fmt.Sprintf("Код для входа на manibandha.ru: %s", code))
	writeJSON(w, http.StatusOK, map[string]any{"sent": true, "exists": exists})
}

// POST /auth/phone/verify — проверка кода: вход или регистрация.
func (s *Server) phoneVerify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	ph := normalizePhone(body.Phone)
	var rec models.SmsCode
	if err := s.DB.Where("phone = ?", ph).Order("id DESC").First(&rec).Error; err != nil {
		httpErr(w, http.StatusBadRequest, "Код истёк, запросите новый")
		return
	}
	if rec.ExpiresAt.Before(time.Now()) {
		httpErr(w, http.StatusBadRequest, "Код истёк, запросите новый")
		return
	}
	rec.Attempts++
	if rec.Attempts > 5 {
		s.DB.Where("phone = ?", ph).Delete(&models.SmsCode{})
		httpErr(w, http.StatusBadRequest, "Слишком много попыток, запросите новый код")
		return
	}
	if rec.Code != strings.TrimSpace(body.Code) {
		s.DB.Model(&rec).Update("attempts", rec.Attempts)
		httpErr(w, http.StatusBadRequest, "Неверный код")
		return
	}

	s.DB.Where("phone = ?", ph).Delete(&models.SmsCode{})

	var u models.User
	if err := s.DB.Where("phone = ?", ph).First(&u).Error; err == nil {
		s.issueToken(w, &u)
		return
	}

	// регистрация: анкета + пользователь + approval-ветка (одной транзакцией).
	newUser, err := s.registerByPhone(ph)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "Не удалось зарегистрировать")
		return
	}
	s.issueToken(w, newUser)
}

func (s *Server) registerByPhone(ph string) (*models.User, error) {
	var out models.User
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		phonePlus := "+" + ph
		d := models.Disciple{
			MaterialName:     "",
			Phone:            &phonePlus,
			InitiationStatus: "recommended",
			IsApproved:       false,
		}
		if err := tx.Create(&d).Error; err != nil {
			return err
		}
		u := models.User{
			Email:          ph + "@phone.local",
			Phone:          &ph,
			HashedPassword: security.HashPassword(security.RandToken(16)),
			FullName:       "+" + ph,
			Role:           "student",
			IsActive:       true,
			DiscipleID:     &d.ID,
		}
		if err := tx.Create(&u).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.Thread{Kind: "approval", DiscipleID: d.ID}).Error; err != nil {
			return err
		}
		out = u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// POST /auth/refresh — продлить сессию.
func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	s.issueToken(w, currentUser(r))
}

// GET /auth/me
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}

// PATCH /auth/me — обновить имя/аватар.
func (s *Server) patchMe(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var body struct {
		FullName  *string `json:"full_name"`
		AvatarURL *string `json:"avatar_url"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	upd := map[string]any{}
	if body.FullName != nil {
		upd["full_name"] = *body.FullName
	}
	if body.AvatarURL != nil {
		upd["avatar_url"] = *body.AvatarURL
	}
	if len(upd) > 0 {
		s.DB.Model(&models.User{}).Where("id = ?", u.ID).Updates(upd)
		s.DB.First(u, u.ID)
	}
	writeJSON(w, http.StatusOK, u)
}

// GET /me/capabilities — права/роли текущего пользователя (для навигации на фронте).
func (s *Server) myCapabilities(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	pending := false
	if u.DiscipleID != nil {
		var d models.Disciple
		if err := s.DB.First(&d, *u.DiscipleID).Error; err == nil {
			pending = !d.IsApproved
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"capabilities": caps.UserCapabilities(s.DB, u.ID),
		"roles":        caps.RoleKeys(s.DB, u.ID),
		"pending":      pending,
		"disciple_id":  u.DiscipleID,
	})
}

// GET /capabilities — каталог прав (для редактора ролей).
func (s *Server) listCapabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, caps.Grouped())
}
