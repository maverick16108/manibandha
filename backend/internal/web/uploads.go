package web

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// microPreview — крошечное (≈24px) превью в data-URI (JPEG q35). Фронт показывает его
// мгновенно как размытую подложку и «проявляет» из неё картинку (blur-up, как в Telegram).
func microPreview(img image.Image) string {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return ""
	}
	const W = 24
	h := W * b.Dy() / b.Dx()
	if h < 1 {
		h = 1
	}
	if h > 48 {
		h = 48
	}
	dst := image.NewRGBA(image.Rect(0, 0, W, h))
	for y := 0; y < h; y++ {
		for x := 0; x < W; x++ {
			sx := b.Min.X + x*b.Dx()/W
			sy := b.Min.Y + y*b.Dy()/h
			dst.Set(x, y, img.At(sx, sy))
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 35}); err != nil {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

// avgColor — грубый средний цвет картинки (по сетке сэмплов) для мгновенной подложки «в цвет».
func avgColor(img image.Image) string {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return ""
	}
	stepX := b.Dx()/16 + 1
	stepY := b.Dy()/16 + 1
	var r, g, bl, n uint64
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			r += uint64(cr >> 8)
			g += uint64(cg >> 8)
			bl += uint64(cb >> 8)
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("#%02x%02x%02x", r/n, g/n, bl/n)
}

// GET /api/uploads-dims?u=/uploads/a.webp,/uploads/b.webp — соотношение сторон + средний цвет
// картинок: фронт резервирует место (без «прыжка») и показывает подложку в цвет ДО загрузки.
func (s *Server) uploadsDims(w http.ResponseWriter, r *http.Request) {
	out := map[string]map[string]any{}
	for _, u := range strings.Split(r.URL.Query().Get("u"), ",") {
		u = strings.TrimSpace(u)
		if u == "" || !strings.HasPrefix(u, "/uploads/") || strings.Contains(u, "..") {
			continue
		}
		name := filepath.Base(u)
		// декодируем миниатюру (маленькая → быстро) для размеров и цвета; иначе сам файл
		ext := filepath.Ext(name)
		thumb := strings.TrimSuffix(name, ext) + ".thumb" + ext
		var img image.Image
		if f, err := os.Open(filepath.Join(s.Cfg.UploadDir, thumb)); err == nil {
			img, _, _ = image.Decode(f)
			f.Close()
		}
		if img == nil {
			if f, err := os.Open(filepath.Join(s.Cfg.UploadDir, name)); err == nil {
				img, _, _ = image.Decode(f)
				f.Close()
			}
		}
		if img == nil {
			continue
		}
		b := img.Bounds()
		if b.Dy() <= 0 {
			continue
		}
		out[u] = map[string]any{
			"a": math.Round(float64(b.Dx())/float64(b.Dy())*1000) / 1000,
			"c": avgColor(img),
			"m": microPreview(img),
		}
	}
	writeJSON(w, http.StatusOK, out)
}

const maxUploadBytes = 16 * 1024 * 1024        // картинки/аудио/документы
const maxVideoBytes = 100 * 1024 * 1024        // видео — больший лимит

var uploadVideoTypes = map[string]bool{"video/mp4": true, "video/quicktime": true, "video/webm": true}

// расширения по content-type (как в app/api/routes/uploads.py)
var uploadAllowed = map[string]string{
	"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif",
	"audio/webm": ".webm", "audio/ogg": ".ogg", "audio/mpeg": ".mp3",
	"audio/mp4": ".m4a", "audio/x-m4a": ".m4a", "audio/wav": ".wav", "audio/x-wav": ".wav",
	"application/pdf": ".pdf",
	"application/msword": ".doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
	"application/vnd.ms-excel": ".xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": ".xlsx",
	"application/vnd.ms-powerpoint": ".ppt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
	"application/zip": ".zip", "application/x-zip-compressed": ".zip",
	"application/x-rar-compressed": ".rar", "application/vnd.rar": ".rar",
	"application/x-7z-compressed": ".7z",
	"text/plain": ".txt", "text/csv": ".csv",
	"video/mp4": ".mp4", "video/quicktime": ".mov", "video/webm": ".webm",
}

var uploadImageTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

const uploadMainMax = 1600
const uploadThumbMax = 320

// POST /uploads
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxVideoBytes + 1024); err != nil {
		httpErr(w, http.StatusBadRequest, "Некорректная форма")
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		httpErr(w, http.StatusBadRequest, "Файлы не переданы")
		return
	}
	_ = os.MkdirAll(s.Cfg.UploadDir, 0o755)

	urls := []any{}
	thumbs := []any{}
	for _, fh := range files {
		ctype := strings.TrimSpace(strings.Split(fh.Header.Get("Content-Type"), ";")[0])
		ext, ok := uploadAllowed[ctype]
		if !ok {
			httpErr(w, http.StatusBadRequest, "Неподдерживаемый тип файла: "+ctype)
			return
		}
		f, err := fh.Open()
		if err != nil {
			httpErr(w, http.StatusBadRequest, "Не удалось прочитать файл")
			return
		}
		limit := int64(maxUploadBytes)
		if uploadVideoTypes[ctype] {
			limit = maxVideoBytes
		}
		data, _ := io.ReadAll(io.LimitReader(f, limit+1))
		f.Close()
		if int64(len(data)) > limit {
			if uploadVideoTypes[ctype] {
				httpErr(w, http.StatusBadRequest, "Видео больше 100 МБ")
			} else {
				httpErr(w, http.StatusBadRequest, "Файл больше 16 МБ")
			}
			return
		}
		stem := randHex()
		if uploadImageTypes[ctype] {
			if url, thumb, err := s.saveImage(data, stem); err == nil {
				urls = append(urls, url)
				thumbs = append(thumbs, thumb)
				continue
			}
		}
		name := stem + ext
		if err := os.WriteFile(filepath.Join(s.Cfg.UploadDir, name), data, 0o644); err != nil {
			httpErr(w, http.StatusInternalServerError, "Не удалось сохранить файл")
			return
		}
		urls = append(urls, "/uploads/"+name)
		thumbs = append(thumbs, nil)
	}
	writeJSON(w, http.StatusOK, map[string]any{"urls": urls, "thumbs": thumbs})
}

// saveImage пережимает картинку в webp + делает превью (через cwebp). Возвращает (url, thumbUrl).
func (s *Server) saveImage(data []byte, stem string) (string, string, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", "", err
	}
	tmp, err := os.CreateTemp("", "upl-*")
	if err != nil {
		return "", "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", "", err
	}
	tmp.Close()

	mainName := stem + ".webp"
	thumbName := stem + ".thumb.webp"
	mainPath := filepath.Join(s.Cfg.UploadDir, mainName)
	thumbPath := filepath.Join(s.Cfg.UploadDir, thumbName)

	if err := cwebp(tmp.Name(), mainPath, cfg.Width, cfg.Height, uploadMainMax, 85); err != nil {
		return "", "", err
	}
	if err := cwebp(tmp.Name(), thumbPath, cfg.Width, cfg.Height, uploadThumbMax, 80); err != nil {
		os.Remove(mainPath)
		return "", "", err
	}
	return "/uploads/" + mainName, "/uploads/" + thumbName, nil
}

// cwebp кодирует webp с ресайзом по длинной стороне (0 = без ресайза для этой оси).
func cwebp(in, out string, w, h, max, q int) error {
	args := []string{"-quiet", "-q", itoa(q)}
	rw, rh := fitResize(w, h, max)
	if rw > 0 || rh > 0 {
		args = append(args, "-resize", itoa(rw), itoa(rh))
	}
	args = append(args, "-o", out, in)
	return exec.Command("cwebp", args...).Run()
}

func fitResize(w, h, max int) (int, int) {
	if w <= max && h <= max {
		return 0, 0
	}
	if w >= h {
		return max, 0
	}
	return 0, max
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
