package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Time — time.Time, который сериализуется в UTC с суффиксом Z (как pydantic в Python).
type Time struct{ time.Time }

func (t Time) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte("null"), nil
	}
	u := t.Time.UTC()
	layout := "2006-01-02T15:04:05.000000Z07:00"
	if u.Nanosecond() == 0 {
		layout = "2006-01-02T15:04:05Z07:00"
	}
	return []byte(`"` + u.Format(layout) + `"`), nil
}

func (t *Time) Scan(v any) error {
	if v == nil {
		t.Time = time.Time{}
		return nil
	}
	tt, ok := v.(time.Time)
	if !ok {
		return errors.New("Time: unsupported scan type")
	}
	t.Time = tt
	return nil
}

func (t Time) Value() (driver.Value, error) { return t.Time, nil }

// StringList — JSON-массив строк в колонке (roles.capabilities).
type StringList []string

func (s *StringList) Scan(v any) error {
	if v == nil {
		*s = nil
		return nil
	}
	var b []byte
	switch t := v.(type) {
	case []byte:
		b = t
	case string:
		b = []byte(t)
	default:
		return errors.New("StringList: unsupported scan type")
	}
	if len(b) == 0 {
		*s = nil
		return nil
	}
	return json.Unmarshal(b, s)
}

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(s))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// User — таблица users (см. app/models/user.py).
type User struct {
	ID             int       `gorm:"primaryKey" json:"id"`
	Email          string    `gorm:"column:email" json:"email"`
	Phone          *string   `gorm:"column:phone" json:"phone"`
	HashedPassword string    `gorm:"column:hashed_password" json:"-"`
	FullName       string    `gorm:"column:full_name" json:"full_name"`
	Role           string    `gorm:"column:role" json:"role"`
	IsActive       bool      `gorm:"column:is_active" json:"is_active"`
	AvatarURL      *string `gorm:"column:avatar_url" json:"avatar_url"`
	DiscipleID     *int    `gorm:"column:disciple_id" json:"disciple_id"`
	CreatedAt      Time    `gorm:"column:created_at" json:"created_at"`
}

func (User) TableName() string { return "users" }

// Disciple — таблица disciples (полный набор колонок из app/models/disciple.py).
type Disciple struct {
	ID                 int        `gorm:"primaryKey" json:"id"`
	SpiritualName      *string    `gorm:"column:spiritual_name" json:"spiritual_name"`
	MaterialName       string     `gorm:"column:material_name" json:"material_name"`
	PhotoURL           *string    `gorm:"column:photo_url" json:"photo_url"`
	Phone              *string    `gorm:"column:phone" json:"phone"`
	Email              *string    `gorm:"column:email" json:"email"`
	Messenger          *string    `gorm:"column:messenger" json:"messenger"`
	Country            *string    `gorm:"column:country" json:"country"`
	Region             *string    `gorm:"column:region" json:"region"`
	City               *string    `gorm:"column:city" json:"city"`
	TempleID           *int       `gorm:"column:temple_id" json:"temple_id"`
	Gender             *string    `gorm:"column:gender" json:"gender"`
	MaritalStatus      *string    `gorm:"column:marital_status" json:"marital_status"`
	DateOfBirth        *time.Time `gorm:"column:date_of_birth" json:"-"`
	InitiationStatus   string     `gorm:"column:initiation_status" json:"initiation_status"`
	PranamaDate        *time.Time `gorm:"column:pranama_date" json:"-"`
	HarinamaDate       *time.Time `gorm:"column:harinama_date" json:"-"`
	HarinamaName       *string    `gorm:"column:harinama_name" json:"harinama_name"`
	BrahmanDate        *time.Time `gorm:"column:brahman_date" json:"-"`
	Seva               *string    `gorm:"column:seva" json:"seva"`
	CurrentActivity    *string    `gorm:"column:current_activity" json:"current_activity"`
	IsMentor           bool       `gorm:"column:is_mentor" json:"is_mentor"`
	MentorID           *int       `gorm:"column:mentor_id" json:"mentor_id"`
	MentorName         *string    `gorm:"column:mentor_name" json:"mentor_name"`
	RecommendedBy      *string    `gorm:"column:recommended_by" json:"recommended_by"`
	ApplicationDate    *time.Time `gorm:"column:application_date" json:"-"`
	ReadyForPranama    bool       `gorm:"column:ready_for_pranama" json:"ready_for_pranama"`
	ReadyForInitiation bool       `gorm:"column:ready_for_initiation" json:"ready_for_initiation"`
	IsApproved         bool       `gorm:"column:is_approved" json:"is_approved"`
	Notes              *string    `gorm:"column:notes" json:"notes"`
	CreatedAt          time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at" json:"updated_at"`

	Temple    *Temple         `gorm:"foreignKey:TempleID" json:"temple"`
	Mentor    *Disciple       `gorm:"foreignKey:MentorID" json:"-"`
	Checklist []ChecklistItem `gorm:"foreignKey:DiscipleID" json:"checklist"`
}

