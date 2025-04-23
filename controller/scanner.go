package controller

import (
	"fmt"
	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// DefaultTime sets default TimeInAM for interns
func DefaultTime(c *fiber.Ctx) error {
	type RequestBody struct {
		Interns []struct {
			User struct {
				FirstName  string `json:"first_name"`
				MiddleName string `json:"middle_name"`
				LastName   string `json:"last_name"`
				SuffixName string `json:"suffix_name"`
			} `json:"user"`
			Intern model.Intern `json:"intern"`
		} `json:"interns"`
	}

	var req RequestBody

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid JSON format",
			"error":   err.Error(),
		})
	}

	defaultTimeInAM := "08:00:00"
	loc, _ := time.LoadLocation("Asia/Manila")
	currentDate := time.Now().In(loc).Format("2006-01-02")
	var processedInterns []fiber.Map

	for _, intern := range req.Interns {
		if intern.Intern.ID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Intern ID is required",
			})
		}

		var internData model.Intern
		if err := middleware.DBConn.Preload("User").First(&internData, intern.Intern.ID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Intern not found",
				"error":   err.Error(),
			})
		}

		if internData.SupervisorID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": fmt.Sprintf("Supervisor ID missing for intern ID %d", intern.Intern.ID),
			})
		}

		if intern.User.FirstName != internData.User.FirstName ||
			intern.User.MiddleName != internData.User.MiddleName ||
			intern.User.LastName != internData.User.LastName ||
			intern.User.SuffixName != internData.User.SuffixName {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Provided data does not match the database values",
				"error":   "Name or suffix mismatch",
			})
		}

		var existingEntry model.DTREntry
		if err := middleware.DBConn.
			Where("intern_id = ? AND DATE(created_at) = ?", intern.Intern.ID, currentDate).
			First(&existingEntry).Error; err == nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"message": fmt.Sprintf("DTR entry for today already exists for intern ID %d", intern.Intern.ID),
			})
		}

		currentMonth := time.Now().Format("01-02-06")
		supervisorID := internData.SupervisorID

		dtrEntry := model.DTREntry{
			UserID:       internData.UserID,
			InternID:     intern.Intern.ID,
			SupervisorID: supervisorID,
			Month:        currentMonth,
			TimeInAM:     defaultTimeInAM,
		}

		if err := middleware.DBConn.Create(&dtrEntry).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to save DTR entry for intern ID " + fmt.Sprint(intern.Intern.ID),
				"error":   err.Error(),
			})
		}

		processedInterns = append(processedInterns, fiber.Map{
			"user": fiber.Map{
				"first_name":  internData.User.FirstName,
				"middle_name": internData.User.MiddleName,
				"last_name":   internData.User.LastName,
				"suffix_name": internData.User.SuffixName,
			},
			"intern": fiber.Map{
				"id":            internData.ID,
				"supervisor_id": internData.SupervisorID,
				"status":        internData.Status,
				"address":       internData.Address,
			},
		})
	}

	return c.JSON(fiber.Map{
		"message": "DTR entries for interns successfully saved",
		"data":    processedInterns,
	})
}

