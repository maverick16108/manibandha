package web

import (
	"context"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Превью ссылок для чата: тянем OG/oEmbed-мета целевой страницы и отдаём карточку.
// Безопасность: ходим только по http(s) на публичные адреса — приватные/loopback IP
// блокируются на этапе соединения (кастомный DialContext), это покрывает и редиректы.

type linkPreview struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image"`
	SiteName    string `json:"site_name"`
}

type cachedPreview struct {
	p   linkPreview
	exp time.Time
}

var (
	lpMu    sync.RWMutex
	lpCache = map[string]cachedPreview{}
)

const (
	lpTTL       = 6 * time.Hour  // положительный кэш
	lpNegTTL    = 30 * time.Minute // отрицательный кэш (не смогли получить)
	lpMaxBody   = 512 * 1024      // читаем не больше 512 КБ HTML
	lpUserAgent = "Mozilla/5.0 (compatible; ManibandhaBot/1.0; +https://manibandha.ru)"
)

// приватные/служебные адреса, куда сервер ходить не должен
func isBlockedIP(ip net.IP) bool {
	return ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip4InCGNAT(ip)
}

// 100.64.0.0/10 — CGNAT, не публичный
func ip4InCGNAT(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127
}

// HTTP-клиент, который отказывается соединяться с приватными адресами (в т.ч. при редиректах)
var lpClient = &http.Client{
	Timeout: 6 * time.Second,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			var target net.IP
			for _, ip := range ips {
				if !isBlockedIP(ip) {
					target = ip
					break
				}
			}
			if target == nil {
				return nil, fmt.Errorf("blocked host %s", host)
			}
			d := &net.Dialer{Timeout: 4 * time.Second}
			// соединяемся ровно с проверенным IP (защита от DNS-rebinding)
			return d.DialContext(ctx, network, net.JoinHostPort(target.String(), port))
		},
		MaxIdleConns:        20,
		IdleConnTimeout:     60 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	},
}

var (
	reMeta    = regexp.MustCompile(`(?is)<meta[^>]+>`)
	reTitle   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reAttrKey = regexp.MustCompile(`(?is)(property|name)\s*=\s*["']([^"']+)["']`)
	reAttrCon = regexp.MustCompile(`(?is)content\s*=\s*["']([^"']*)["']`)
)

func metaMap(htmlStr string) map[string]string {
	out := map[string]string{}
	for _, tag := range reMeta.FindAllString(htmlStr, -1) {
		key := reAttrKey.FindStringSubmatch(tag)
		con := reAttrCon.FindStringSubmatch(tag)
		if key == nil || con == nil {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(key[2]))
		if _, exists := out[k]; !exists {
			out[k] = html.UnescapeString(strings.TrimSpace(con[1]))
		}
	}
	return out
}

func fetchPreview(raw string) (linkPreview, bool) {
	p := linkPreview{URL: raw}
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return p, false
	}
	req.Header.Set("User-Agent", lpUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "ru,en;q=0.8")
	resp, err := lpClient.Do(req)
	if err != nil {
		return p, false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return p, false
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "html") {
		return p, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, lpMaxBody))
	if err != nil {
		return p, false
	}
	m := metaMap(string(body))
	p.Title = firstNonEmpty(m["og:title"], m["twitter:title"])
	p.Description = firstNonEmpty(m["og:description"], m["twitter:description"], m["description"])
	p.Image = firstNonEmpty(m["og:image"], m["og:image:url"], m["twitter:image"], m["twitter:image:src"])
	p.SiteName = m["og:site_name"]
	if p.Title == "" {
		if t := reTitle.FindStringSubmatch(string(body)); t != nil {
			p.Title = html.UnescapeString(strings.TrimSpace(t[1]))
		}
	}
	// абсолютизируем картинку и валидируем её схему
	p.Image = absURL(raw, p.Image)
	if p.SiteName == "" {
		if u, e := url.Parse(raw); e == nil {
			p.SiteName = strings.TrimPrefix(u.Hostname(), "www.")
		}
	}
	ok := p.Title != "" || p.Image != "" || p.Description != ""
	return p, ok
}

func absURL(base, ref string) string {
	if ref == "" {
		return ""
	}
	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	res := b.ResolveReference(r)
	if res.Scheme != "http" && res.Scheme != "https" {
		return ""
	}
	return res.String()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// GET /api/link-preview?url=...
func (s *Server) linkPreview(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("url"))
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeJSON(w, http.StatusOK, linkPreview{URL: raw})
		return
	}
	// кэш
	lpMu.RLock()
	c, ok := lpCache[raw]
	lpMu.RUnlock()
	if ok && time.Now().Before(c.exp) {
		writeJSON(w, http.StatusOK, c.p)
		return
	}
	p, good := fetchPreview(raw)
	ttl := lpTTL
	if !good {
		ttl = lpNegTTL
	}
	lpMu.Lock()
	lpCache[raw] = cachedPreview{p: p, exp: time.Now().Add(ttl)}
	// не даём кэшу разрастаться бесконечно
	if len(lpCache) > 2000 {
		now := time.Now()
		for k, v := range lpCache {
			if now.After(v.exp) {
				delete(lpCache, k)
			}
		}
	}
	lpMu.Unlock()
	writeJSON(w, http.StatusOK, p)
}