func (Disciple) TableName() string { return "disciples" }

// Name — духовное имя или мирское (property name в Python).
func (d *Disciple) Name() string {
	if d.SpiritualName != nil && *d.SpiritualName != "" {
		return *d.SpiritualName
	}
	return d.MaterialName
}

// ProfileFilled — заполнены обязательные поля анкеты.
func (d *Disciple) ProfileFilled() bool {
	return strings.TrimSpace(d.MaterialName) != "" &&
		d.Country != nil && *d.Country != "" &&
		d.City != nil && *d.City != "" &&
		d.DateOfBirth != nil && d.MaritalStatus != nil && *d.MaritalStatus != ""
}

// ChecklistItem — пункт чек-листа подготовки к инициации.
type ChecklistItem struct {
	ID         int       `gorm:"primaryKey" json:"id"`
	DiscipleID int       `gorm:"column:disciple_id" json:"disciple_id"`
	Title      string    `gorm:"column:title" json:"title"`
	IsDone     bool      `gorm:"column:is_done" json:"is_done"`
	Note       *string   `gorm:"column:note" json:"note"`
	Target     string    `gorm:"column:target" json:"target"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"-"`
}

func (ChecklistItem) TableName() string { return "checklist_items" }

// DiscipleNote — заметка куратора об ученике.
type DiscipleNote struct {
	ID         int       `gorm:"primaryKey" json:"id"`
	DiscipleID int       `gorm:"column:disciple_id" json:"disciple_id"`
	AuthorID   *int      `gorm:"column:author_id" json:"author_id"`
	Text       string    `gorm:"column:text" json:"text"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	Author     *User     `gorm:"foreignKey:AuthorID" json:"-"`
}

func (DiscipleNote) TableName() string { return "disciple_notes" }

// DiscipleFile — файл, прикреплённый к анкете.
type DiscipleFile struct {
	ID          int       `gorm:"primaryKey" json:"id"`
	DiscipleID  int       `gorm:"column:disciple_id" json:"disciple_id"`
	UploadedBy  *int      `gorm:"column:uploaded_by" json:"uploaded_by"`
	Name        string    `gorm:"column:name" json:"name"`
	URL         string    `gorm:"column:url" json:"url"`
	Size        *int      `gorm:"column:size" json:"size"`
	ContentType *string   `gorm:"column:content_type" json:"content_type"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	Uploader    *User     `gorm:"foreignKey:UploadedBy" json:"-"`
}

func (DiscipleFile) TableName() string { return "disciple_files" }