// ScanQRCode handles scanning QR code and saving data to PostgreSQL
func ScanQRCode(c *fiber.Ctx) error {
	type RequestBody struct {
		Interns []struct {
			User struct {
				FirstName  string `json:"first_name"`
				MiddleName string `json:"middle_name"`
				LastName   string `json:"last_name"`
				SuffixName string `json:"suffix_name"`
			} `json:"user"`
			Intern model.Intern `json:"intern"`
		} `json:"interns"`
	}

	var req RequestBody

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid JSON format",
			"error":   err.Error(),
		})
	}

	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	currentTimeStr := currentTime.Format("15:04:05")
	currentDate := currentTime.Format("2006-01-02")
	var scannedInterns []fiber.Map

	for _, intern := range req.Interns {
		if intern.Intern.ID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Intern ID is required",
			})
		}

		var internData model.Intern
		if err := middleware.DBConn.Preload("User").First(&internData, intern.Intern.ID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Intern not found",
				"error":   err.Error(),
			})
		}

		if internData.SupervisorID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": fmt.Sprintf("Supervisor ID missing for intern ID %d", intern.Intern.ID),
			})
		}

		if intern.User.FirstName != internData.User.FirstName ||
			intern.User.MiddleName != internData.User.MiddleName ||
			intern.User.LastName != internData.User.LastName ||
			intern.User.SuffixName != internData.User.SuffixName {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Provided data does not match the database values",
				"error":   "Name or suffix mismatch",
			})
		}

		var existingEntry model.DTREntry
		if err := middleware.DBConn.
			Where("intern_id = ? AND DATE(created_at) = ?", intern.Intern.ID, currentDate).
			First(&existingEntry).Error; err == nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"message": fmt.Sprintf("DTR entry for today already exists for intern ID %d", intern.Intern.ID),
			})
		}

		currentMonth := currentTime.Format("01-02-06")
		supervisorID := internData.SupervisorID

		timeInAMStart := "06:00:00"
		timeInAMEnd := "12:00:00"
		timeInPMStart := "13:00:00"
		timeInPMEnd := "17:00:00"

		var timeInAM, timeInPM string
		if currentTimeStr >= timeInAMStart && currentTimeStr <= timeInAMEnd {
			timeInAM = currentTimeStr
		} else if currentTimeStr >= timeInPMStart && currentTimeStr <= timeInPMEnd {
			timeInPM = currentTimeStr
		}

		dtrEntry := model.DTREntry{
			UserID:       internData.UserID,
			InternID:     internData.ID,
			SupervisorID: supervisorID,
			Month:        currentMonth,
			TimeInAM:     timeInAM,
			TimeOutAM:    "",
			TimeInPM:     timeInPM,
			TimeOutPM:    "",
			TotalHours:   "",
		}

		if err := middleware.DBConn.Create(&dtrEntry).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to save DTR entry for intern ID " + fmt.Sprint(intern.Intern.ID),
				"error":   err.Error(),
			})
		}

		scannedInterns = append(scannedInterns, fiber.Map{
			"user": fiber.Map{
				"first_name":  internData.User.FirstName,
				"middle_name": internData.User.MiddleName,
				"last_name":   internData.User.LastName,
				"suffix_name": internData.User.SuffixName,
			},
			"intern": fiber.Map{
				"id":            internData.ID,
				"supervisor_id": internData.SupervisorID,
				"status":        internData.Status,
				"address":       internData.Address,
			},
		})
	}

	return c.JSON(fiber.Map{
		"message": "DTR entries for interns successfully saved",
		"data":    scannedInterns,
	})
}

// UpdateTimeOutAndIn updates the TimeOutAM, TimeInPM, and TimeOutPM for an intern
func UpdateTimeOutAM(c *fiber.Ctx) error {
	// Get the current time in the Asia/Manila timezone
	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	timeOutAM := currentTime.Format("15:04:05")
	currentMonth := currentTime.Format("01-02-06") // MM-DD-YY

	var internIDs []string
	paramIDs := c.Params("id")

	if paramIDs != "" {
		internIDs = strings.Split(paramIDs, ",")
	} else {
		// Fetch all intern IDs who have TimeInAM set and TimeOutAM not set for today
		var dtrEntries []model.DTREntry
		if err := middleware.DBConn.
			Where("time_in_am IS NOT NULL AND time_in_am != '' AND (time_out_am IS NULL OR time_out_am = '') AND month = ?", currentMonth).
			Find(&dtrEntries).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch interns with valid TimeInAM",
			})
		}

		for _, entry := range dtrEntries {
			internIDs = append(internIDs, fmt.Sprintf("%d", entry.InternID))
		}
	}

	if len(internIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No valid intern IDs found for update",
		})
	}

	var errors []string

	for _, id := range internIDs {
		// Validate that the intern exists
		var intern model.Intern
		if err := middleware.DBConn.Table("interns").
			Where("id = ?", id).
			First(&intern).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Intern ID %s not found", id))
			continue
		}

		// Fetch the DTR entry for the intern
		var dtrEntry model.DTREntry
		err := middleware.DBConn.Where("intern_id = ? AND month = ?", intern.ID, currentMonth).First(&dtrEntry).Error
		if err != nil {
			errors = append(errors, fmt.Sprintf("DTR entry not found for intern ID %s", id))
			continue
		}

		if dtrEntry.TimeOutAM != "" {
			errors = append(errors, fmt.Sprintf("TimeOutAM already set for intern ID %s", id))
			continue
		}

		if dtrEntry.TimeInAM != "" {
			timeInAM, _ := time.Parse("15:04:05", dtrEntry.TimeInAM)
			timeOutAMParsed, _ := time.Parse("15:04:05", timeOutAM)
			duration := timeOutAMParsed.Sub(timeInAM)

			totalHours := fmt.Sprintf("%02d:%02d:%02d",
				int(duration.Hours()),
				int(duration.Minutes())%60,
				int(duration.Seconds())%60)
			dtrEntry.TotalHours = totalHours
		}

		dtrEntry.TimeOutAM = timeOutAM
		if err := middleware.DBConn.Save(&dtrEntry).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Failed to update TimeOutAM for intern ID %s", id))
			continue
		}
	}

	if len(errors) > 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"errors": errors,
		})
	}

	return c.JSON(fiber.Map{
		"message": "TimeOutAM successfully updated",
	})
}

