package web

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"manibandha/internal/caps"
	"manibandha/internal/models"
)

const codeAlphabet = "abcdefghijkmnpqrstuvwxyzACDEFGHJKLMNPQRSTUVWXYZ23456789"

func tsPtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return tsUTC(*t)
}

func (s *Server) isSuperadmin(userID int) bool {
	for _, r := range caps.UserRoles(s.DB, userID) {
		if r.IsSuperadmin {
			return true
		}
	}
	return false
}

func (s *Server) genCode() string {
	for i := 0; i < 20; i++ {
		var b [7]byte
		rand.Read(b[:])
		code := make([]byte, 7)
		for j := range code {
			code[j] = codeAlphabet[int(b[j])%len(codeAlphabet)]
		}
		cs := string(code)
		var cnt int64
		s.DB.Model(&models.Conference{}).Where("code = ?", cs).Count(&cnt)
		if cnt == 0 {
			return cs
		}
	}
	return randHex()[:10]
}

func (s *Server) lkConfigured() bool {
	return s.Cfg.LiveKitAPIKey != "" && s.Cfg.LiveKitAPISecret != ""
}

// ── LiveKit токены ───────────────────────────────────────────────────────────

func (s *Server) lkSign(claims jwt.MapClaims) string {
	str, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.Cfg.LiveKitAPISecret))
	return str
}

func (s *Server) adminToken(room string) string {
	now := time.Now().Unix()
	return s.lkSign(jwt.MapClaims{"iss": s.Cfg.LiveKitAPIKey, "nbf": now - 5, "exp": now + 120,
		"video": map[string]any{"room": room, "roomAdmin": true, "roomJoin": false}})
}

func (s *Server) egressToken() string {
	now := time.Now().Unix()
	return s.lkSign(jwt.MapClaims{"iss": s.Cfg.LiveKitAPIKey, "nbf": now - 5, "exp": now + 300,
		"video": map[string]any{"roomRecord": true}})
}

func (s *Server) mintToken(identity, name, room string, canPublish bool, sources []string) string {
	now := time.Now().Unix()
	video := map[string]any{"room": room, "roomJoin": true, "canPublish": canPublish, "canSubscribe": true, "canPublishData": true}
	if sources != nil {
		video["canPublishSources"] = sources
	}
	return s.lkSign(jwt.MapClaims{"iss": s.Cfg.LiveKitAPIKey, "sub": identity, "name": name,
		"nbf": now - 5, "exp": now + 6*3600, "video": video})
}

func (s *Server) twirp(service, method, bearer string, body map[string]any) (map[string]any, error) {
	return s.twirpTimeout(service, method, bearer, body, 10*time.Second)
}

// twirpTimeout — как twirp, но с настраиваемым таймаутом. Запуск egress (StartRoomCompositeEgress)
// на слабом сервере отвечает медленно (10-14с): при 10с клиент отваливался с «request canceled»,
// хотя egress уже стартовал → пользователь видел «Не удалось начать запись», а запись была осиротевшей.
func (s *Server) twirpTimeout(service, method, bearer string, body map[string]any, timeout time.Duration) (map[string]any, error) {
	url := s.Cfg.LiveKitAPIURL + "/twirp/livekit." + service + "/" + method
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if len(raw) > 0 {
		json.Unmarshal(raw, &out)
	}
	return out, nil
}

func (s *Server) roomService(method string, body map[string]any, room string) (map[string]any, error) {
	return s.twirp("RoomService", method, s.adminToken(room), body)
}

func (s *Server) egressService(method string, body map[string]any) (map[string]any, error) {
	return s.twirp("Egress", method, s.egressToken(), body)
}

// maybeAutoRecord — запускает запись, если у конференции включена авто-запись и активной записи ещё нет.
// Вызывается по webhook room_started (комната уже существует). Автор записи — ведущий.
func (s *Server) maybeAutoRecord(c *models.Conference) {
	if !c.AutoRecord || !s.recordingOn() {
		return
	}
	var cnt int64
	s.DB.Model(&models.ConferenceRecording{}).Where("conference_id = ? AND status IN ?", c.ID, []string{"active", "stopping"}).Count(&cnt)
	if cnt > 0 {
		return
	}
	fn := "conf" + strconv.Itoa(c.ID) + "-" + randHex()[:12] + ".mp4"
	if eid := s.startEgress(c.Room, fn, s.recHeight()); eid != "" {
		s.DB.Create(&models.ConferenceRecording{ConferenceID: c.ID, EgressID: &eid, Filename: &fn, Status: "active", StartedAt: time.Now(), CreatedBy: c.HostID})
	}
}

func (s *Server) closeRoom(room string) {
	if !s.lkConfigured() {
		return
	}
	s.roomService("DeleteRoom", map[string]any{"room": room}, room)
}

var recRes = map[int][2]int{480: {854, 1200}, 720: {1280, 2000}, 1080: {1920, 3500}}

func (s *Server) startEgress(room, filename string, height int) string {
	res, ok := recRes[height]
	if !ok {
		res = recRes[720]
	}
	body := map[string]any{
		"roomName": room, "layout": "grid",
		"fileOutputs": []map[string]any{{"fileType": "MP4", "filepath": "/out/" + filename}},
		"advanced":    map[string]any{"width": res[0], "height": height, "framerate": 20, "videoBitrate": res[1], "audioBitrate": 128},
	}
	// запуск egress отвечает медленно — даём до 40с, чтобы получить egressId и не осиротить запись
	info, err := s.twirpTimeout("Egress", "StartRoomCompositeEgress", s.egressToken(), body, 40*time.Second)
	if err != nil || info == nil {
		log.Printf("[egress] StartRoomCompositeEgress failed: err=%v info=%v", err, info)
		return ""
	}
	if v, ok := info["egressId"].(string); ok {
		return v
	}
	if v, ok := info["egress_id"].(string); ok {
		return v
	}
	return ""
}