// Role — таблица roles (динамические роли с набором прав).
type Role struct {
	ID           int        `gorm:"primaryKey" json:"id"`
	Key          string     `gorm:"column:key" json:"key"`
	Name         string     `gorm:"column:name" json:"name"`
	IsSystem     bool       `gorm:"column:is_system" json:"is_system"`
	IsSuperadmin bool       `gorm:"column:is_superadmin" json:"is_superadmin"`
	IsDefault    bool       `gorm:"column:is_default" json:"is_default"`
	Capabilities StringList `gorm:"column:capabilities" json:"capabilities"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
}

func (Role) TableName() string { return "roles" }

// UserRole — связь пользователь ↔ роль.
type UserRole struct {
	ID     int `gorm:"primaryKey" json:"id"`
	UserID int `gorm:"column:user_id" json:"user_id"`
	RoleID int `gorm:"column:role_id" json:"role_id"`
}

func (UserRole) TableName() string { return "user_roles" }

// SmsCode — код подтверждения по телефону.
type SmsCode struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	Phone     string    `gorm:"column:phone" json:"phone"`
	Code      string    `gorm:"column:code" json:"code"`
	ExpiresAt time.Time `gorm:"column:expires_at" json:"expires_at"`
	Attempts  int       `gorm:"column:attempts" json:"attempts"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (SmsCode) TableName() string { return "sms_codes" }

// AppSetting — key-value настройки приложения.
type AppSetting struct {
	Key   string `gorm:"column:key;primaryKey" json:"key"`
	Value string `gorm:"column:value" json:"value"`
}

func (AppSetting) TableName() string { return "app_settings" }

// ── справочники ──────────────────────────────────────────────────────────

type City struct {
	ID      int     `gorm:"primaryKey" json:"id"`
	Name    string  `gorm:"column:name" json:"name"`
	Country *string `gorm:"column:country" json:"country"`
	Region  *string `gorm:"column:region" json:"region"`
}

func (City) TableName() string { return "cities" }

type Region struct {
	ID   int    `gorm:"primaryKey" json:"id"`
	Name string `gorm:"column:name" json:"name"`
}

func (Region) TableName() string { return "regions" }

type Country struct {
	ID   int    `gorm:"primaryKey" json:"id"`
	Name string `gorm:"column:name" json:"name"`
}

func (Country) TableName() string { return "countries" }

type Temple struct {
	ID            int     `gorm:"primaryKey" json:"id"`
	Name          string  `gorm:"column:name" json:"name"`
	City          *string `gorm:"column:city" json:"city"`
	Country       *string `gorm:"column:country" json:"country"`
	PresidentName *string `gorm:"column:president_name" json:"president_name"`
	Notes         *string `gorm:"column:notes" json:"notes"`
}

func (Temple) TableName() string { return "temples" }

// Thread — ветка общения (вопрос/отчёт/approval).
type Thread struct {
	ID          int        `gorm:"primaryKey" json:"id"`
	Kind        string     `gorm:"column:kind" json:"kind"`
	DiscipleID  int        `gorm:"column:disciple_id" json:"disciple_id"`
	Subject     *string    `gorm:"column:subject" json:"subject"`
	Period      *string    `gorm:"column:period" json:"period"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`
	StaffSeenAt *time.Time `gorm:"column:staff_seen_at" json:"-"`

	Disciple *Disciple       `gorm:"foreignKey:DiscipleID" json:"-"`
	Messages []ThreadMessage `gorm:"foreignKey:ThreadID" json:"-"`
}

func (Thread) TableName() string { return "threads" }

// ThreadMessage — сообщение в ветке.
type ThreadMessage struct {
	ID        int        `gorm:"primaryKey" json:"id"`
	ThreadID  int        `gorm:"column:thread_id" json:"thread_id"`
	AuthorID  *int       `gorm:"column:author_id" json:"author_id"`
	Body      string     `gorm:"column:body" json:"body"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	EditedAt  *time.Time `gorm:"column:edited_at" json:"-"`
	EditCount int        `gorm:"column:edit_count" json:"edit_count"`
	ReplyToID *int       `gorm:"column:reply_to_id" json:"reply_to_id"`

	Author  *User          `gorm:"foreignKey:AuthorID" json:"-"`
	ReplyTo *ThreadMessage `gorm:"foreignKey:ReplyToID" json:"-"`
	Likes   []MessageLike  `gorm:"foreignKey:MessageID" json:"-"`
}

func (ThreadMessage) TableName() string { return "thread_messages" }

// ThreadRead — отметка последнего прочтения ветки пользователем.
type ThreadRead struct {
	ID         int       `gorm:"primaryKey" json:"id"`
	ThreadID   int       `gorm:"column:thread_id" json:"thread_id"`
	UserID     int       `gorm:"column:user_id" json:"user_id"`
	LastSeenAt time.Time `gorm:"column:last_seen_at" json:"last_seen_at"`
}

func (ThreadRead) TableName() string { return "thread_reads" }

// MessageLike — реакция-эмодзи на сообщение ветки.
type MessageLike struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	MessageID int       `gorm:"column:message_id" json:"message_id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	Emoji     string    `gorm:"column:emoji" json:"emoji"`
	CreatedAt time.Time `gorm:"column:created_at" json:"-"`
}

func (MessageLike) TableName() string { return "message_likes" }

// ── минимальные модели для nav-counts (полные — в модулях forum/conferences) ──

type ForumSection struct {
	ID          int       `gorm:"primaryKey" json:"id"`
	Title       string    `gorm:"column:title" json:"title"`
	Description *string   `gorm:"column:description" json:"description"`
	Color       string    `gorm:"column:color" json:"color"`
	CoverURL    *string   `gorm:"column:cover_url" json:"cover_url"`
	AuthorID    *int      `gorm:"column:author_id" json:"author_id"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"-"`

	Author *User        `gorm:"foreignKey:AuthorID" json:"-"`
	Topics []ForumTopic `gorm:"foreignKey:SectionID" json:"-"`
}

func (ForumSection) TableName() string { return "forum_sections" }

type ForumTopic struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	SectionID *int      `gorm:"column:section_id" json:"section_id"`
	Title     string    `gorm:"column:title" json:"title"`
	AuthorID  *int      `gorm:"column:author_id" json:"author_id"`
	Pinned    bool      `gorm:"column:pinned" json:"pinned"`
	Views     int       `gorm:"column:views" json:"views"`
	CoverURL  *string   `gorm:"column:cover_url" json:"cover_url"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`

	Author  *User         `gorm:"foreignKey:AuthorID" json:"-"`
	Section *ForumSection `gorm:"foreignKey:SectionID" json:"-"`
	Posts   []ForumPost   `gorm:"foreignKey:TopicID" json:"-"`
}

