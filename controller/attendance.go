package controller

import (
	"github.com/gofiber/fiber/v2"
	"intern_template_v1/middleware"
)

type AttendanceSummary struct {
	CustomInternID string  `json:"custom_intern_id"`
	FirstName      string  `json:"first_name"`
	LastName       string  `json:"last_name"`
	SupervisorID   *int    `json:"supervisor_id"`
	HandlerID      *int    `json:"handler_id"`
	TimeInAM       *string `json:"time_in_am"`
	TimeOutAM      *string `json:"time_out_am"`
	TimeInPM       *string `json:"time_in_pm"`
	TimeOutPM      *string `json:"time_out_pm"`
	Month          string  `json:"month"`
	Status         string  `json:"status"` // Present, Half-Day-AM, Half-Day-PM, Absent
}

func GetInternAttendanceSummary(c *fiber.Ctx) error {
	date := c.Params("date") // Format: MM-DD-YY

	var records []AttendanceSummary

	// Fetch attendance records for the given date
	err := middleware.DBConn.Table("dtr_entries").
		Select("interns.custom_intern_id, users.first_name, users.last_name, interns.supervisor_id, interns.handler_id, dtr_entries.time_in_am, dtr_entries.time_out_am, dtr_entries.time_in_pm, dtr_entries.time_out_pm, dtr_entries.month").
		Joins("JOIN interns ON dtr_entries.intern_id = interns.id").
		Joins("JOIN users ON users.id = interns.id").
		Where("dtr_entries.month = ?", date).
		Scan(&records).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch attendance",
		})
	}

	var present []AttendanceSummary
	var halfDayAM []AttendanceSummary
	var halfDayPM []AttendanceSummary
	var absent []AttendanceSummary

	for _, record := range records {
		timeAMIn := record.TimeInAM != nil && *record.TimeInAM != ""
		timeAMOut := record.TimeOutAM != nil && *record.TimeOutAM != ""
		timePMIn := record.TimeInPM != nil && *record.TimeInPM != ""
		timePMOut := record.TimeOutPM != nil && *record.TimeOutPM != ""

		if timeAMIn && timeAMOut && timePMIn && timePMOut {
			record.Status = "Present"
			present = append(present, record)
		} else if timeAMIn && timeAMOut && !timePMIn && !timePMOut {
			record.Status = "Half-Day-AM"
			halfDayAM = append(halfDayAM, record)
		} else if !timeAMIn && !timeAMOut && timePMIn && timePMOut {
			record.Status = "Half-Day-PM"
			halfDayPM = append(halfDayPM, record)
		} else {
			record.Status = "Absent"
			absent = append(absent, record)
		}
	}

	return c.JSON(fiber.Map{
		"date":              date,
		"present":           present,
		"half_day_am":       halfDayAM,
		"half_day_pm":       halfDayPM,
		"absent":            absent,
		"total_records":     len(records),
		"total_present":     len(present),
		"total_half_day_am": len(halfDayAM),
		"total_half_day_pm": len(halfDayPM),
		"total_absent":      len(absent),
	})
}