func (s *Server) stopEgress(egressID string) {
	s.egressService("StopEgress", map[string]any{"egressId": egressID})
}

// stopEgressWhenReady — останавливает egress ТОЛЬКО после того, как он реально запустился (EGRESS_ACTIVE).
// Egress на этом сервере стартует ~20-30с (запуск chrome+gstreamer). Если нажать «стоп» раньше, LiveKit
// отвечает «Stop called before pipeline could start» и АБОРТИТ запись без файла — короткие записи терялись.
// Ждём активного состояния (до ~75с), даём ~2с контента и останавливаем корректно → получаем файл.
func (s *Server) stopEgressWhenReady(egressID string) {
	active := false
	for i := 0; i < 75; i++ {
		out, err := s.egressService("ListEgress", map[string]any{"egressId": egressID})
		if err == nil && out != nil {
			if items, ok := out["items"].([]any); ok && len(items) > 0 {
				if info, ok := items[0].(map[string]any); ok {
					switch anyStr(info["status"]) {
					case "EGRESS_ACTIVE":
						active = true
					case "EGRESS_COMPLETE", "EGRESS_FAILED", "EGRESS_ABORTED", "EGRESS_ENDING":
						return // уже завершается/завершился сам — трогать не нужно
					}
				}
			}
		}
		if active {
			break
		}
		time.Sleep(time.Second)
	}
	if active {
		time.Sleep(2 * time.Second) // гарантируем немного контента в файле
	}
	s.stopEgress(egressID)
}

// ── доступ ────────────────────────────────────────────────────────────────

func (s *Server) confEditable(userID int, c *models.Conference) bool {
	return (c.HostID != nil && *c.HostID == userID) || s.isSuperadmin(userID)
}

func (s *Server) validHost(hostID *int) *int {
	if hostID == nil || *hostID == 0 {
		return nil
	}
	var u models.User
	if err := s.DB.First(&u, *hostID).Error; err != nil || !u.IsActive {
		return nil
	}
	if caps.HasCap(s.DB, u.ID, "conference.host") {
		return &u.ID
	}
	return nil
}

func (s *Server) recordingOn() bool {
	return s.lkConfigured() && s.getIntSetting("recording_enabled", 1) != 0
}

func (s *Server) recHeight() int {
	h := s.getIntSetting("recording_height", 720)
	if h == 480 || h == 720 || h == 1080 {
		return h
	}
	return 720
}

// ── сериализация ──────────────────────────────────────────────────────────

func (s *Server) confOut(c *models.Conference, userID int, elevated bool, parts []map[string]any) map[string]any {
	var hn any
	if c.Host != nil {
		hn = c.Host.FullName
	}
	first := parts
	if first == nil {
		first = []map[string]any{}
	}
	if len(first) > 6 {
		first = first[:6]
	}
	pc := 0
	if parts != nil {
		pc = len(parts)
	}
	return map[string]any{
		"id": c.ID, "title": c.Title, "description": c.Description, "mode": c.Mode, "status": c.Status,
		"room": c.Room, "code": c.Code,
		"mic_allowed": c.MicAllowed, "cam_allowed": c.CamAllowed, "screen_allowed": c.ScreenAllowed,
		"guests_allowed": c.GuestsAllowed, "auto_record": c.AutoRecord,
		"host_id": c.HostID, "host_name": hn, "can_host": (c.HostID != nil && *c.HostID == userID) || elevated,
		"scheduled_at": tsPtr(c.ScheduledAt), "started_at": tsPtr(c.StartedAt), "ended_at": tsPtr(c.EndedAt),
		"created_at": tsUTC(c.CreatedAt), "participant_count": pc, "participants": first,
	}
}

func (s *Server) liveParticipants(c *models.Conference) []map[string]any {
	if !s.lkConfigured() {
		return nil
	}
	info, err := s.roomService("ListParticipants", map[string]any{"room": c.Room}, c.Room)
	if err != nil || info == nil {
		return nil
	}
	raw, _ := info["participants"].([]any)
	uids := []int{}
	for _, p := range raw {
		pm, _ := p.(map[string]any)
		ident, _ := pm["identity"].(string)
		if strings.HasPrefix(ident, "u") {
			if id, err := strconv.Atoi(ident[1:]); err == nil {
				uids = append(uids, id)
			}
		}
	}
	avatars := map[string]any{}
	if len(uids) > 0 {
		var us []models.User
		s.DB.Where("id IN ?", uids).Find(&us)
		for _, u := range us {
			avatars["u"+strconv.Itoa(u.ID)] = u.AvatarURL
		}
	}
	out := []map[string]any{}
	for _, p := range raw {
		pm, _ := p.(map[string]any)
		ident, _ := pm["identity"].(string)
		name, _ := pm["name"].(string)
		if name == "" {
			name = "Гость"
		}
		out = append(out, map[string]any{"name": name, "avatar_url": avatars[ident]})
	}
	return out
}

// ── эндпоинты ───────────────────────────────────────────────────────────────

