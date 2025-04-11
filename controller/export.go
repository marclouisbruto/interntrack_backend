package controller

import (
	"bytes"
	"fmt"
	"intern_template_v1/middleware"
	"intern_template_v1/model"

	"github.com/gofiber/fiber/v2"
	"github.com/jung-kurt/gofpdf"
)

// Struct for exporting intern data
// Struct for exporting intern data
type ExportIntern struct {
	ID             int
	CustomInternID string
	FirstName      string
	MiddleName     string
	LastName       string
	SuffixName     string
	Email          string
	SupervisorID   *int
	HandlerID      *int
	SchoolName     string
	Course         string
	PhoneNumber    string
}

// Helper function to format full name
func formatFullName(f, m, l, s string) string {
	full := f
	if m != "" {
		full += " " + m
	}
	full += " " + l
	if s != "" {
		full += " " + s
	}
	return full
}

// Export interns data to PDF
func ExportDataToPDF(c *fiber.Ctx) error {
	var interns []ExportIntern

	// Update SELECT query to include school_name, course, phone_number
	err := middleware.DBConn.Table("users").
		Select(`users.id AS id, interns.custom_intern_id, users.first_name, users.middle_name, users.last_name, 
	        users.suffix_name, users.email, interns.supervisor_id, handlers.id AS handler_id,
			interns.school_name, interns.course, users.phone_number`).
		Joins("JOIN interns ON users.id = interns.id").
		Joins("LEFT JOIN handlers ON interns.handler_id = handlers.id").
		Where("users.role_id = ? AND interns.custom_intern_id IS NOT NULL AND interns.custom_intern_id != ''", 2).
		Order("users.id ASC").
		Scan(&interns).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to fetch interns: %v", err))
	}

	// Maps
	supervisorMap := make(map[int]string)
	handlerMap := make(map[int]string)

	// Supervisor names
	var supervisors []struct {
		UserID int `gorm:"column:id"`
	}
	if err := middleware.DBConn.Table("supervisors").Select("id").Find(&supervisors).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error retrieving supervisors")
	}
	for _, s := range supervisors {
		var user model.User
		if err := middleware.DBConn.Table("users").Where("id = ?", s.UserID).First(&user).Error; err == nil {
			supervisorMap[s.UserID] = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}
	}

	// Handler names
	var handlers []struct {
		ID     int
		UserID int
	}
	if err := middleware.DBConn.Table("handlers").Select("id, user_id").Find(&handlers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error retrieving handlers")
	}
	for _, h := range handlers {
		var user model.User
		if err := middleware.DBConn.Table("users").Where("id = ?", h.UserID).First(&user).Error; err == nil {
			handlerMap[h.ID] = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		}
	}

	// Create PDF
	pdf := gofpdf.New("L", "mm", "Legal", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 10, "List of Interns", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	// Headers
	headers := []string{"ID", "Custom ID", "Full Name", "Email", "School Name", "Course", "Phone", "Supervisor/Handler"}

	// Prepare data rows
	dataRows := [][]string{}
	pdf.SetFont("Arial", "", 11)

	for _, intern := range interns {
		fullName := formatFullName(intern.FirstName, intern.MiddleName, intern.LastName, intern.SuffixName)

		assignedName := "N/A"
		if intern.HandlerID != nil {
			if name, ok := handlerMap[*intern.HandlerID]; ok {
				assignedName = name
			}
		} else if intern.SupervisorID != nil {
			if name, ok := supervisorMap[*intern.SupervisorID]; ok {
				assignedName = name
			}
		}

		row := []string{
			fmt.Sprintf("%d", intern.ID),
			intern.CustomInternID,
			fullName,
			intern.Email,
			intern.SchoolName,
			intern.Course,
			intern.PhoneNumber,
			assignedName,
		}
		dataRows = append(dataRows, row)
	}

	// Calculate max width per column
	pdf.SetFont("Arial", "", 11)
	colWidths := make([]float64, len(headers))
	for i, header := range headers {
		colWidths[i] = pdf.GetStringWidth(header) + 6 // Add some padding
	}

	for _, row := range dataRows {
		for i, cell := range row {
			w := pdf.GetStringWidth(cell) + 6
			if w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	// Table Headers
	pdf.SetFont("Arial", "B", 12)
	pdf.SetFillColor(200, 200, 200)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 10, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Table Rows
	pdf.SetFont("Arial", "", 11)
	for _, row := range dataRows {
		for i, data := range row {
			align := "L"
			if i == 0 {
				align = "C"
			}
			pdf.CellFormat(colWidths[i], 10, data, "1", 0, align, false, 0, "")
		}
		pdf.Ln(-1)
	}

	// Output PDF to buffer
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to generate PDF: %v", err))
	}

	// Return as downloadable PDF
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", "attachment; filename=intern_list.pdf")
	return c.Send(buf.Bytes())
}