func (ForumTopic) TableName() string { return "forum_topics" }

type ForumPost struct {
	ID        int        `gorm:"primaryKey" json:"id"`
	TopicID   int        `gorm:"column:topic_id" json:"topic_id"`
	AuthorID  *int       `gorm:"column:author_id" json:"author_id"`
	Body      string     `gorm:"column:body" json:"body"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	EditedAt  *time.Time `gorm:"column:edited_at" json:"-"`
	EditCount int        `gorm:"column:edit_count" json:"edit_count"`

	Author *User           `gorm:"foreignKey:AuthorID" json:"-"`
	Likes  []ForumPostLike `gorm:"foreignKey:PostID" json:"-"`
}

func (ForumPost) TableName() string { return "forum_posts" }

type ForumPostLike struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	PostID    int       `gorm:"column:post_id" json:"post_id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	Emoji     string    `gorm:"column:emoji" json:"emoji"`
	CreatedAt time.Time `gorm:"column:created_at" json:"-"`

	User *User `gorm:"foreignKey:UserID" json:"-"`
}

func (ForumPostLike) TableName() string { return "forum_post_likes" }

type ForumTopicRead struct {
	ID         int       `gorm:"primaryKey" json:"id"`
	TopicID    int       `gorm:"column:topic_id" json:"topic_id"`
	UserID     int       `gorm:"column:user_id" json:"user_id"`
	LastSeenAt time.Time `gorm:"column:last_seen_at" json:"last_seen_at"`
}

func (ForumTopicRead) TableName() string { return "forum_topic_reads" }

type Conference struct {
	ID            int        `gorm:"primaryKey" json:"id"`
	Title         string     `gorm:"column:title" json:"title"`
	Description   *string    `gorm:"column:description" json:"description"`
	Mode          string     `gorm:"column:mode" json:"mode"`
	MicAllowed    bool       `gorm:"column:mic_allowed" json:"mic_allowed"`
	CamAllowed    bool       `gorm:"column:cam_allowed" json:"cam_allowed"`
	ScreenAllowed bool       `gorm:"column:screen_allowed" json:"screen_allowed"`
	GuestsAllowed bool       `gorm:"column:guests_allowed" json:"guests_allowed"`
	AutoRecord    bool       `gorm:"column:auto_record" json:"auto_record"`
	Room          string     `gorm:"column:room" json:"room"`
	Code          *string    `gorm:"column:code" json:"code"`
	Status        string     `gorm:"column:status" json:"status"`
	HostID        *int       `gorm:"column:host_id" json:"host_id"`
	ScheduledAt   *time.Time `gorm:"column:scheduled_at" json:"-"`
	StartedAt     *time.Time `gorm:"column:started_at" json:"-"`
	EndedAt       *time.Time `gorm:"column:ended_at" json:"-"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"-"`

	Host *User `gorm:"foreignKey:HostID" json:"-"`
}

func (Conference) TableName() string { return "conferences" }

type ConferenceBan struct {
	ID           int       `gorm:"primaryKey" json:"id"`
	ConferenceID int       `gorm:"column:conference_id" json:"conference_id"`
	Identity     string    `gorm:"column:identity" json:"identity"`
	Name         *string   `gorm:"column:name" json:"name"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"-"`
}

func (ConferenceBan) TableName() string { return "conference_bans" }

