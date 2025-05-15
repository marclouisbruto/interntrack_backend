package model

import (
	"gorm.io/gorm"
)

type TokenRequest struct {
	gorm.Model
	InternID string `json:"intern_id"`
	Token    string `json:"token"`

	Intern Intern `gorm:"foreignKey:InternID"`
}

type Role struct {
	gorm.Model
	RoleName string `json:"role_name"`
	User     []User `gorm:"foreignKey:RoleID"`
}

type User struct {
	gorm.Model
	FirstName       string `json:"first_name"`
	MiddleName      string `json:"middle_name"`
	LastName        string `json:"last_name"`
	SuffixName      string `json:"suffix_name"`
	PhoneNumber     string `json:"phone_number"`
	Email           string `gorm:"unique" json:"email"`
	Password        string `json:"password"`
	RoleID          uint   `json:"role_id"`
	Role            Role   `gorm:"foreignKey:RoleID"`
	ConfirmPassword string `json:"confirm_password" gorm:"-"`

	Supervisor *Supervisor `gorm:"foreignKey:UserID"`
	Intern     *Intern     `gorm:"foreignKey:UserID"`
}

type Supervisor struct {
	gorm.Model
	UserID     uint   `json:"user_id"`
	Department string `json:"department"`
	Status     string `json:"status"`

	User User `gorm:"foreignKey:UserID"`
}

type Handler struct {
	gorm.Model
	UserID       uint   `json:"user_id"`
	SupervisorID uint   `json:"supervisor_id"`
	Department   string `json:"department"`
	Status       string `json:"status"`

	Supervisor Supervisor `gorm:"foreignKey:SupervisorID"`
	User       User       `gorm:"foreignKey:UserID"`
}

type Intern struct {
	gorm.Model
	CustomInternID   *string `json:"custom_intern_id" gorm:"unique;default:null"`
	UserID           uint    `json:"user_id"`
	ProfilePicture   string  `json:"profile_picture"`
	StudentID        string  `json:"student_id"`
	SchoolName       string  `json:"school_name"`
	SupervisorID     uint    `json:"supervisor_id"`
	HandlerID        *uint   `json:"handler_id"`
	Course           string  `json:"course"`
	OjtHoursRequired int     `json:"ojt_hours_required"`
	OjtHoursRendered string  `json:"ojt_hours_rendered,omitempty" gorm:"default:0"`
	Status           string  `json:"status"`
	Address          string  `json:"address"`

	User       User       `gorm:"foreignKey:UserID"`
	Supervisor Supervisor `gorm:"foreignKey:SupervisorID"`
	Handler    Handler    `gorm:"foreignKey:HandlerID"`
	QRCodes    []QRCode   `gorm:"foreignKey:InternID"`
	DtrEntries []DTREntry `gorm:"foreignKey:InternID"`
}

type QRCode struct {
	gorm.Model
	ID           uint   `gorm:"primaryKey"`
	InternID     uint   `gorm:"index"`
	QRCode       string `gorm:"type:text"` // This will store the QR code content
	Base64QRCode string `gorm:"type:text"` // This will store the base64 QR code

	Intern Intern `gorm:"foreignKey:InternID"`
}

type DTREntry struct {
	gorm.Model
	UserID       uint   `json:"user_id"`
	InternID     uint   `json:"intern_id"`
	SupervisorID uint   `json:"supervisor_id"`
	Month        string `json:"month"`
	TimeInAM     string `json:"time_in_am"`
	TimeOutAM    string `json:"time_out_am"`
	TimeInPM     string `json:"time_in_pm"`
	TimeOutPM    string `json:"time_out_pm"`
	TotalHours   string `json:"total_hours"`

	Intern     Intern     `gorm:"foreignKey:InternID"`
	Supervisor Supervisor `gorm:"foreignKey:SupervisorID"`
}

type LeaveRequest struct {
	gorm.Model
	InternID     uint   `json:"intern_id" gorm:"not null;constraint:OnDelete:CASCADE"`
	LeaveDate    string `json:"leave_date"`
	LeaveHours   string `json:"leave_hours"`
	Reason       string `json:"reason" gorm:"type:text;not null"`
	Status       string `json:"status" gorm:"type:varchar(20);default:'Pending'"`
	ExcuseLetter string `json:"excuse_letter,omitempty"` // File path or URL (optional)
	OnDayReason  string `json:"On_Day_Reason"`

	Intern Intern `gorm:"foreignKey:InternID"` // Relationship to Interns table
}
