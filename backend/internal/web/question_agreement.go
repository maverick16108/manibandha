package web

import (
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm/clause"

	"manibandha/internal/caps"
	"manibandha/internal/models"
)

// GET /questions/agreement — соглашение для раздела «Вопросы»: текст, включено ли, подтверждено ли
// текущим пользователем, и может ли он управлять соглашением.
func (s *Server) getQuestionAgreement(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	enabled := s.getIntSetting("qagr_enabled", 1) != 0
	version := s.getIntSetting("qagr_version", 1)
	text := s.getStrSetting("qagr_text", "")
	var ack models.QuestionAgreementAck
	acknowledged := s.DB.Where("user_id = ?", u.ID).First(&ack).Error == nil && ack.Version >= version
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": enabled, "text": text, "version": version,
		"acknowledged": acknowledged,
		"can_manage":   caps.HasCapIn(s.DB, u.ID, "questions.agreement_manage", activeSpaceID(r)),
	})
}

// POST /questions/agreement/ack — пользователь подтвердил, что прочитал соглашение (текущая версия).
func (s *Server) ackQuestionAgreement(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	version := s.getIntSetting("qagr_version", 1)
	s.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"version"}),
	}).Create(&models.QuestionAgreementAck{UserID: u.ID, Version: version})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PUT /questions/agreement — изменить текст/включённость соглашения (право questions.agreement_manage).
// При изменении текста версия увеличивается — соглашение снова показывается всем.
func (s *Server) updateQuestionAgreement(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Enabled *bool   `json:"enabled"`
		Text    *string `json:"text"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	if p.Enabled != nil {
		val := "0"
		if *p.Enabled {
			val = "1"
		}
		s.setSetting("qagr_enabled", val)
	}
	if p.Text != nil {
		newText := strings.TrimSpace(*p.Text)
		if newText != strings.TrimSpace(s.getStrSetting("qagr_text", "")) {
			s.setSetting("qagr_text", newText)
			s.setSetting("qagr_version", strconv.Itoa(s.getIntSetting("qagr_version", 1)+1)) // все подтвердят заново
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": s.getIntSetting("qagr_enabled", 1) != 0,
		"text":    s.getStrSetting("qagr_text", ""),
		"version": s.getIntSetting("qagr_version", 1),
	})
}