type ConferenceRecording struct {
	ID           int        `gorm:"primaryKey" json:"id"`
	ConferenceID int        `gorm:"column:conference_id" json:"conference_id"`
	EgressID     *string    `gorm:"column:egress_id" json:"egress_id"`
	Filename     *string    `gorm:"column:filename" json:"filename"`
	Title        *string    `gorm:"column:title" json:"title"`
	Description  *string    `gorm:"column:description" json:"description"`
	Status       string     `gorm:"column:status" json:"status"`
	DurationMs   int64      `gorm:"column:duration_ms" json:"duration_ms"`
	SizeBytes    int64      `gorm:"column:size_bytes" json:"size_bytes"`
	StartedAt    time.Time  `gorm:"column:started_at" json:"-"`
	EndedAt      *time.Time `gorm:"column:ended_at" json:"-"`

	Conference *Conference `gorm:"foreignKey:ConferenceID" json:"-"`
}

func (ConferenceRecording) TableName() string { return "conference_recordings" }

// Event — событие календаря.
type Event struct {
	ID          int        `gorm:"primaryKey" json:"id"`
	Title       string     `gorm:"column:title" json:"title"`
	Location    *string    `gorm:"column:location" json:"location"`
	StartsOn    time.Time  `gorm:"column:starts_on" json:"-"`
	EndsOn      *time.Time `gorm:"column:ends_on" json:"-"`
	Description *string    `gorm:"column:description" json:"description"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"-"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"-"`
}

func (Event) TableName() string { return "events" }

// ── Мессенджер ──────────────────────────────────────────────────────────────

type Chat struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	Type      string    `gorm:"column:type" json:"type"`
	Title     *string   `gorm:"column:title" json:"title"`
	PhotoURL  *string   `gorm:"column:photo_url" json:"photo_url"`
	CreatedBy *int      `gorm:"column:created_by" json:"created_by"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`

	Members []ChatMember `gorm:"foreignKey:ChatID" json:"-"`
}

func (Chat) TableName() string { return "chats" }

type ChatMember struct {
	ID          int       `gorm:"primaryKey" json:"id"`
	ChatID      int       `gorm:"column:chat_id" json:"chat_id"`
	UserID      int       `gorm:"column:user_id" json:"user_id"`
	Role        string    `gorm:"column:role" json:"role"`
	Pinned      bool      `gorm:"column:pinned" json:"pinned"`
	LastReadSeq int64     `gorm:"column:last_read_seq" json:"last_read_seq"`
	JoinedAt    time.Time `gorm:"column:joined_at" json:"-"`

	User *User `gorm:"foreignKey:UserID" json:"-"`
}

func (ChatMember) TableName() string { return "chat_members" }

type ChatMessage struct {
	ID         int        `gorm:"primaryKey" json:"id"`
	ChatID     int        `gorm:"column:chat_id" json:"chat_id"`
	Seq        int64      `gorm:"column:seq" json:"seq"`
	ClientUUID *string    `gorm:"column:client_uuid" json:"client_uuid"`
	AuthorID   *int       `gorm:"column:author_id" json:"author_id"`
	Body       string     `gorm:"column:body" json:"body"`
	ReplyToID  *int       `gorm:"column:reply_to_id" json:"reply_to_id"`
	ReplyQuote *string    `gorm:"column:reply_quote" json:"reply_quote"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
	EditedAt   *time.Time `gorm:"column:edited_at" json:"-"`
	EditCount  int        `gorm:"column:edit_count" json:"edit_count"`
	Deleted    bool       `gorm:"column:deleted" json:"deleted"`

	Author    *User                 `gorm:"foreignKey:AuthorID" json:"-"`
	ReplyTo   *ChatMessage          `gorm:"foreignKey:ReplyToID" json:"-"`
	Reactions []ChatMessageReaction `gorm:"foreignKey:MessageID" json:"-"`
}

func (ChatMessage) TableName() string { return "chat_messages" }

type ChatMessageReaction struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	MessageID int       `gorm:"column:message_id" json:"message_id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	Emoji     string    `gorm:"column:emoji" json:"emoji"`
	CreatedAt time.Time `gorm:"column:created_at" json:"-"`

	User *User `gorm:"foreignKey:UserID" json:"-"`
}

func (ChatMessageReaction) TableName() string { return "chat_message_reactions" }

// Draft — черновик автосохранения текста пользователя.
type Draft struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	Scope     string    `gorm:"column:scope" json:"scope"`
	Body      string    `gorm:"column:body" json:"body"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"-"`
}

func (Draft) TableName() string { return "drafts" }
