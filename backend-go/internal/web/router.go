package web

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Router собирает все маршруты под API_PREFIX + статику /uploads.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.Cfg.CorsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}))

	api := chi.NewRouter()

	api.Get("/health", s.health)

	// auth (публичные)
	api.Post("/auth/login", s.login)
	api.Post("/auth/phone/request", s.phoneRequest)
	api.Post("/auth/phone/verify", s.phoneVerify)

	// требующие токена
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth)
		pr.Post("/auth/refresh", s.refresh)
		pr.Get("/auth/me", s.me)
		pr.Patch("/auth/me", s.patchMe)
		pr.Get("/me/capabilities", s.myCapabilities)
		pr.Get("/users/mentors", s.listMentors)
	})

	// staff (гуру/секретарь): управление пользователями
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.staff)
		pr.Get("/users", s.listUsers)
		pr.Post("/users", s.createUser)
		pr.Patch("/users/{id}", s.updateUser)
		pr.Delete("/users/{id}", s.deleteUser)
	})

	// право roles.manage: справочник прав и CRUD ролей
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.requireCap("roles.manage"))
		pr.Get("/capabilities", s.listCapabilities)
		pr.Get("/roles", s.listRoles)
		pr.Post("/roles", s.createRole)
		pr.Put("/roles/{id}", s.updateRole)
		pr.Delete("/roles/{id}", s.deleteRole)
	})

	// право users.manage: роли пользователя
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.requireCap("users.manage"))
		pr.Get("/users/{id}/roles", s.getUserRoles)
		pr.Put("/users/{id}/roles", s.setUserRoles)
	})

	r.Mount(s.Cfg.APIPrefix, api)

	// статика загруженных файлов (в проде обычно раздаёт nginx)
	_ = os.MkdirAll(s.Cfg.UploadDir, 0o755)
	fs := http.StripPrefix("/uploads/", http.FileServer(http.Dir(s.Cfg.UploadDir)))
	r.Handle("/uploads/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "..") {
			http.NotFound(w, req)
			return
		}
		fs.ServeHTTP(w, req)
	}))

	return r
}