func UpdateTimeInPM(c *fiber.Ctx) error {
	internIDs := c.Params("id")
	if internIDs == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Intern IDs are required",
		})
	}

	idList := strings.Split(internIDs, ",")
	if len(idList) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid intern IDs format",
		})
	}

	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	timeInPM := currentTime.Format("15:04:05")

	var errors []string

	for _, id := range idList {
		var intern model.Intern
		if err := middleware.DBConn.Table("interns").Where("id = ?", id).First(&intern).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Intern ID %s not found", id))
			continue
		}

		var dtrEntry model.DTREntry
		currentMonth := time.Now().In(loc).Format("01-02-06")

		err := middleware.DBConn.Where("intern_id = ? AND month = ?", intern.ID, currentMonth).First(&dtrEntry).Error
		if err != nil {
			errors = append(errors, fmt.Sprintf("DTR entry not found for intern ID %s", id))
			continue
		}

		// Check if TimeInPM is already set
		if dtrEntry.TimeInPM != "" {
			errors = append(errors, fmt.Sprintf("TimeInPM already set for intern ID %s", id))
			continue
		}

		updateData := map[string]interface{}{
			"time_in_pm": timeInPM,
		}

		// Start a transaction
		tx := middleware.DBConn.Begin()

		// Update DTR entry with new TimeInPM
		if err := tx.Model(&dtrEntry).Updates(updateData).Error; err != nil {
			tx.Rollback() // Rollback if any error occurs
			errors = append(errors, fmt.Sprintf("Failed to update DTR entry for intern ID %s", id))
			continue
		}

		// Commit the transaction if everything goes well
		tx.Commit()
	}

	if len(errors) > 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Some updates failed",
			"errors":  errors,
		})
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("DTR entries updated successfully for intern IDs %s with TimeInPM %s", internIDs, timeInPM),
	})
}