func (s *Server) listConferences(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	elevated := s.isSuperadmin(u.ID)
	var rows []models.Conference
	s.DB.Preload("Host").
		Order("CASE WHEN status='live' THEN 0 WHEN status='scheduled' THEN 1 ELSE 2 END").
		Order("scheduled_at IS NULL").Order("scheduled_at ASC").Order("created_at DESC").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for i := range rows {
		var parts []map[string]any
		if rows[i].Status == "live" {
			parts = s.liveParticipants(&rows[i])
		}
		out = append(out, s.confOut(&rows[i], u.ID, elevated, parts))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createConference(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var p struct {
		Title         string     `json:"title"`
		Description    *string    `json:"description"`
		Mode          string     `json:"mode"`
		ScheduledAt   *time.Time `json:"scheduled_at"`
		MicAllowed    *bool      `json:"mic_allowed"`
		CamAllowed    *bool      `json:"cam_allowed"`
		ScreenAllowed *bool      `json:"screen_allowed"`
		GuestsAllowed *bool      `json:"guests_allowed"`
		AutoRecord    *bool      `json:"auto_record"`
		HostID        *int       `json:"host_id"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	title := strings.TrimSpace(p.Title)
	if title == "" {
		httpErr(w, http.StatusBadRequest, "Нужно название конференции")
		return
	}
	mode := "interactive"
	if p.Mode == "broadcast" {
		mode = "broadcast"
	}
	hostID := u.ID
	if vh := s.validHost(p.HostID); vh != nil {
		hostID = *vh
	}
	c := models.Conference{
		Title: truncRunes(title, 255), Mode: mode, Room: "conf_" + randHex()[:20], Status: "scheduled",
		HostID: &hostID, ScheduledAt: p.ScheduledAt,
		MicAllowed: boolOr(p.MicAllowed, false), CamAllowed: boolOr(p.CamAllowed, false),
		ScreenAllowed: boolOr(p.ScreenAllowed, false), GuestsAllowed: boolOr(p.GuestsAllowed, false),
		AutoRecord: boolOr(p.AutoRecord, false),
	}
	code := s.genCode()
	c.Code = &code
	if p.Description != nil {
		if d := strings.TrimSpace(*p.Description); d != "" {
			c.Description = &d
		}
	}
	s.DB.Create(&c)
	s.DB.Preload("Host").First(&c, c.ID)
	writeJSON(w, http.StatusCreated, s.confOut(&c, u.ID, true, nil))
}

func boolOr(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}

func (s *Server) resolveCode(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	var c models.Conference
	if err := s.DB.Where("code = ?", code).First(&c).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Конференция не найдена")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": c.ID, "room": c.Room, "guests_allowed": c.GuestsAllowed, "title": c.Title})
}

func (s *Server) listModerators(w http.ResponseWriter, r *http.Request) {
	var users []models.User
	s.DB.Where("is_active = ?", true).Order("full_name").Find(&users)
	out := []map[string]any{}
	for _, u := range users {
		if caps.HasCap(s.DB, u.ID, "conference.host") {
			out = append(out, map[string]any{"id": u.ID, "name": u.FullName})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"moderators": out})
}

func (s *Server) updateConference(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var c models.Conference
	if err := s.DB.First(&c, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Конференция не найдена")
		return
	}
	if !s.confEditable(u.ID, &c) {
		httpErr(w, http.StatusForbidden, "Управлять может только модератор конференции")
		return
	}
	var p struct {
		Title         *string    `json:"title"`
		Description   *string    `json:"description"`
		ScheduledAt   *time.Time `json:"scheduled_at"`
		Status        *string    `json:"status"`
		Mode          *string    `json:"mode"`
		MicAllowed    *bool      `json:"mic_allowed"`
		CamAllowed    *bool      `json:"cam_allowed"`
		ScreenAllowed *bool      `json:"screen_allowed"`
		GuestsAllowed *bool      `json:"guests_allowed"`
		AutoRecord    *bool      `json:"auto_record"`
		HostID        *int       `json:"host_id"`
	}
	if err := decodeJSON(r, &p); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	upd := map[string]any{}
	if p.Title != nil {
		if t := strings.TrimSpace(*p.Title); t != "" {
			upd["title"] = truncRunes(t, 255)
		}
	}
	if p.Description != nil {
		d := strings.TrimSpace(*p.Description)
		if d == "" {
			upd["description"] = nil
		} else {
			upd["description"] = d
		}
	}
	if p.ScheduledAt != nil {
		upd["scheduled_at"] = *p.ScheduledAt
	}
	if p.Mode != nil && (*p.Mode == "interactive" || *p.Mode == "broadcast") {
		upd["mode"] = *p.Mode
	}
	if p.MicAllowed != nil {
		upd["mic_allowed"] = *p.MicAllowed
	}
	if p.CamAllowed != nil {
		upd["cam_allowed"] = *p.CamAllowed
	}
	if p.ScreenAllowed != nil {
		upd["screen_allowed"] = *p.ScreenAllowed
	}
	if p.GuestsAllowed != nil {
		upd["guests_allowed"] = *p.GuestsAllowed
	}
	if p.AutoRecord != nil {
		upd["auto_record"] = *p.AutoRecord
	}
	if p.HostID != nil {
		if vh := s.validHost(p.HostID); vh != nil {
			upd["host_id"] = *vh
		}
	}
	ended := false
	if p.Status != nil && (*p.Status == "scheduled" || *p.Status == "live" || *p.Status == "ended") {
		upd["status"] = *p.Status
		if *p.Status == "live" && c.StartedAt == nil {
			upd["started_at"] = gorm.Expr("now()")
		}
		if *p.Status == "ended" {
			upd["ended_at"] = gorm.Expr("now()")
			ended = true
		}
	}
	if len(upd) > 0 {
		s.DB.Model(&models.Conference{}).Where("id = ?", id).Updates(upd)
		s.DB.Preload("Host").First(&c, id)
	}
	if ended {
		s.closeRoom(c.Room)
	}
	writeJSON(w, http.StatusOK, s.confOut(&c, u.ID, s.isSuperadmin(u.ID), nil))
}

func (s *Server) deleteConference(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var c models.Conference
	if err := s.DB.First(&c, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Конференция не найдена")
		return
	}
	if !s.confEditable(u.ID, &c) {
		httpErr(w, http.StatusForbidden, "Управлять может только модератор конференции")
		return
	}
	room := c.Room
	s.DB.Delete(&models.Conference{}, id)
	s.closeRoom(room)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) joinConference(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	if !(s.lkConfigured() && s.Cfg.LiveKitURL != "") {
		httpErr(w, http.StatusServiceUnavailable, "Видеосервер не настроен")
		return
	}
	var c models.Conference
	if err := s.DB.First(&c, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Конференция не найдена")
		return
	}
	isHost := (c.HostID != nil && *c.HostID == u.ID) || s.isSuperadmin(u.ID)
	identity := "u" + strconv.Itoa(u.ID)
	if !isHost {
		var cnt int64
		s.DB.Model(&models.ConferenceBan{}).Where("conference_id = ? AND identity = ?", c.ID, identity).Count(&cnt)
		if cnt > 0 {
			httpErr(w, http.StatusForbidden, "Вы удалены из этой встречи ведущим")
			return
		}
	}
	canPublish := isHost || c.Mode == "interactive"
	if c.Status == "ended" {
		upd := map[string]any{"status": "live", "ended_at": nil}
		if c.StartedAt == nil {
			upd["started_at"] = gorm.Expr("now()")
		}
		s.DB.Model(&models.Conference{}).Where("id = ?", id).Updates(upd)
		c.Status = "live"
	} else if isHost && c.Status == "scheduled" {
		s.DB.Model(&models.Conference{}).Where("id = ?", id).Updates(map[string]any{"status": "live", "started_at": gorm.Expr("now()")})
		c.Status = "live"
	}
	// авто-запись НЕ запускаем здесь (комната в LiveKit ещё не существует — хост только получил токен,
	// но не подключился → egress падает с «room does not exist»). Запускаем по webhook room_started.
	var sources []string
	if isHost {
		sources = nil
	} else {
		sources = []string{}
		if c.Mode != "broadcast" {
			if c.MicAllowed {
				sources = append(sources, "MICROPHONE")
			}
			if c.CamAllowed {
				sources = append(sources, "CAMERA")
			}
			if c.ScreenAllowed {
				sources = append(sources, "SCREEN_SHARE")
			}
		}
		canPublish = len(sources) > 0
	}
	token := s.mintToken(identity, orDefault(u.FullName, "Гость"), c.Room, canPublish, sources)
	writeJSON(w, http.StatusOK, s.joinOut(&c, token, canPublish, isHost, identity))
}

func (s *Server) joinOut(c *models.Conference, token string, canPublish, isHost bool, identity string) map[string]any {
	return map[string]any{
		"url": s.Cfg.LiveKitURL, "token": token, "room": c.Room, "code": c.Code, "mode": c.Mode, "title": c.Title,
		"can_publish": canPublish, "is_host": isHost, "identity": identity,
		"mic_allowed": c.MicAllowed, "cam_allowed": c.CamAllowed, "screen_allowed": c.ScreenAllowed,
	}
}

func (s *Server) guestJoin(w http.ResponseWriter, r *http.Request) {
	room := chi.URLParam(r, "room")
	if !(s.lkConfigured() && s.Cfg.LiveKitURL != "") {
		httpErr(w, http.StatusServiceUnavailable, "Видеосервер не настроен")
		return
	}
	var c models.Conference
	if err := s.DB.Where("room = ?", room).First(&c).Error; err != nil || !c.GuestsAllowed {
		httpErr(w, http.StatusNotFound, "Конференция недоступна для гостей")
		return
	}
	var p struct {
		Name string `json:"name"`
	}
	_ = decodeJSON(r, &p)
	name := truncRunes(strings.TrimSpace(p.Name), 60)
	if name == "" {
		name = "Гость"
	}
	if c.Status == "ended" {
		upd := map[string]any{"status": "live", "ended_at": nil}
		if c.StartedAt == nil {
			upd["started_at"] = gorm.Expr("now()")
		}
		s.DB.Model(&models.Conference{}).Where("id = ?", c.ID).Updates(upd)
	}
	var sources []string
	canPublish := false
	if c.Mode != "broadcast" {
		sources = []string{}
		if c.MicAllowed {
			sources = append(sources, "MICROPHONE")
		}
		if c.CamAllowed {
			sources = append(sources, "CAMERA")
		}
		if c.ScreenAllowed {
			sources = append(sources, "SCREEN_SHARE")
		}
		canPublish = len(sources) > 0
	} else {
		sources = []string{}
	}
	identity := "g_" + randHex()[:12]
	token := s.mintToken(identity, name, c.Room, canPublish, sources)
	writeJSON(w, http.StatusOK, s.joinOut(&c, token, canPublish, false, identity))
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// ── модерация публикации (разрешить/запретить микрофон/камеру/экран) ─────────

var permSrc = map[string][]string{"audio": {"MICROPHONE"}, "video": {"CAMERA"}, "screen": {"SCREEN_SHARE"}}
var permMuteSrc = map[string]string{"audio": "MICROPHONE", "video": "CAMERA", "screen": "SCREEN_SHARE"}

func (s *Server) applyPerm(room string, p map[string]any, kind string, allow bool) {
	perm, _ := p["permission"].(map[string]any)
	cur := map[string]bool{}
	if arr, ok := perm["canPublishSources"].([]any); ok {
		for _, x := range arr {
			cur[anyStr(x)] = true
		}
	}
	if len(cur) == 0 {
		if cp, _ := perm["canPublish"].(bool); cp {
			cur = map[string]bool{"CAMERA": true, "MICROPHONE": true, "SCREEN_SHARE": true}
		}
	}
	for _, src := range permSrc[kind] {
		if allow {
			cur[src] = true
		} else {
			delete(cur, src)
		}
	}
	var srcs []string
	for k := range cur {
		srcs = append(srcs, k)
	}
	sort.Strings(srcs)
	ident := anyStr(p["identity"])
	s.roomService("UpdateParticipant", map[string]any{
		"room": room, "identity": ident,
		"permission": map[string]any{"canPublish": len(srcs) > 0, "canPublishSources": srcs, "canSubscribe": true, "canPublishData": true},
	}, room)
	if !allow {
		src := permMuteSrc[kind]
		typ := "VIDEO"
		if kind == "audio" {
			typ = "AUDIO"
		}
		if tracks, ok := p["tracks"].([]any); ok {
			for _, t := range tracks {
				tm, _ := t.(map[string]any)
				ts := anyStr(tm["source"])
				if ts == src || ((ts == "" || ts == "UNKNOWN") && anyStr(tm["type"]) == typ) {
					s.roomService("MutePublishedTrack", map[string]any{"room": room, "identity": ident, "track_sid": tm["sid"], "muted": true}, room)
				}
			}
		}
	}
}

func (s *Server) moderatePermit(w http.ResponseWriter, r *http.Request) {
	c, ok := s.confByIDEditable(w, r)
	if !ok {
		return
	}
	var p struct {
		Kind     string `json:"kind"`
		Allow    bool   `json:"allow"`
		Identity string `json:"identity"`
		Except   string `json:"except"`
	}
	_ = decodeJSON(r, &p)
	if p.Kind != "audio" && p.Kind != "video" && p.Kind != "screen" {
		httpErr(w, http.StatusBadRequest, "kind: audio|video|screen")
		return
	}
	if p.Identity == "all" {
		col := map[string]string{"audio": "mic_allowed", "video": "cam_allowed", "screen": "screen_allowed"}[p.Kind]
		s.DB.Model(&models.Conference{}).Where("id = ?", c.ID).Update(col, p.Allow)
	}
	hostIdent := ""
	if c.HostID != nil {
		hostIdent = "u" + strconv.Itoa(*c.HostID)
	}
	info, err := s.roomService("ListParticipants", map[string]any{"room": c.Room}, c.Room)
	if err == nil && info != nil {
		parts, _ := info["participants"].([]any)
		for _, pp := range parts {
			pm, _ := pp.(map[string]any)
			ident := anyStr(pm["identity"])
			if ident == hostIdent {
				continue
			}
			if p.Identity != "all" && ident != p.Identity {
				continue
			}
			if p.Identity == "all" && p.Except != "" && ident == p.Except {
				s.applyPerm(c.Room, pm, p.Kind, true)
				continue
			}
			s.applyPerm(c.Room, pm, p.Kind, p.Allow)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ── баны/кик ──────────────────────────────────────────────────────────────

func (s *Server) bansOut(confID int) []map[string]any {
	var rows []models.ConferenceBan
	s.DB.Where("conference_id = ?", confID).Order("created_at DESC").Find(&rows)
	out := []map[string]any{}
	for _, b := range rows {
		name := "Участник"
		if b.Name != nil && *b.Name != "" {
			name = *b.Name
		}
		out = append(out, map[string]any{"identity": b.Identity, "name": name})
	}
	return out
}

func (s *Server) confByIDEditable(w http.ResponseWriter, r *http.Request) (*models.Conference, bool) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var c models.Conference
	if err := s.DB.First(&c, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Конференция не найдена")
		return nil, false
	}
	if !s.confEditable(u.ID, &c) {
		httpErr(w, http.StatusForbidden, "Управлять может только модератор конференции")
		return nil, false
	}
	return &c, true
}

func (s *Server) listBans(w http.ResponseWriter, r *http.Request) {
	c, ok := s.confByIDEditable(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bans": s.bansOut(c.ID)})
}

func (s *Server) kickParticipant(w http.ResponseWriter, r *http.Request) {
	c, ok := s.confByIDEditable(w, r)
	if !ok {
		return
	}
	var p struct {
		Identity string `json:"identity"`
		Name     string `json:"name"`
	}
	_ = decodeJSON(r, &p)
	identity := strings.TrimSpace(p.Identity)
	name := truncRunes(strings.TrimSpace(p.Name), 120)
	if name == "" {
		name = "Участник"
	}
	if identity == "" {
		httpErr(w, http.StatusBadRequest, "Не указан участник")
		return
	}
	if c.HostID != nil && identity == "u"+strconv.Itoa(*c.HostID) {
		httpErr(w, http.StatusBadRequest, "Нельзя удалить ведущего")
		return
	}
	var cnt int64
	s.DB.Model(&models.ConferenceBan{}).Where("conference_id = ? AND identity = ?", c.ID, identity).Count(&cnt)
	if cnt == 0 {
		s.DB.Create(&models.ConferenceBan{ConferenceID: c.ID, Identity: identity, Name: &name})
	}
	s.roomService("RemoveParticipant", map[string]any{"room": c.Room, "identity": identity}, c.Room)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "bans": s.bansOut(c.ID)})
}

func (s *Server) unbanParticipant(w http.ResponseWriter, r *http.Request) {
	c, ok := s.confByIDEditable(w, r)
	if !ok {
		return
	}
	identity := chi.URLParam(r, "identity")
	s.DB.Where("conference_id = ? AND identity = ?", c.ID, identity).Delete(&models.ConferenceBan{})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "bans": s.bansOut(c.ID)})
}

// ── записи ──────────────────────────────────────────────────────────────────

func (s *Server) recOut(rec *models.ConferenceRecording, confTitle *string, canEdit bool) map[string]any {
	title := "Запись"
	if rec.Title != nil && *rec.Title != "" {
		title = *rec.Title
	} else if confTitle != nil && *confTitle != "" {
		title = *confTitle
	}
	var url any
	if rec.Status == "done" && rec.Filename != nil {
		url = s.Cfg.APIPrefix + "/conferences/recordings/" + strconv.Itoa(rec.ID) + "/file"
	}
	var ct any
	if confTitle != nil {
		ct = *confTitle
	}
	var recorder any
	if rec.Recorder != nil {
		recorder = rec.Recorder.FullName
	}
	return map[string]any{
		"id": rec.ID, "conference_id": rec.ConferenceID, "conference_title": ct,
		"title": title, "description": rec.Description, "status": rec.Status,
		"duration_ms": rec.DurationMs, "size_bytes": rec.SizeBytes,
		"started_at": tsUTC(rec.StartedAt), "ended_at": tsPtr(rec.EndedAt),
		"recorded_by": recorder, "can_edit": canEdit, "url": url,
	}
}

func (s *Server) recordStatus(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var cnt int64
	s.DB.Model(&models.ConferenceRecording{}).Where("conference_id = ? AND status = ?", id, "active").Count(&cnt)
	writeJSON(w, http.StatusOK, map[string]any{"recording": cnt > 0, "enabled": s.recordingOn()})
}

func (s *Server) recordStart(w http.ResponseWriter, r *http.Request) {
	c, ok := s.confByIDEditable(w, r)
	if !ok {
		return
	}
	if !s.recordingOn() {
		httpErr(w, http.StatusServiceUnavailable, "Запись отключена в настройках")
		return
	}
	var cnt int64
	s.DB.Model(&models.ConferenceRecording{}).Where("conference_id = ? AND status = ?", c.ID, "active").Count(&cnt)
	if cnt > 0 {
		writeJSON(w, http.StatusOK, map[string]any{"recording": true})
		return
	}
	fn := "conf" + strconv.Itoa(c.ID) + "-" + randHex()[:12] + ".mp4"
	eid := s.startEgress(c.Room, fn, s.recHeight())
	if eid == "" {
		httpErr(w, http.StatusBadGateway, "Не удалось начать запись")
		return
	}
	u := currentUser(r)
	s.DB.Create(&models.ConferenceRecording{ConferenceID: c.ID, EgressID: &eid, Filename: &fn, Status: "active", StartedAt: time.Now(), CreatedBy: &u.ID})
	writeJSON(w, http.StatusOK, map[string]any{"recording": true})
}

func (s *Server) recordStop(w http.ResponseWriter, r *http.Request) {
	c, ok := s.confByIDEditable(w, r)
	if !ok {
		return
	}
	var recs []models.ConferenceRecording
	s.DB.Where("conference_id = ? AND status = ?", c.ID, "active").Find(&recs)
	for _, rec := range recs {
		// сразу помечаем «stopping», чтобы новая запись в этой же конференции не блокировалась
		// проверкой активных (иначе повторное «начать запись» до прихода webhook не создаёт egress).
		s.DB.Model(&models.ConferenceRecording{}).Where("id = ?", rec.ID).Update("status", "stopping")
		if rec.EgressID != nil {
			go s.stopEgressWhenReady(*rec.EgressID) // не аборти́ть ещё стартующий egress — иначе файл теряется
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"recording": false})
}

// recordParticipant — фиксируем участника, зашедшего в конференцию (webhook participant_joined).
// Гость определяется по префиксу identity ("g_"), зарегистрированный пользователь — "u<id>".
func (s *Server) recordParticipant(room string, pm map[string]any) {
	identity := anyStr(pm["identity"])
	if identity == "" {
		return
	}
	var c models.Conference
	if err := s.DB.Where("room = ?", room).First(&c).Error; err != nil {
		return
	}
	name := strings.TrimSpace(anyStr(pm["name"]))
	var namePtr *string
	if name != "" {
		namePtr = &name
	}
	isGuest := strings.HasPrefix(identity, "g_")
	s.DB.Exec(`INSERT INTO conference_participants (conference_id, identity, name, is_guest)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (conference_id, identity) DO UPDATE SET name = COALESCE(EXCLUDED.name, conference_participants.name)`,
		c.ID, identity, namePtr, isGuest)
}

// GET /conferences/{id}/participants — кто был на созвоне (зарегистрированные + гости).
func (s *Server) listConferenceParticipants(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	var rows []models.ConferenceParticipant
	s.DB.Where("conference_id = ?", id).Order("is_guest ASC, joined_at ASC").Find(&rows)
	out := []map[string]any{}
	for i := range rows {
		var nm any
		if rows[i].Name != nil {
			nm = *rows[i].Name
		}
		out = append(out, map[string]any{"name": nm, "is_guest": rows[i].IsGuest})
	}
	writeJSON(w, http.StatusOK, map[string]any{"participants": out})
}

func (s *Server) listRecordings(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	s.reconcileRecordings() // добрать статус у зависших/остановленных записей (потерянный webhook)
	var rows []models.ConferenceRecording
	s.DB.Preload("Conference").Preload("Recorder").Where("status = ? AND filename IS NOT NULL", "done").
		Order("started_at DESC").Find(&rows)
	dirty := false
	for i := range rows {
		if rows[i].DurationMs == 0 || rows[i].SizeBytes == 0 {
			s.probeFile(&rows[i])
			dirty = true
		}
	}
	if dirty {
		for i := range rows {
			s.DB.Model(&models.ConferenceRecording{}).Where("id = ?", rows[i].ID).
				Updates(map[string]any{"duration_ms": rows[i].DurationMs, "size_bytes": rows[i].SizeBytes})
		}
	}
	elevated := s.isSuperadmin(u.ID)
	out := []map[string]any{}
	for i := range rows {
		var ct *string
		canEdit := false
		if rows[i].Conference != nil {
			ct = &rows[i].Conference.Title
			canEdit = (rows[i].Conference.HostID != nil && *rows[i].Conference.HostID == u.ID) || elevated
		}
		out = append(out, s.recOut(&rows[i], ct, canEdit))
	}
	writeJSON(w, http.StatusOK, map[string]any{"recordings": out})
}

func (s *Server) updateRecording(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var rec models.ConferenceRecording
	if err := s.DB.First(&rec, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Запись не найдена")
		return
	}
	var c models.Conference
	if err := s.DB.First(&c, rec.ConferenceID).Error; err != nil || !s.confEditable(u.ID, &c) {
		httpErr(w, http.StatusForbidden, "Менять запись может ведущий или модератор")
		return
	}
	var p map[string]any
	_ = decodeJSON(r, &p)
	upd := map[string]any{}
	if v, ok := p["title"]; ok {
		t := truncRunes(strings.TrimSpace(anyStr(v)), 255)
		if t == "" {
			upd["title"] = nil
		} else {
			upd["title"] = t
		}
	}
	if v, ok := p["description"]; ok {
		d := strings.TrimSpace(anyStr(v))
		if d == "" {
			upd["description"] = nil
		} else {
			upd["description"] = d
		}
	}
	if len(upd) > 0 {
		s.DB.Model(&models.ConferenceRecording{}).Where("id = ?", id).Updates(upd)
		s.DB.First(&rec, id)
	}
	writeJSON(w, http.StatusOK, s.recOut(&rec, &c.Title, true))
}

func (s *Server) deleteRecording(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	u := currentUser(r)
	var rec models.ConferenceRecording
	if err := s.DB.First(&rec, id).Error; err != nil {
		httpErr(w, http.StatusNotFound, "Запись не найдена")
		return
	}
	var c models.Conference
	if err := s.DB.First(&c, rec.ConferenceID).Error; err != nil || !s.confEditable(u.ID, &c) {
		httpErr(w, http.StatusForbidden, "Удалить запись может ведущий или модератор")
		return
	}
	if rec.Status == "active" && rec.EgressID != nil {
		s.stopEgress(*rec.EgressID)
	}
	if rec.Filename != nil {
		os.Remove(filepath.Join(s.Cfg.RecordingsDir, filepath.Base(*rec.Filename)))
	}
	s.DB.Delete(&models.ConferenceRecording{}, id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) recordingFile(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	raw := r.URL.Query().Get("token")
	if raw == "" {
		raw = strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	}
	claims, err := s.JWT.Parse(raw)
	if err != nil {
		httpErr(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	email, _ := claims["sub"].(string)
	var u models.User
	if email == "" || s.DB.Where("email = ?", email).First(&u).Error != nil || !u.IsActive {
		httpErr(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	if !caps.HasCap(s.DB, u.ID, "conference.view") {
		httpErr(w, http.StatusForbidden, "Недостаточно прав")
		return
	}
	var rec models.ConferenceRecording
	if err := s.DB.First(&rec, id).Error; err != nil || rec.Filename == nil || rec.Status != "done" {
		httpErr(w, http.StatusNotFound, "Запись не найдена")
		return
	}
	path := filepath.Join(s.Cfg.RecordingsDir, filepath.Base(*rec.Filename))
	f, err := os.Open(path)
	if err != nil {
		httpErr(w, http.StatusNotFound, "Файл записи не найден")
		return
	}
	defer f.Close()
	fi, _ := f.Stat()
	w.Header().Set("Content-Type", "video/mp4")
	http.ServeContent(w, r, filepath.Base(path), fi.ModTime(), f)
}

func (s *Server) livekitWebhook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	auth := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if _, err := jwt.Parse(auth, func(t *jwt.Token) (any, error) { return []byte(s.Cfg.LiveKitAPISecret), nil },
		jwt.WithoutClaimsValidation()); err != nil {
		httpErr(w, http.StatusUnauthorized, "bad signature")
		return
	}
	var data map[string]any
	if json.Unmarshal(body, &data) != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	event, _ := data["event"].(string)
	room := ""
	if rm, ok := data["room"].(map[string]any); ok {
		room, _ = rm["name"].(string)
	}
	if event == "room_started" && room != "" {
		var c models.Conference
		if err := s.DB.Where("room = ?", room).First(&c).Error; err == nil {
			go s.maybeAutoRecord(&c) // комната создана — можно ставить авто-запись (egress стартует долго)
		}
	} else if event == "room_finished" && room != "" {
		var c models.Conference
		if err := s.DB.Where("room = ? AND status = ?", room, "live").First(&c).Error; err == nil {
			s.DB.Model(&models.Conference{}).Where("id = ?", c.ID).Updates(map[string]any{"status": "ended", "ended_at": gorm.Expr("now()")})
			var recs []models.ConferenceRecording
			s.DB.Where("conference_id = ? AND status = ?", c.ID, "active").Find(&recs)
			for _, rec := range recs {
				if rec.EgressID != nil {
					s.stopEgress(*rec.EgressID)
				}
			}
		}
	} else if event == "participant_joined" && room != "" {
		if pm, ok := data["participant"].(map[string]any); ok {
			s.recordParticipant(room, pm)
		}
	} else if (event == "egress_ended" || event == "egress_updated") && data["egressInfo"] != nil {
		info, _ := data["egressInfo"].(map[string]any)
		eid := anyStr(info["egressId"])
		if eid == "" {
			eid = anyStr(info["egress_id"])
		}
		var rec models.ConferenceRecording
		if err := s.DB.Where("egress_id = ? AND status IN ?", eid, []string{"active", "stopping"}).First(&rec).Error; err == nil {
			s.applyEgressWebhook(&rec, event, info)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) applyEgressWebhook(rec *models.ConferenceRecording, event string, info map[string]any) {
	st := anyStr(info["status"])
	files := egressFiles(info)
	upd := map[string]any{}
	if event == "egress_ended" {
		if (st == "EGRESS_COMPLETE") && len(files) > 0 {
			f := files[0]
			upd["status"] = "done"
			if fn := anyStr(f["filename"]); fn != "" {
				parts := strings.Split(fn, "/")
				upd["filename"] = parts[len(parts)-1]
			}
			if d := anyInt(f["duration"]); d > 0 {
				upd["duration_ms"] = d / 1_000_000
			}
			if sz := anyInt(f["size"]); sz > 0 {
				upd["size_bytes"] = sz
			}
			upd["ended_at"] = gorm.Expr("now()")
		} else if st == "EGRESS_FAILED" || st == "EGRESS_ABORTED" || st == "EGRESS_LIMIT_REACHED" {
			upd["status"] = "failed"
			upd["ended_at"] = gorm.Expr("now()")
		}
	}
	if len(upd) > 0 {
		s.DB.Model(&models.ConferenceRecording{}).Where("id = ?", rec.ID).Updates(upd)
		if upd["status"] == "done" {
			s.DB.First(rec, rec.ID)
			s.probeFile(rec)
			s.DB.Model(&models.ConferenceRecording{}).Where("id = ?", rec.ID).
				Updates(map[string]any{"duration_ms": rec.DurationMs, "size_bytes": rec.SizeBytes, "filename": rec.Filename})
		}
	}
}

// reconcileRecordings — самолечение списка: если webhook egress_ended потерялся, добираем финальный
// статус напрямую у LiveKit (или, как крайний случай, по готовому файлу на диске). Благодаря этому
// короткие/«потерянные» записи всё равно попадают в список, а не зависают в статусе active/stopping.
func (s *Server) reconcileRecordings() {
	var recs []models.ConferenceRecording
	// все остановленные (stopping) + давно «висящие» active (сессия оборвалась без webhook)
	s.DB.Where("status = ? OR (status = ? AND started_at < now() - interval '6 hours')", "stopping", "active").Find(&recs)
	for i := range recs {
		rec := &recs[i]
		if rec.EgressID == nil {
			continue
		}
		if out, err := s.egressService("ListEgress", map[string]any{"egressId": *rec.EgressID}); err == nil && out != nil {
			if items, ok := out["items"].([]any); ok && len(items) > 0 {
				if info, ok := items[0].(map[string]any); ok {
					switch anyStr(info["status"]) {
					case "EGRESS_COMPLETE", "EGRESS_FAILED", "EGRESS_ABORTED", "EGRESS_LIMIT_REACHED":
						s.applyEgressWebhook(rec, "egress_ended", info)
					}
					continue
				}
			}
		}
		// LiveKit не отдал данных по egress — финализируем по файлу на диске, если он уже записан
		if rec.Filename != nil {
			path := filepath.Join(s.Cfg.RecordingsDir, filepath.Base(*rec.Filename))
			if fi, e := os.Stat(path); e == nil && fi.Size() > 0 {
				s.DB.Model(&models.ConferenceRecording{}).Where("id = ?", rec.ID).
					Updates(map[string]any{"status": "done", "ended_at": gorm.Expr("now()")})
				s.DB.First(rec, rec.ID)
				s.probeFile(rec)
				s.DB.Model(&models.ConferenceRecording{}).Where("id = ?", rec.ID).
					Updates(map[string]any{"duration_ms": rec.DurationMs, "size_bytes": rec.SizeBytes})
			}
		}
	}
}

func egressFiles(info map[string]any) []map[string]any {
	var files []map[string]any
	for _, key := range []string{"fileResults", "file_results"} {
		if arr, ok := info[key].([]any); ok {
			for _, x := range arr {
				if m, ok := x.(map[string]any); ok {
					files = append(files, m)
				}
			}
		}
	}
	if len(files) == 0 {
		if f, ok := info["file"].(map[string]any); ok {
			files = append(files, f)
		}
	}
	return files
}

func anyStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func anyInt(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	}
	return 0
}

// ── длительность MP4 из атома mvhd ───────────────────────────────────────────

func (s *Server) probeFile(rec *models.ConferenceRecording) {
	if rec.Filename == nil {
		return
	}
	path := filepath.Join(s.Cfg.RecordingsDir, filepath.Base(*rec.Filename))
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	rec.SizeBytes = fi.Size()
	if d := mp4DurationMs(path); d > 0 {
		rec.DurationMs = d
	}
}

func mp4DurationMs(path string) int64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	fi, _ := f.Stat()
	total := fi.Size()
	findBox := func(start, end int64, target string) (int64, int64) {
		pos := start
		for pos < end {
			f.Seek(pos, io.SeekStart)
			var hdr [8]byte
			if _, err := io.ReadFull(f, hdr[:]); err != nil {
				return 0, 0
			}
			size := int64(binary.BigEndian.Uint32(hdr[:4]))
			typ := string(hdr[4:8])
			boxStart := pos
			hdrLen := int64(8)
			if size == 1 {
				var ext [8]byte
				io.ReadFull(f, ext[:])
				size = int64(binary.BigEndian.Uint64(ext[:]))
				hdrLen = 16
			} else if size == 0 {
				size = end - boxStart
			}
			if typ == target {
				return boxStart + hdrLen, size - hdrLen
			}
			pos = boxStart + size
		}
		return 0, 0
	}
	moovStart, moovLen := findBox(0, total, "moov")
	if moovLen == 0 {
		return 0
	}
	mvhdStart, _ := findBox(moovStart, moovStart+moovLen, "mvhd")
	if mvhdStart == 0 {
		return 0
	}
	f.Seek(mvhdStart, io.SeekStart)
	var ver [4]byte
	io.ReadFull(f, ver[:])
	var timescale, duration uint64
	if ver[0] == 1 {
		var buf [28]byte
		io.ReadFull(f, buf[:])
		timescale = uint64(binary.BigEndian.Uint32(buf[16:20]))
		duration = binary.BigEndian.Uint64(buf[20:28])
	} else {
		var buf [16]byte
		io.ReadFull(f, buf[:])
		timescale = uint64(binary.BigEndian.Uint32(buf[8:12]))
		duration = uint64(binary.BigEndian.Uint32(buf[12:16]))
	}
	if timescale == 0 {
		return 0
	}
	return int64(duration * 1000 / timescale)
}
