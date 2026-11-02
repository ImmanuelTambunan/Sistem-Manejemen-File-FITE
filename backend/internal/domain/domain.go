package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const (
	RoleSuperAdmin = "super_admin"
	RoleDekan       = "dekan"
	RoleBaak        = "baak"
	RoleKaprodi     = "kaprodi"
	RoleDosen       = "dosen"
	RoleMahasiswa   = "mahasiswa"
)

var AllRoles = []string{RoleSuperAdmin, RoleDekan, RoleBaak, RoleKaprodi, RoleDosen, RoleMahasiswa}

// StringList represents a list of strings stored as JSON in MySQL.
type StringList []string

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		s = StringList{}
	}
	b, err := json.Marshal([]string(s))
	return string(b), err
}

func (s *StringList) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*s = StringList{}
		return nil
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	}
	return fmt.Errorf("StringList: tipe %T tidak didukung", src)
}

func (s StringList) Contains(v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// JSONMap represents a JSON object stored in MySQL.
type JSONMap map[string]any

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(map[string]any(m))
	return string(b), err
}

func (m *JSONMap) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*m = nil
		return nil
	case []byte:
		return json.Unmarshal(v, m)
	case string:
		return json.Unmarshal([]byte(v), m)
	}
	return fmt.Errorf("JSONMap: tipe %T tidak didukung", src)
}

// Models

type Role struct {
	ID        uint      `gorm:"primaryKey"`
	Key       string    `gorm:"column:key;size:30;uniqueIndex"`
	Name      string    `gorm:"size:50"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Prodi struct {
	ID        uint      `gorm:"primaryKey"`
	Code      string    `gorm:"size:5;uniqueIndex"`
	Name      string    `gorm:"size:100"`
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Prodi) TableName() string { return "prodi" }

type User struct {
	ID                 uint           `gorm:"primaryKey"`
	Name               string         `gorm:"size:100"`
	Email              string         `gorm:"size:255;uniqueIndex"`
	PasswordHash       string         `gorm:"column:password;size:255"`
	RoleID             uint
	Role               Role
	ProdiID            *uint
	Prodi              *Prodi
	NIM                *string        `gorm:"column:nim;size:30"`
	NIP                *string        `gorm:"column:nip;size:30"`
	Phone              *string        `gorm:"size:20"`
	IsActive           bool
	MustChangePassword bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

type AuthSession struct {
	ID                  string     `gorm:"primaryKey;type:char(36)"`
	UserID              uint       `gorm:"index"`
	RefreshTokenHash    string     `gorm:"size:64;uniqueIndex"`
	PreviousRefreshHash *string    `gorm:"size:64;index"`
	UserAgent           string     `gorm:"size:255"`
	IP                  string     `gorm:"column:ip;size:45"`
	ExpiresAt           time.Time
	AbsoluteExpiresAt   time.Time
	LastUsedAt          *time.Time
	RevokedAt           *time.Time
	CreatedAt           time.Time
}

type Category struct {
	ID            uint       `gorm:"primaryKey"`
	Name          string     `gorm:"size:100;uniqueIndex"`
	AllowedRoles  StringList `gorm:"type:json"`
	IsOperational bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Document struct {
	ID                 uint           `gorm:"primaryKey"`
	Title              string         `gorm:"size:255"`
	Description        *string        `gorm:"type:text"`
	OriginalName       string         `gorm:"size:255"`
	Extension          string         `gorm:"size:10"`
	MimeType           string         `gorm:"size:100"`
	SizeKB             uint           `gorm:"column:size_kb"`
	StoragePath        string         `gorm:"size:500;uniqueIndex" json:"-"`
	CategoryID         uint
	Category           Category
	ProdiID            *uint
	Prodi              *Prodi
	UploaderID         uint
	Uploader           User           `gorm:"foreignKey:UploaderID"`
	OwnerID            uint
	Owner              User           `gorm:"foreignKey:OwnerID"`
	OwnerRole          string         `gorm:"size:30"`
	Visibility         string         `gorm:"size:10"`
	VerificationStatus *string        `gorm:"size:10"`
	ArchivedAt         *time.Time
	RetentionStatus    *string        `gorm:"size:10"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

type Verification struct {
	ID          uint      `gorm:"primaryKey"`
	DocumentID  uint      `gorm:"uniqueIndex"`
	Document    Document
	Status      string    `gorm:"size:10"`
	Reason      *string   `gorm:"size:500"`
	RequestedBy uint
	Requester   User      `gorm:"foreignKey:RequestedBy"`
	VerifiedBy  *uint
	Verifier    *User     `gorm:"foreignKey:VerifiedBy"`
	RequestedAt time.Time
	VerifiedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Verification) TableName() string { return "document_verifications" }

type Notification struct {
	ID        string    `gorm:"primaryKey;type:char(36)"`
	UserID    uint      `gorm:"index"`
	Type      string    `gorm:"size:50"`
	Title     string    `gorm:"size:150"`
	Message   string    `gorm:"type:text"`
	Data      JSONMap   `gorm:"type:json"`
	IsRead    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type StorageLog struct {
	ID         uint      `gorm:"primaryKey"`
	DocumentID *uint
	UserID     *uint
	Action     string    `gorm:"size:40"`
	Meta       JSONMap   `gorm:"type:json"`
	CreatedAt  time.Time
}