func UpdateTimeOutPM(c *fiber.Ctx) error {
	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	timeOutPM := currentTime.Format("15:04:05")
	currentMonth := currentTime.Format("01-02-06")

	var internIDs []string
	param := c.Params("id")

	// Determine if this is a batch update for all with TimeInPM but no TimeOutPM
	if param == "" {
		var dtrEntries []model.DTREntry
		if err := middleware.DBConn.
			Where("time_in_pm != '' AND time_out_pm = '' AND month = ?", currentMonth).
			Find(&dtrEntries).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch eligible DTR entries",
			})
		}
		for _, entry := range dtrEntries {
			internIDs = append(internIDs, fmt.Sprintf("%d", entry.InternID))
		}
		if len(internIDs) == 0 {
			return c.JSON(fiber.Map{
				"message": "No interns found with pending TimeOutPM",
			})
		}
	} else {
		internIDs = strings.Split(param, ",")
		if len(internIDs) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid intern IDs format",
			})
		}
	}

	var errors []string

	for _, id := range internIDs {
		var intern model.Intern
		if err := middleware.DBConn.Where("id = ?", id).First(&intern).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Intern ID %s not found", id))
			continue
		}

		var dtrEntry model.DTREntry
		if err := middleware.DBConn.
			Where("intern_id = ? AND month = ?", intern.ID, currentMonth).
			First(&dtrEntry).Error; err != nil {
			errors = append(errors, fmt.Sprintf("DTR entry not found for intern ID %s", id))
			continue
		}

		if dtrEntry.TimeOutPM != "" {
			errors = append(errors, fmt.Sprintf("TimeOutPM already recorded for intern ID %s", id))
			continue
		}

		var totalAM, totalPM float64
		var pmValid bool

		if dtrEntry.TimeInAM != "" && dtrEntry.TimeOutAM != "" {
			tInAM, err1 := time.Parse("15:04:05", dtrEntry.TimeInAM)
			tOutAM, err2 := time.Parse("15:04:05", dtrEntry.TimeOutAM)
			if err1 == nil && err2 == nil {
				totalAM = tOutAM.Sub(tInAM).Hours()
			}
		}

		if dtrEntry.TimeInPM != "" {
			tInPM, err1 := time.Parse("15:04:05", dtrEntry.TimeInPM)
			tOutPM, err2 := time.Parse("15:04:05", timeOutPM)
			if err1 == nil && err2 == nil {
				totalPM = tOutPM.Sub(tInPM).Hours()
				pmValid = true
			} else {
				errors = append(errors, fmt.Sprintf("Invalid PM time format for intern ID %s", id))
				continue
			}
		} else {
			errors = append(errors, fmt.Sprintf("TimeInPM is missing for intern ID %s", id))
			continue
		}

		if totalAM == 0 && !pmValid {
			errors = append(errors, fmt.Sprintf("No valid AM or PM times for intern ID %s", id))
			continue
		}

		total := totalAM + totalPM
		h := int(total)
		m := int((total - float64(h)) * 60)
		s := int(((total-float64(h))*60 - float64(m)) * 60)
		totalHoursFormatted := fmt.Sprintf("%02d:%02d:%02d", h, m, s)

		updateData := map[string]interface{}{
			"time_out_pm": timeOutPM,
			"total_hours": totalHoursFormatted,
		}

		tx := middleware.DBConn.Begin()

		if err := tx.Model(&dtrEntry).Updates(updateData).Error; err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("Failed to update DTR entry for intern ID %s", id))
			continue
		}

		// Recalculate total rendered
		var allDTR []model.DTREntry
		if err := middleware.DBConn.Where("intern_id = ?", intern.ID).Find(&allDTR).Error; err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("Failed to fetch DTR entries for intern ID %s", id))
			continue
		}

		var totalRendered time.Duration
		for _, entry := range allDTR {
			parts := strings.Split(entry.TotalHours, ":")
			if len(parts) != 3 {
				continue
			}
			h, _ := strconv.Atoi(parts[0])
			m, _ := strconv.Atoi(parts[1])
			s, _ := strconv.Atoi(parts[2])
			totalRendered += time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second
		}

		totalRenderedStr := fmt.Sprintf("%02d:%02d:%02d", int(totalRendered.Hours()), int(totalRendered.Minutes())%60, int(totalRendered.Seconds())%60)
		if err := tx.Model(&intern).Update("ojt_hours_rendered", totalRenderedStr).Error; err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("Failed to update OjtHoursRendered for intern ID %s", id))
			continue
		}

		tx.Commit()
	}

	if len(errors) > 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Some updates failed",
			"errors":  errors,
		})
	}

	successMessage := "TimeOutPM successfully updated"
	if param != "" {
		successMessage += fmt.Sprintf(" for intern IDs: %s", param)
	}
	return c.JSON(fiber.Map{
		"message": successMessage,
	})
}

func AutoInsertAbsentDTR() {
	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	currentHour := currentTime.Hour()

	// Debug: Show current time and hour
	fmt.Println("Current Manila time:", currentTime.Format("2006-01-02 15:04:05"))
	fmt.Println("Current hour (24h):", currentHour)

	// Only run this logic if it's 8 AM or later
	if currentHour < 8 {
		fmt.Println("It's before 08:00, skipping AutoInsertAbsentDTR")
		return
	}

	currentDate := currentTime.Format("2006-01-02")
	currentMonth := currentTime.Format("01-02-06")

	var interns []model.Intern
	if err := middleware.DBConn.Preload("User").Find(&interns).Error; err != nil {
		fmt.Println("Failed to fetch interns:", err)
		return
	}

	for _, intern := range interns {
		if intern.SupervisorID == 0 {
			fmt.Printf("Skipping intern ID %d due to missing Supervisor ID\n", intern.ID)
			continue
		}

		var existingEntry model.DTREntry
		err := middleware.DBConn.
			Where("intern_id = ? AND DATE(created_at) = ?", intern.ID, currentDate).
			First(&existingEntry).Error

		if err != nil {
			absentEntry := model.DTREntry{
				UserID:       intern.UserID,
				InternID:     intern.ID,
				SupervisorID: intern.SupervisorID,
				Month:        currentMonth,
				TimeInAM:     "",
				TimeOutAM:    "",
				TimeInPM:     "",
				TimeOutPM:    "",
				TotalHours:   "00:00:00",
			}

			if err := middleware.DBConn.Create(&absentEntry).Error; err != nil {
				fmt.Printf("Failed to create absent DTR for intern ID %d: %v\n", intern.ID, err)
			} else {
				fmt.Printf("Absent DTR inserted for intern ID %d\n", intern.ID)
			}
		} else {
			fmt.Printf("DTR already exists for intern ID %d, skipping insert.\n", intern.ID)
		}
	}

}
