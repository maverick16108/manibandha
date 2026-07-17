package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

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
	AvatarURL      *string   `gorm:"column:avatar_url" json:"avatar_url"`
	DiscipleID     *int      `gorm:"column:disciple_id" json:"disciple_id"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
}

func (User) TableName() string { return "users" }

// Disciple — таблица disciples (минимальный набор колонок NOT NULL без server default,
// чтобы корректно создавать анкету при регистрации).
type Disciple struct {
	ID                 int     `gorm:"primaryKey" json:"id"`
	SpiritualName      *string `gorm:"column:spiritual_name" json:"spiritual_name"`
	MaterialName       string  `gorm:"column:material_name" json:"material_name"`
	PhotoURL           *string `gorm:"column:photo_url" json:"photo_url"`
	Phone              *string `gorm:"column:phone" json:"phone"`
	Email              *string `gorm:"column:email" json:"email"`
	Messenger          *string `gorm:"column:messenger" json:"messenger"`
	MaritalStatus      *string `gorm:"column:marital_status" json:"marital_status"`
	InitiationStatus   string  `gorm:"column:initiation_status" json:"initiation_status"`
	IsMentor           bool    `gorm:"column:is_mentor" json:"is_mentor"`
	ReadyForPranama    bool    `gorm:"column:ready_for_pranama" json:"ready_for_pranama"`
	ReadyForInitiation bool    `gorm:"column:ready_for_initiation" json:"ready_for_initiation"`
	IsApproved         bool    `gorm:"column:is_approved" json:"is_approved"`
}

func (Disciple) TableName() string { return "disciples" }

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

// Thread — ветка общения (нужна при регистрации: создаётся approval-ветка).
type Thread struct {
	ID         int     `gorm:"primaryKey" json:"id"`
	Kind       string  `gorm:"column:kind" json:"kind"`
	DiscipleID int     `gorm:"column:disciple_id" json:"disciple_id"`
	Subject    *string `gorm:"column:subject" json:"subject"`
	Period     *string `gorm:"column:period" json:"period"`
}

func (Thread) TableName() string { return "threads" }
