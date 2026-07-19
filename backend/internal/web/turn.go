package web

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"
)

// GET /api/turn-credentials — эфемерные креденшелы для TURN (coturn use-auth-secret).
// username = "<expiry-unix>:webrtc", credential = base64(HMAC-SHA1(secret, username)).
func (s *Server) turnCredentials(w http.ResponseWriter, r *http.Request) {
	secret := s.Cfg.TurnSecret
	host := s.Cfg.TurnHost
	// STUN всегда пригодится (публичный + свой), TURN — только при наличии секрета
	iceServers := []map[string]any{
		{"urls": []string{"stun:stun.l.google.com:19302"}},
	}
	if secret != "" && host != "" {
		ttl := 3600
		username := fmt.Sprintf("%d:webrtc", time.Now().Add(time.Duration(ttl)*time.Second).Unix())
		mac := hmac.New(sha1.New, []byte(secret))
		mac.Write([]byte(username))
		cred := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		iceServers = append(iceServers, map[string]any{
			"urls": []string{
				fmt.Sprintf("turn:%s:3478?transport=udp", host),
				fmt.Sprintf("turn:%s:3478?transport=tcp", host),
				fmt.Sprintf("turns:%s:5349?transport=tcp", host),
			},
			"username":   username,
			"credential": cred,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"iceServers": iceServers})
}
