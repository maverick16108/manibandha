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

	// публичные (без токена): расписание для лендинга
	api.Get("/events/public/upcoming", s.publicUpcoming)
	api.Get("/events/public", s.publicList)
	api.Get("/events/public/{id}", s.publicDetail)

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
		pr.Get("/users/{id}/card", s.userCardHandler)
		// справочники: чтение — любому авторизованному
		pr.Get("/cities", s.listCities)
		pr.Get("/regions", s.listRegions)
		pr.Get("/countries", s.listCountries)
		pr.Get("/temples", s.listTemples)
		// ученики: доступ по скоупу проверяется внутри
		pr.Get("/disciples", s.listDisciples)
		pr.Get("/disciples/{id}", s.getDisciple)
		pr.Patch("/disciples/{id}", s.updateDisciple)
		pr.Get("/disciples/{id}/files", s.listFiles)
		// чек-лист (pipeline): чтение/правка — доступ и права внутри
		pr.Get("/disciples/{id}/checklist", s.listChecklist)
		pr.Post("/disciples/{id}/checklist", s.addChecklist)
		pr.Patch("/disciples/{id}/checklist/{itemId}", s.updateChecklist)
		// ветки (вопросы/отчёты/approval)
		pr.Get("/threads", s.listThreads)
		pr.Get("/threads/nav-counts", s.navCounts)
		pr.Get("/threads/stats", s.threadStats)
		pr.Get("/threads/{id}", s.getThread)
		pr.Post("/threads", s.createThread)
		pr.Post("/threads/{id}/messages", s.addThreadMessage)
		pr.Patch("/threads/{id}/messages/{mid}", s.editThreadMessage)
		pr.Delete("/threads/{id}/messages/{mid}", s.deleteThreadMessage)
		pr.Post("/threads/{id}/messages/{mid}/react", s.reactThreadMessage)
		// события (чтение), черновики, настройки (чтение)
		pr.Get("/events", s.listEvents)
		pr.Get("/events/{id}", s.getEvent)
		pr.Get("/drafts/{scope}", s.getDraft)
		pr.Put("/drafts/{scope}", s.saveDraft)
		pr.Delete("/drafts/{scope}", s.deleteDraft)
		pr.Get("/settings", s.readSettings)
		pr.Post("/uploads", s.upload)
		// отчёты (агрегаты + экспорт)
		pr.Get("/reports/summary", s.reportSummary)
		pr.Get("/reports/timeline", s.reportTimeline)
		pr.Get("/reports/group", s.reportGroup)
		pr.Get("/reports/disciples.xlsx", s.exportXlsx)
		pr.Get("/reports/disciples.pdf", s.exportPdf)
	})

	// события: изменение — staff
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.staff)
		pr.Post("/events", s.createEvent)
		pr.Patch("/events/{id}", s.updateEvent)
		pr.Delete("/events/{id}", s.deleteEvent)
	})

	// настройки: изменение — право settings.manage
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.requireCap("settings.manage"))
		pr.Put("/settings", s.updateSettings)
	})

	// форум: чтение — право forum.view
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.requireCap("forum.view"))
		pr.Get("/forum/users/{id}", s.forumUserCard)
		pr.Get("/forum/sections", s.listSections)
		pr.Get("/forum/topics", s.listTopics)
		pr.Get("/forum/topics/{id}", s.getTopic)
		pr.Post("/forum/posts/{id}/like", s.toggleLike)
	})

	// форум: запись — право forum.post
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.requireCap("forum.post"))
		pr.Post("/forum/sections", s.createSection)
		pr.Patch("/forum/sections/{id}", s.updateSection)
		pr.Delete("/forum/sections/{id}", s.deleteSection)
		pr.Post("/forum/topics", s.createTopic)
		pr.Post("/forum/topics/{id}/posts", s.addForumPost)
		pr.Delete("/forum/topics/{id}", s.deleteTopic)
		pr.Patch("/forum/posts/{id}", s.editForumPost)
		pr.Delete("/forum/posts/{id}", s.deleteForumPost)
	})

	// ученики: заметки (право disciples.note)
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.requireCap("disciples.note"))
		pr.Get("/disciples/{id}/notes", s.listNotes)
		pr.Post("/disciples/{id}/notes", s.addNote)
		pr.Delete("/disciples/{id}/notes/{noteId}", s.deleteNote)
	})

	// ученики: файлы (право disciples.edit)
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.requireCap("disciples.edit"))
		pr.Post("/disciples/{id}/files", s.uploadDiscipleFile)
		pr.Delete("/disciples/{id}/files/{fileId}", s.deleteDiscipleFile)
	})

	// ученики: апрув (право disciples.approve)
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.requireCap("disciples.approve"))
		pr.Post("/disciples/{id}/approve", s.approveDisciple)
	})

	// ученики: создание/удаление (staff)
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.staff)
		pr.Post("/disciples", s.createDisciple)
		pr.Delete("/disciples/{id}", s.deleteDisciple)
		pr.Delete("/disciples/{id}/checklist/{itemId}", s.deleteChecklist)
		// наставники (справочник кураторов)
		pr.Get("/mentors", s.listMentorsDict)
		pr.Post("/mentors", s.createMentor)
		pr.Patch("/mentors/{id}", s.renameMentor)
		pr.Delete("/mentors/{id}", s.deleteMentor)
	})

	// справочники: изменение — staff
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.staff)
		pr.Post("/cities", s.createCity)
		pr.Patch("/cities/{id}", s.updateCity)
		pr.Delete("/cities/{id}", s.deleteCity)
		pr.Post("/regions", s.createRegion)
		pr.Patch("/regions/{id}", s.updateRegion)
		pr.Delete("/regions/{id}", s.deleteRegion)
		pr.Post("/countries", s.createCountry)
		pr.Patch("/countries/{id}", s.updateCountry)
		pr.Delete("/countries/{id}", s.deleteCountry)
		pr.Post("/temples", s.createTemple)
		pr.Patch("/temples/{id}", s.updateTemple)
		pr.Delete("/temples/{id}", s.deleteTemple)
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

	// мессенджер (доступ: авторизация + не-pending)
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.chatGate)
		pr.Get("/chats", s.listChats)
		pr.Get("/chats/contacts", s.listContacts)
		pr.Get("/chats/updates", s.getUpdates)
		pr.Post("/chats", s.createChat)
		pr.Get("/chats/{id}", s.getChat)
		pr.Patch("/chats/{id}", s.updateChatHandler)
		pr.Get("/chats/{id}/messages", s.listMessages)
		pr.Post("/chats/{id}/messages", s.sendMessage)
		pr.Patch("/chats/{id}/messages/{mid}", s.editChatMessage)
		pr.Delete("/chats/{id}/messages/{mid}", s.deleteChatMessage)
		pr.Post("/chats/{id}/read", s.markChatRead)
		pr.Post("/chats/{id}/pin", s.pinChat)
		pr.Post("/chats/pins/reorder", s.reorderPins)
		pr.Delete("/chats/{id}/leave", s.leaveChat)
		pr.Post("/chats/{id}/messages/{mid}/react", s.reactChatMessage)
		pr.Get("/link-preview", s.linkPreview) // OG-превью ссылок в сообщениях
		pr.Get("/chats/{id}/search", s.searchChatMessages)
		pr.Get("/chats/{id}/info", s.chatInfo)
	})

	// конференции: публичные (без токена)
	api.Get("/conferences/by-code/{code}", s.resolveCode)
	api.Post("/conferences/guest/{room}", s.guestJoin)
	api.Post("/conferences/livekit-webhook", s.livekitWebhook)
	api.Get("/conferences/recordings/{id}/file", s.recordingFile) // авторизация по ?token= внутри

	// конференции: право conference.host
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.requireCap("conference.host"))
		pr.Post("/conferences", s.createConference)
		pr.Get("/conferences/moderators", s.listModerators)
	})

	// конференции: право conference.view
	api.Group(func(pr chi.Router) {
		pr.Use(s.auth, s.requireCap("conference.view"))
		pr.Get("/conferences", s.listConferences)
		pr.Get("/conferences/recordings", s.listRecordings)
		pr.Patch("/conferences/recordings/{id}", s.updateRecording)
		pr.Delete("/conferences/recordings/{id}", s.deleteRecording)
		pr.Patch("/conferences/{id}", s.updateConference)
		pr.Delete("/conferences/{id}", s.deleteConference)
		pr.Post("/conferences/{id}/join", s.joinConference)
		pr.Post("/conferences/{id}/permit", s.moderatePermit)
		pr.Get("/conferences/{id}/bans", s.listBans)
		pr.Post("/conferences/{id}/kick", s.kickParticipant)
		pr.Delete("/conferences/{id}/bans/{identity}", s.unbanParticipant)
		pr.Get("/conferences/{id}/record", s.recordStatus)
		pr.Post("/conferences/{id}/record/start", s.recordStart)
		pr.Post("/conferences/{id}/record/stop", s.recordStop)
	})

	// WebSocket (аутентификация по ?token=... внутри)
	api.Get("/ws/chat", s.wsChat)
	api.Get("/ws/threads/{id}", s.wsThread)

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
