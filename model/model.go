package model

import "gorm.io/gorm"

type (
	SampleModel struct {
		Name string `json:"name"`
	}
)

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

type Intern struct {
	gorm.Model
	UserID           uint   `json:"user_id"`
	StudentID        string `json:"student_id"`
	SchoolName       string `json:"school_name"`
	SupervisorID     uint   `json:"supervisor_id"`
	Course           string `json:"course"`
	OjtHoursRequired int    `json:"ojt_hours_required"`
	OjtHoursRendered int    `json:"ojt_hours_rendered"`
	Status           string `json:"status"`
	Address          string `json:"address"`

	User       User       `gorm:"foreignKey:UserID"`
	Supervisor Supervisor `gorm:"foreignKey:SupervisorID"`
	QRCodes    []QRCode   `gorm:"foreignKey:InternID"`
	DtrEntries []DTREntry `gorm:"foreignKey:InternID"`
}

type QRCode struct {
	gorm.Model
	InternID uint   `json:"intern_id"`
	QRCode   string `json:"qrcode"`

	Intern Intern `gorm:"foreignKey:InternID"`
}

type DTREntry struct {
	gorm.Model
	UserID       uint    `json:"user_id"`
	InternID     uint    `json:"intern_id"`
	SupervisorID uint    `json:"supervisor_id"`
	Month        string  `json:"month"`
	TimeInAM     string  `json:"time_in_am"`
	TimeOutAM    string  `json:"time_out_am"`
	TimeInPM     string  `json:"time_in_pm"`
	TimeOutPM    string  `json:"time_out_pm"`
	TotalHours   float64 `json:"total_hours"`

	Intern     Intern     `gorm:"foreignKey:InternID"`
	Supervisor Supervisor `gorm:"foreignKey:SupervisorID"`
}

type LeaveRequest struct {
	gorm.Model
	InternID     uint   `json:"intern_id" gorm:"not null;constraint:OnDelete:CASCADE"`
	Reason       string `json:"reason" gorm:"type:text;not null"`
	ExcuseLetter string `json:"excuse_letter,omitempty"` // File path or URL (optional)
	Status       string `json:"status" gorm:"type:varchar(20);default:'Pending'"`

	Intern Intern `gorm:"foreignKey:InternID"` // Relationship to Interns table
}
