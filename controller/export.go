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
type InternInfo struct {
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
	var interns []InternInfo

	// Fetch interns
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

	// Maps for names and departments
	supervisorMap := make(map[int]struct {
		Name       string
		Department string
	})
	handlerMap := make(map[int]struct {
		Name       string
		Department string
	})

	// Fetch supervisors
	var supervisors []struct {
		UserID     int `gorm:"column:id"`
		Department string
	}
	if err := middleware.DBConn.Table("supervisors").Select("id, department").Find(&supervisors).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error retrieving supervisors")
	}
	for _, s := range supervisors {
		var user model.User
		if err := middleware.DBConn.Table("users").Where("id = ?", s.UserID).First(&user).Error; err == nil {
			supervisorMap[s.UserID] = struct {
				Name       string
				Department string
			}{
				Name:       fmt.Sprintf("%s %s", user.FirstName, user.LastName),
				Department: s.Department,
			}
		}
	}

	// Fetch handlers
	var handlers []struct {
		ID         int
		UserID     int
		Department string
	}
	if err := middleware.DBConn.Table("handlers").Select("id, user_id, department").Find(&handlers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error retrieving handlers")
	}
	for _, h := range handlers {
		var user model.User
		if err := middleware.DBConn.Table("users").Where("id = ?", h.UserID).First(&user).Error; err == nil {
			handlerMap[h.ID] = struct {
				Name       string
				Department string
			}{
				Name:       fmt.Sprintf("%s %s", user.FirstName, user.LastName),
				Department: h.Department,
			}
		}
	}

	// Create PDF
	pdf := gofpdf.New("L", "mm", "Legal", "")
	pdf.SetFont("Arial", "", 11)

	// Table headers
	headers := []string{"No.", "Custom ID", "Full Name", "Email", "School Name", "Course", "Phone", "Supervisor/Handler", "Department"}

	// Prepare data rows
	dataRows := [][]string{}
	for i, intern := range interns {
		fullName := formatFullName(intern.FirstName, intern.MiddleName, intern.LastName, intern.SuffixName)

		assignedName := "N/A"
		assignedDept := "N/A"

		if intern.HandlerID != nil {
			if info, ok := handlerMap[*intern.HandlerID]; ok {
				assignedName = info.Name
				assignedDept = info.Department
			}
		} else if intern.SupervisorID != nil {
			if info, ok := supervisorMap[*intern.SupervisorID]; ok {
				assignedName = info.Name
				assignedDept = info.Department
			}
		}

		row := []string{
			fmt.Sprintf("%d", i+1),
			intern.CustomInternID,
			fullName,
			intern.Email,
			intern.SchoolName,
			intern.Course,
			intern.PhoneNumber,
			assignedName,
			assignedDept,
		}
		dataRows = append(dataRows, row)
	}

	// Column widths
	pdf.SetFont("Arial", "", 11)
	colWidths := make([]float64, len(headers))
	for i, header := range headers {
		colWidths[i] = pdf.GetStringWidth(header) + 6
	}
	for _, row := range dataRows {
		for i, cell := range row {
			w := pdf.GetStringWidth(cell) + 6
			if w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	// Total width of the table for centering
	totalTableWidth := 0.0
	for _, w := range colWidths {
		totalTableWidth += w
	}

	const internsPerPage = 20
	totalPages := (len(dataRows)-1)/internsPerPage + 1
	rowIndex := 0

	for page := 0; page < totalPages; page++ {
		pdf.AddPage()

		// Title
		pdf.SetFont("Arial", "B", 12)
		pdf.CellFormat(0, 10, "List of Interns", "", 1, "C", false, 0, "")
		pdf.Ln(3)

		// Centering start X
		pageWidth, _ := pdf.GetPageSize()
		startX := (pageWidth - totalTableWidth) / 2

		// Table Headers
		pdf.SetFont("Arial", "B", 11)
		pdf.SetFillColor(200, 200, 200)
		pdf.SetX(startX)
		for i, h := range headers {
			pdf.CellFormat(colWidths[i], 10, h, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)

		// Table Rows
		pdf.SetFont("Arial", "", 10)
		for count := 0; count < internsPerPage && rowIndex < len(dataRows); count++ {
			row := dataRows[rowIndex]
			pdf.SetX(startX)
			for i, data := range row {
				align := "L"
				rowHeight := 8.1

				if i == 0 {
					align = "C"
					pdf.SetFont("Arial", "", 8) // small font for "No."
					pdf.CellFormat(colWidths[i], rowHeight, data, "1", 0, align, false, 0, "")
					pdf.SetFont("Arial", "", 10) // reset
				} else {
					pdf.CellFormat(colWidths[i], rowHeight, data, "1", 0, align, false, 0, "")
				}
			}
			pdf.Ln(-1)
			rowIndex++
		}
	}

	// Output
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to generate PDF: %v", err))
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", "attachment; filename=intern_list.pdf")
	return c.Send(buf.Bytes())
}

// Export intern attendance data to PDF
func ExportInternAttendanceToPDF(c *fiber.Ctx) error {
	type AttendanceInfo struct {
		CustomInternID string
		SupervisorID   *int
		HandlerID      *int
		Month          string
		TimeInAM       *string
		TimeOutAM      *string
		TimeInPM       *string
		TimeOutPM      *string
	}

	var records []AttendanceInfo

	err := middleware.DBConn.Table("dtr_entries").
		Select(`interns.custom_intern_id, interns.supervisor_id, interns.handler_id, dtr_entries.month, 
			dtr_entries.time_in_am, dtr_entries.time_out_am, dtr_entries.time_in_pm, dtr_entries.time_out_pm`).
		Joins("JOIN interns ON dtr_entries.intern_id = interns.id").
		Where("interns.custom_intern_id != ''").
		Order("dtr_entries.month ASC").
		Scan(&records).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to fetch attendance: %v", err))
	}

	supervisorMap := make(map[int]string)
	handlerMap := make(map[int]string)

	var supervisors []struct {
		UserID int `gorm:"column:id"`
	}
	if err := middleware.DBConn.Table("supervisors").Select("id").Find(&supervisors).Error; err == nil {
		for _, s := range supervisors {
			var user model.User
			if err := middleware.DBConn.Table("users").Where("id = ?", s.UserID).First(&user).Error; err == nil {
				supervisorMap[s.UserID] = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
			}
		}
	}

	var handlers []struct {
		ID     int
		UserID int
	}
	if err := middleware.DBConn.Table("handlers").Select("id, user_id").Find(&handlers).Error; err == nil {
		for _, h := range handlers {
			var user model.User
			if err := middleware.DBConn.Table("users").Where("id = ?", h.UserID).First(&user).Error; err == nil {
				handlerMap[h.ID] = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
			}
		}
	}

	pdf := gofpdf.New("L", "mm", "Legal", "")
	pdf.SetFont("Arial", "", 11)

	headers := []string{"Custom ID", "Supervisor/Handler", "Month", "Time In AM", "Time Out AM", "Time In PM", "Time Out PM", "Remarks"}
	colWidths := []float64{35, 50, 25, 30, 30, 30, 30, 35}

	totalTableWidth := 0.0
	for _, w := range colWidths {
		totalTableWidth += w
	}

	dataRows := [][]string{}
	for _, r := range records {
		assigned := "N/A"
		if r.HandlerID != nil {
			if val, ok := handlerMap[*r.HandlerID]; ok {
				assigned = val
			}
		} else if r.SupervisorID != nil {
			if val, ok := supervisorMap[*r.SupervisorID]; ok {
				assigned = val
			}
		}

		getTime := func(t *string) string {
			if t == nil || *t == "" {
				return "-"
			}
			return *t
		}

		timeAMIn := getTime(r.TimeInAM)
		timeAMOut := getTime(r.TimeOutAM)
		timePMIn := getTime(r.TimeInPM)
		timePMOut := getTime(r.TimeOutPM)

		remarks := "Absent"
		isAMFilled := timeAMIn != "-" && timeAMOut != "-"
		isPMFilled := timePMIn != "-" && timePMOut != "-"

		if isAMFilled && isPMFilled {
			remarks = "Present"
		} else if isAMFilled && !isPMFilled {
			remarks = "Half-Day-AM"
		} else if !isAMFilled && isPMFilled {
			remarks = "Half-Day-PM"
		}

		row := []string{
			r.CustomInternID,
			assigned,
			r.Month,
			timeAMIn,
			timeAMOut,
			timePMIn,
			timePMOut,
			remarks,
		}
		dataRows = append(dataRows, row)
	}

	const rowsPerPage = 20
	totalPages := (len(dataRows)-1)/rowsPerPage + 1
	rowIndex := 0

	for page := 0; page < totalPages; page++ {
		pdf.AddPage()
		pageWidth, _ := pdf.GetPageSize()
		marginX := (pageWidth - totalTableWidth) / 2
		pdf.SetX(marginX)

		pdf.SetFont("Arial", "B", 12)
		pdf.CellFormat(totalTableWidth, 10, "Intern Attendance Records", "", 1, "C", false, 0, "")
		pdf.Ln(3)
		pdf.SetX(marginX)

		pdf.SetFont("Arial", "B", 11)
		pdf.SetFillColor(220, 220, 220)
		for i, h := range headers {
			pdf.CellFormat(colWidths[i], 10, h, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)

		pdf.SetFont("Arial", "", 10)
		for count := 0; count < rowsPerPage && rowIndex < len(dataRows); count++ {
			row := dataRows[rowIndex]
			pdf.SetX(marginX)
			for i, cell := range row {
				align := "L"
				if i >= 2 && i <= 6 || cell == "-" || i == 7 {
					align = "C"
				}
				pdf.CellFormat(colWidths[i], 8, cell, "1", 0, align, false, 0, "")
			}
			pdf.Ln(-1)
			rowIndex++
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to generate PDF: %v", err))
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", "attachment; filename=intern_attendance.pdf")
	return c.Send(buf.Bytes())
}
