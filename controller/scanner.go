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

// Define QRCodeData struct to be shared between ScanQRCode and DefaultTime
type QRCodeData struct {
	UserID       uint   `json:"user_id"`
	FirstName    string `json:"first_name"`
	MiddleName   string `json:"middle_name"`
	LastName     string `json:"last_name"`
	SuffixName   string `json:"suffix_name"`
	SupervisorID uint   `json:"supervisor_id"`
	InternID     uint   `json:"intern_id"`
	Status       string `json:"status"`
	Address      string `json:"address"`
	QRCode       string `json:"qr_code"`
}

// RequestBody struct with the shared QRCodeData
type RequestBody struct {
	Interns []struct {
		QRCodeData QRCodeData `json:"qr_code_data"`
	} `json:"interns"`
}

func ScanQRCode(c *fiber.Ctx) error {
	var req RequestBody

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid JSON format",
			"error":   err.Error(),
		})
	}

	// Get current time in Asia/Manila timezone
	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	currentTimeStr := currentTime.Format("15:04:05")
	currentDate := currentTime.Format("2006-01-02")
	var scannedInterns []fiber.Map

	// Determine if the current time is AM or PM
	hour := currentTime.Hour()

	// Iterate through all interns in the request
	for _, intern := range req.Interns {
		if intern.QRCodeData.InternID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Intern ID is required in QR code data",
			})
		}

		if intern.QRCodeData.SupervisorID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": fmt.Sprintf("Supervisor ID missing for intern ID %d", intern.QRCodeData.InternID),
			})
		}

		// Fetch intern record from the database along with related user data
		var storedIntern model.Intern
		if err := middleware.DBConn.Preload("User").Where("id = ?", intern.QRCodeData.InternID).First(&storedIntern).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": fmt.Sprintf("Intern with ID %d not found", intern.QRCodeData.InternID),
			})
		}

		// ✅ NEW: Validate UserID from QRCodeData vs actual User.ID in DB
		if intern.QRCodeData.UserID != storedIntern.User.ID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": fmt.Sprintf("User ID mismatch for intern ID %d", intern.QRCodeData.InternID),
			})
		}

		// Validate QR code data against intern's record
		if storedIntern.User.FirstName != intern.QRCodeData.FirstName ||
			storedIntern.User.MiddleName != intern.QRCodeData.MiddleName ||
			storedIntern.User.LastName != intern.QRCodeData.LastName ||
			storedIntern.User.SuffixName != intern.QRCodeData.SuffixName ||
			storedIntern.Status != intern.QRCodeData.Status ||
			storedIntern.Address != intern.QRCodeData.Address ||
			storedIntern.SupervisorID != intern.QRCodeData.SupervisorID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": fmt.Sprintf("QR code data does not match the intern's record for intern ID %d", intern.QRCodeData.InternID),
			})
		}

		// Check if DTR entry for today already exists for this intern
		var existingEntry model.DTREntry
		if err := middleware.DBConn.
			Where("intern_id = ? AND DATE(created_at) = ?", intern.QRCodeData.InternID, currentDate).
			First(&existingEntry).Error; err == nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"message": fmt.Sprintf("DTR entry for today already exists for intern ID %d", intern.QRCodeData.InternID),
			})
		}

		currentMonth := currentTime.Format("01-02-06")

		// Create a new DTR entry
		dtrEntry := model.DTREntry{
			UserID:       storedIntern.User.ID, // ✅ safe value from DB
			InternID:     intern.QRCodeData.InternID,
			SupervisorID: intern.QRCodeData.SupervisorID,
			Month:        currentMonth,
			TimeInAM:     "",
			TimeOutAM:    "",
			TimeInPM:     "",
			TimeOutPM:    "",
			TotalHours:   "",
		}

		if hour < 12 {
			dtrEntry.TimeInAM = currentTimeStr
		} else {
			dtrEntry.TimeInPM = currentTimeStr
		}

		if err := middleware.DBConn.Create(&dtrEntry).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to save DTR entry for intern ID " + fmt.Sprint(intern.QRCodeData.InternID),
				"error":   err.Error(),
			})
		}

		scannedInterns = append(scannedInterns, fiber.Map{
			"user": fiber.Map{
				"first_name":  intern.QRCodeData.FirstName,
				"middle_name": intern.QRCodeData.MiddleName,
				"last_name":   intern.QRCodeData.LastName,
				"suffix_name": intern.QRCodeData.SuffixName,
			},
			"intern": fiber.Map{
				"id":            intern.QRCodeData.InternID,
				"supervisor_id": intern.QRCodeData.SupervisorID,
				"status":        intern.QRCodeData.Status,
				"address":       intern.QRCodeData.Address,
			},
		})
	}

	return c.JSON(fiber.Map{
		"message": "DTR entries for interns successfully saved",
		"data":    scannedInterns,
	})
}

func DefaultTime(c *fiber.Ctx) error {
	var req RequestBody

	// Parse the request body
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

	// Iterate through all interns in the request
	for _, intern := range req.Interns {
		// Validate intern ID from QRCodeData
		if intern.QRCodeData.InternID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Intern ID is required",
			})
		}

		// Fetch the intern and related user data from the database
		var internData model.Intern
		if err := middleware.DBConn.Preload("User").First(&internData, intern.QRCodeData.InternID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Intern not found",
				"error":   err.Error(),
			})
		}

		// Validate QR code data against intern and user fields
		if intern.QRCodeData.FirstName != internData.User.FirstName ||
			intern.QRCodeData.MiddleName != internData.User.MiddleName ||
			intern.QRCodeData.LastName != internData.User.LastName ||
			intern.QRCodeData.SuffixName != internData.User.SuffixName ||
			intern.QRCodeData.InternID != internData.ID ||
			intern.QRCodeData.SupervisorID != internData.SupervisorID ||
			intern.QRCodeData.Status != internData.Status ||
			intern.QRCodeData.Address != internData.Address {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Provided intern data does not match the database records",
				"error":   "Intern details mismatch",
			})
		}

		// Check if DTR entry for today already exists for this intern
		var existingEntry model.DTREntry
		if err := middleware.DBConn.
			Where("intern_id = ? AND DATE(created_at) = ?", intern.QRCodeData.InternID, currentDate).
			First(&existingEntry).Error; err == nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"message": fmt.Sprintf("DTR entry for today already exists for intern ID %d", intern.QRCodeData.InternID),
			})
		}

		// Get current month and supervisor ID
		currentMonth := time.Now().Format("01-02-06")
		supervisorID := internData.SupervisorID

		// Create a new DTR entry with the default time for AM
		dtrEntry := model.DTREntry{
			UserID:       internData.UserID,
			InternID:     intern.QRCodeData.InternID,
			SupervisorID: supervisorID,
			Month:        currentMonth,
			TimeInAM:     defaultTimeInAM,
		}

		// Save the DTR entry to the database
		if err := middleware.DBConn.Create(&dtrEntry).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to save DTR entry for intern ID " + fmt.Sprint(intern.QRCodeData.InternID),
				"error":   err.Error(),
			})
		}

		// Prepare response data with the intern's details
		processedInterns = append(processedInterns, fiber.Map{
			"user": fiber.Map{
				"first_name":  intern.QRCodeData.FirstName,
				"middle_name": intern.QRCodeData.MiddleName,
				"last_name":   intern.QRCodeData.LastName,
				"suffix_name": intern.QRCodeData.SuffixName,
			},
			"intern": fiber.Map{
				"id":            intern.QRCodeData.InternID,
				"supervisor_id": intern.QRCodeData.SupervisorID,
				"status":        intern.QRCodeData.Status,
				"address":       intern.QRCodeData.Address,
			},
		})
	}

	// Return success response
	return c.JSON(fiber.Map{
		"message": "DTR entries for interns successfully saved",
		"data":    processedInterns,
	})
}

func UpdateScannerQr(c *fiber.Ctx) error {
	var req RequestBody

	// Parse the incoming request body
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid JSON format",
			"error":   err.Error(),
		})
	}

	// Get current time in Asia/Manila timezone
	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	currentTimeStr := currentTime.Format("15:04:05")
	currentDate := currentTime.Format("2006-01-02")
	var updatedInterns []fiber.Map

	// Determine if the current time is AM or PM
	hour := currentTime.Hour()

	// Iterate through all interns in the request
	for _, intern := range req.Interns {
		// Check if intern ID is provided
		if intern.QRCodeData.InternID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Intern ID is required",
			})
		}

		// Fetch intern data from the database
		var internData model.Intern
		if err := middleware.DBConn.Preload("User").First(&internData, intern.QRCodeData.InternID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Intern not found",
				"error":   err.Error(),
			})
		}

		// Check if Supervisor ID is missing
		if internData.SupervisorID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": fmt.Sprintf("Supervisor ID missing for intern ID %d", intern.QRCodeData.InternID),
			})
		}

		// Check if provided name data matches database values
		if intern.QRCodeData.FirstName != internData.User.FirstName ||
			intern.QRCodeData.MiddleName != internData.User.MiddleName ||
			intern.QRCodeData.LastName != internData.User.LastName ||
			intern.QRCodeData.SuffixName != internData.User.SuffixName ||
			intern.QRCodeData.SupervisorID != internData.SupervisorID ||
			intern.QRCodeData.Status != internData.Status ||
			intern.QRCodeData.Address != internData.Address {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Provided intern data does not match the database records",
				"error":   "Intern details mismatch",
			})
		}

		// Check if DTR entry for today exists
		var existingEntry model.DTREntry
		if err := middleware.DBConn.
			Where("intern_id = ? AND DATE(created_at) = ?", intern.QRCodeData.InternID, currentDate).
			First(&existingEntry).Error; err == nil {
			// If the entry exists, we update the time-in or time-out field based on AM/PM
			if hour < 12 && existingEntry.TimeInAM == "" {
				existingEntry.TimeInAM = currentTimeStr
			} else if hour >= 12 && existingEntry.TimeInPM == "" {
				existingEntry.TimeInPM = currentTimeStr
			}

			// Update the DTR entry with the current time
			if err := middleware.DBConn.Save(&existingEntry).Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"message": fmt.Sprintf("Failed to update DTR entry for intern ID %d", intern.QRCodeData.InternID),
					"error":   err.Error(),
				})
			}
		} else {
			// No entry exists for today, so create a new DTR entry
			currentMonth := currentTime.Format("01-02-06")
			supervisorID := internData.SupervisorID

			// Create a new DTR entry and set the time-in based on AM/PM
			dtrEntry := model.DTREntry{
				UserID:       internData.UserID,
				InternID:     internData.ID,
				SupervisorID: supervisorID,
				Month:        currentMonth,
				TimeInAM:     "", // Initially set as empty
				TimeOutAM:    "",
				TimeInPM:     "", // Initially set as empty
				TimeOutPM:    "",
				TotalHours:   "",
			}

			// Set TimeInAM or TimeInPM based on the time of day
			if hour < 12 {
				// If it's AM, set TimeInAM to the current time
				dtrEntry.TimeInAM = currentTimeStr
			} else {
				// If it's PM, set TimeInPM to the current time
				dtrEntry.TimeInPM = currentTimeStr
			}

			// Save the DTR entry to the database
			if err := middleware.DBConn.Create(&dtrEntry).Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"message": "Failed to save DTR entry for intern ID " + fmt.Sprint(intern.QRCodeData.InternID),
					"error":   err.Error(),
				})
			}
		}

		// Prepare response data with the intern's details
		updatedInterns = append(updatedInterns, fiber.Map{
			"user": fiber.Map{
				"first_name":  intern.QRCodeData.FirstName,
				"middle_name": intern.QRCodeData.MiddleName,
				"last_name":   intern.QRCodeData.LastName,
				"suffix_name": intern.QRCodeData.SuffixName,
			},
			"intern": fiber.Map{
				"id":            intern.QRCodeData.InternID,
				"supervisor_id": intern.QRCodeData.SupervisorID,
				"status":        intern.QRCodeData.Status,
				"address":       intern.QRCodeData.Address,
			},
		})
	}

	// Return success response
	return c.JSON(fiber.Map{
		"message": "DTR entries for interns successfully updated",
		"data":    updatedInterns,
	})
}

// UpdateTimeOutAM updates the TimeOutAM and recalculates TotalHours and OjtHoursRendered
func UpdateTimeOutAMDefault(c *fiber.Ctx) error {
	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	timeOutAM := "12:00:00"
	currentDate := currentTime.Format("2006-01-02")

	// Fetch all DTR entries where TimeInAM is set but TimeOutAM is missing
	var dtrEntries []model.DTREntry
	if err := middleware.DBConn.
		Where("time_in_am IS NOT NULL AND time_in_am != '' AND (time_out_am IS NULL OR time_out_am = '') AND DATE(created_at) = ?", currentDate).
		Find(&dtrEntries).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch interns with valid TimeInAM",
		})
	}

	if len(dtrEntries) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No valid intern DTR entries found for update",
		})
	}

	var errors []string

	// Process each DTR entry
	for _, dtrEntry := range dtrEntries {
		var intern model.Intern
		if err := middleware.DBConn.First(&intern, dtrEntry.InternID).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Intern ID %d not found", dtrEntry.InternID))
			continue
		}

		// Check if TimeInAM is missing before allowing TimeOutAM to be set
		if dtrEntry.TimeInAM == "" {
			errors = append(errors, fmt.Sprintf("TimeInAM is missing for intern ID %d, cannot set TimeOutAM", dtrEntry.InternID))
			continue
		}

		// Prevent updating if TimeOutAM is already set
		if dtrEntry.TimeOutAM != "" {
			errors = append(errors, fmt.Sprintf("TimeOutAM already set for intern ID %d", dtrEntry.InternID))
			continue
		}

		// Calculate the duration between TimeInAM and TimeOutAM
		totalSeconds := 0
		if dtrEntry.TimeInAM != "" {
			timeInAM, _ := time.Parse("15:04:05", dtrEntry.TimeInAM)
			timeOutAMParsed, _ := time.Parse("15:04:05", timeOutAM)
			duration := timeOutAMParsed.Sub(timeInAM)
			totalSeconds += int(duration.Seconds())
		}

		// Always set TimeOutAM to default value
		dtrEntry.TimeOutAM = timeOutAM

		// Check if TimeInPM and TimeOutPM already exist and calculate
		if dtrEntry.TimeInPM != "" && dtrEntry.TimeOutPM != "" {
			timeInPM, _ := time.Parse("15:04:05", dtrEntry.TimeInPM)
			timeOutPM, _ := time.Parse("15:04:05", dtrEntry.TimeOutPM)
			durationPM := timeOutPM.Sub(timeInPM)
			totalSeconds += int(durationPM.Seconds())
		}

		// Update the TotalHours field
		hours := totalSeconds / 3600
		minutes := (totalSeconds % 3600) / 60
		seconds := totalSeconds % 60
		dtrEntry.TotalHours = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

		// Save the updated DTR entry
		if err := middleware.DBConn.Save(&dtrEntry).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Failed to update DTR for intern ID %d", dtrEntry.InternID))
			continue
		}

		// Sum up all TotalHours for this intern
		var allDTRs []model.DTREntry
		if err := middleware.DBConn.Where("intern_id = ?", intern.ID).Find(&allDTRs).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Failed to fetch DTR entries for intern ID %d", intern.ID))
			continue
		}

		// Calculate the total rendered hours
		totalRenderedSeconds := 0
		for _, dtr := range allDTRs {
			// Ensure that TotalHours is in the correct format
			if dtr.TotalHours != "" {
				parts := strings.Split(dtr.TotalHours, ":")
				if len(parts) == 3 {
					hours, err := strconv.Atoi(parts[0])
					if err != nil {
						errors = append(errors, fmt.Sprintf("Failed to parse hours from TotalHours for intern ID %d", intern.ID))
						continue
					}
					minutes, err := strconv.Atoi(parts[1])
					if err != nil {
						errors = append(errors, fmt.Sprintf("Failed to parse minutes from TotalHours for intern ID %d", intern.ID))
						continue
					}
					seconds, err := strconv.Atoi(parts[2])
					if err != nil {
						errors = append(errors, fmt.Sprintf("Failed to parse seconds from TotalHours for intern ID %d", intern.ID))
						continue
					}
					totalRenderedSeconds += (hours * 3600) + (minutes * 60) + seconds
				} else {
					errors = append(errors, fmt.Sprintf("Invalid TotalHours format for intern ID %d", intern.ID))
					continue
				}
			}
		}

		// Update OjtHoursRendered for the intern
		intern.OjtHoursRendered = fmt.Sprintf("%02d:%02d:%02d", totalRenderedSeconds/3600, (totalRenderedSeconds%3600)/60, totalRenderedSeconds%60)
		if err := middleware.DBConn.Save(&intern).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Failed to update OjtHoursRendered for intern ID %d", intern.ID))
			continue
		}
	}

	// Return the errors if any
	if len(errors) > 0 {
		return c.Status(fiber.StatusPartialContent).JSON(fiber.Map{
			"message": "Some entries were not updated",
			"errors":  errors,
		})
	}

	// Success message
	return c.JSON(fiber.Map{
		"message": "TimeOutAM, TotalHours, and OjtHoursRendered updated successfully",
	})
}

func UpdateTimeOutAMCurrent(c *fiber.Ctx) error {
	// Load the Asia/Manila time zone
	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	currentTimeStr := currentTime.Format("15:04:05") // Current time in HH:MM:SS format
	currentDate := currentTime.Format("2006-01-02")  // Current date in YYYY-MM-DD format

	// Get the intern IDs from the URL parameter (comma-separated)
	idsParam := c.Params("ids")
	if idsParam == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Intern IDs are required",
		})
	}

	// Split the IDs into a slice
	ids := strings.Split(idsParam, ",")
	var successCount int
	var errorMessages []string

	// Loop over each intern ID and update the TimeOutAM
	for _, internID := range ids {
		// Fetch the DTR entry for the intern with the specified ID and current date
		var dtrEntry model.DTREntry
		if err := middleware.DBConn.Where("intern_id = ? AND DATE(created_at) = ?", internID, currentDate).First(&dtrEntry).Error; err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("Failed to fetch DTR entry for intern ID %s", internID))
			continue
		}

		// Prevent updating if TimeInAM is missing
		if dtrEntry.TimeInAM == "" {
			errorMessages = append(errorMessages, fmt.Sprintf("TimeInAM is missing for intern ID %s, cannot set TimeOutAM", internID))
			continue
		}

		// Prevent updating if TimeOutAM is already set
		if dtrEntry.TimeOutAM != "" {
			errorMessages = append(errorMessages, fmt.Sprintf("TimeOutAM already set for intern ID %s", internID))
			continue
		}

		// Calculate the duration between TimeInAM and the current TimeOutAM
		totalSeconds := 0
		if dtrEntry.TimeInAM != "" {
			timeInAM, _ := time.Parse("15:04:05", dtrEntry.TimeInAM)
			timeOutAMParsed, _ := time.Parse("15:04:05", currentTimeStr)
			duration := timeOutAMParsed.Sub(timeInAM)
			totalSeconds += int(duration.Seconds())
		}

		// Set TimeOutAM to the current time
		dtrEntry.TimeOutAM = currentTimeStr

		// Update the TotalHours field
		hours := totalSeconds / 3600
		minutes := (totalSeconds % 3600) / 60
		seconds := totalSeconds % 60
		dtrEntry.TotalHours = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

		// Save the updated DTR entry
		if err := middleware.DBConn.Save(&dtrEntry).Error; err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("Failed to update DTR for intern ID %s", internID))
			continue
		}

		// Sum up all TotalHours for this intern
		var allDTRs []model.DTREntry
		if err := middleware.DBConn.Where("intern_id = ?", internID).Find(&allDTRs).Error; err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("Failed to fetch DTR entries for intern ID %s", internID))
			continue
		}

		// Calculate the total rendered hours
		totalRenderedSeconds := 0
		for _, dtr := range allDTRs {
			if dtr.TotalHours != "" {
				parts := strings.Split(dtr.TotalHours, ":")
				if len(parts) == 3 {
					hours, err := strconv.Atoi(parts[0])
					if err != nil {
						errorMessages = append(errorMessages, fmt.Sprintf("Failed to parse hours from TotalHours for intern ID %s", internID))
						continue
					}
					minutes, err := strconv.Atoi(parts[1])
					if err != nil {
						errorMessages = append(errorMessages, fmt.Sprintf("Failed to parse minutes from TotalHours for intern ID %s", internID))
						continue
					}
					seconds, err := strconv.Atoi(parts[2])
					if err != nil {
						errorMessages = append(errorMessages, fmt.Sprintf("Failed to parse seconds from TotalHours for intern ID %s", internID))
						continue
					}
					totalRenderedSeconds += (hours * 3600) + (minutes * 60) + seconds
				} else {
					errorMessages = append(errorMessages, fmt.Sprintf("Invalid TotalHours format for intern ID %s", internID))
					continue
				}
			}
		}

		// Update OjtHoursRendered for the intern
		intern := model.Intern{}
		if err := middleware.DBConn.First(&intern, internID).Error; err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("Failed to fetch intern with ID %s", internID))
			continue
		}

		intern.OjtHoursRendered = fmt.Sprintf("%02d:%02d:%02d", totalRenderedSeconds/3600, (totalRenderedSeconds%3600)/60, totalRenderedSeconds%60)

		if err := middleware.DBConn.Save(&intern).Error; err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("Failed to update OjtHoursRendered for intern ID %s", internID))
			continue
		}

		successCount++
	}

	// Prepare response
	if len(errorMessages) > 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": fmt.Sprintf("%d intern(s) successfully updated, %d failed", successCount, len(errorMessages)),
			"errors":  errorMessages,
		})
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("%d intern(s) updated successfully", successCount),
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
	// Fetch intern IDs from the URL parameters
	internIDs := c.Params("id")
	if internIDs == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Intern IDs are required",
		})
	}

	// Split the intern IDs by commas
	idList := strings.Split(internIDs, ",")
	if len(idList) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid intern IDs format",
		})
	}

	// Get the current time in Asia/Manila timezone
	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	timeOutPM := currentTime.Format("15:04:05")

	var errors []string

	// Loop through each intern ID
	for _, id := range idList {
		var intern model.Intern
		// Look up intern by ID
		if err := middleware.DBConn.Table("interns").Where("id = ?", id).First(&intern).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Intern ID %s not found", id))
			continue
		}

		var dtrEntry model.DTREntry
		// Get current month in format "MM-DD-YY"
		currentMonth := time.Now().In(loc).Format("01-02-06")

		// Fetch DTR entry for the intern
		err := middleware.DBConn.Where("intern_id = ? AND month = ?", intern.ID, currentMonth).First(&dtrEntry).Error
		if err != nil {
			errors = append(errors, fmt.Sprintf("DTR entry not found for intern ID %s", id))
			continue
		}

		// Check if TimeOutPM already exists
		if dtrEntry.TimeOutPM != "" {
			errors = append(errors, fmt.Sprintf("TimeOutPM already recorded for intern ID %s", id))
			continue
		}

		// Variables to hold the total hours calculations
		var totalAM, totalPM float64
		var pmValid bool

		// Try parsing AM times if available
		if dtrEntry.TimeInAM != "" && dtrEntry.TimeOutAM != "" {
			timeInAM, err1 := time.Parse("15:04:05", dtrEntry.TimeInAM)
			timeOutAM, err2 := time.Parse("15:04:05", dtrEntry.TimeOutAM)
			if err1 == nil && err2 == nil {
				totalAM = timeOutAM.Sub(timeInAM).Hours()
			}
		}

		// Try parsing PM times if available
		if dtrEntry.TimeInPM != "" {
			timeInPM, err1 := time.Parse("15:04:05", dtrEntry.TimeInPM)
			timeOutPMParsed, err2 := time.Parse("15:04:05", timeOutPM)
			if err1 == nil && err2 == nil {
				totalPM = timeOutPMParsed.Sub(timeInPM).Hours()
				pmValid = true
			} else {
				errors = append(errors, fmt.Sprintf("Invalid PM time format for intern ID %s", id))
				continue
			}
		} else {
			errors = append(errors, fmt.Sprintf("TimeInPM is missing for intern ID %s", id))
			continue
		}

		// Skip if no valid working hours
		if totalAM == 0 && !pmValid {
			errors = append(errors, fmt.Sprintf("No valid AM or PM times for intern ID %s", id))
			continue
		}

		// Compute total hours
		totalHours := totalAM + totalPM
		hours := int(totalHours)
		minutes := int((totalHours - float64(hours)) * 60)
		seconds := int(((totalHours-float64(hours))*60 - float64(minutes)) * 60)
		totalHoursFormatted := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

		// Data to be updated in the DTR entry
		updateData := map[string]interface{}{
			"time_out_pm": timeOutPM,
			"total_hours": totalHoursFormatted,
		}

		// Start a database transaction
		tx := middleware.DBConn.Begin()

		// Update DTR entry with new timeOutPM and totalHours
		if err := tx.Model(&dtrEntry).Updates(updateData).Error; err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("Failed to update DTR entry for intern ID %s", id))
			continue
		}

		// Recalculate OJT hours rendered for the intern
		var dtrEntries []model.DTREntry
		err = middleware.DBConn.Where("intern_id = ?", intern.ID).Find(&dtrEntries).Error
		if err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("Failed to fetch DTR entries for intern ID %s", id))
			continue
		}

		// Sum all total hours and update OjtHoursRendered
		var totalRendered time.Duration
		for _, entry := range dtrEntries {
			parts := strings.Split(entry.TotalHours, ":")
			if len(parts) != 3 {
				continue
			}
			h, _ := strconv.Atoi(parts[0])
			m, _ := strconv.Atoi(parts[1])
			s, _ := strconv.Atoi(parts[2])
			totalRendered += time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second
		}

		// Update OjtHoursRendered in the intern record
		totalRenderedStr := fmt.Sprintf("%02d:%02d:%02d", int(totalRendered.Hours()), int(totalRendered.Minutes())%60, int(totalRendered.Seconds())%60)
		err = middleware.DBConn.Model(&intern).Update("ojt_hours_rendered", totalRenderedStr).Error
		if err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("Failed to update OjtHoursRendered for intern ID %s", id))
			continue
		}

		// Commit the transaction
		tx.Commit()
	}

	// Return error details if there were any issues
	if len(errors) > 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Some updates failed",
			"errors":  errors,
		})
	}

	// Return a success message
	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("DTR entries updated successfully for intern IDs %s with TimeOutPM %s", internIDs, timeOutPM),
	})
}

func UpdateTimeOutPMDefault(c *fiber.Ctx) error {
	loc, _ := time.LoadLocation("Asia/Manila")
	now := time.Now().In(loc)
	timeOutPM := "17:00:00"
	currentMonth := now.Format("01-02-06")

	var dtrEntries []model.DTREntry
	err := middleware.DBConn.
		Where("month = ? AND time_in_pm != '' AND time_out_pm = ''", currentMonth).
		Find(&dtrEntries).Error

	if err != nil || len(dtrEntries) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "No eligible DTR entries found for update.",
		})
	}

	var errors []string
	for _, dtrEntry := range dtrEntries {
		var intern model.Intern
		if err := middleware.DBConn.Where("id = ?", dtrEntry.InternID).First(&intern).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Intern ID %d not found", dtrEntry.InternID))
			continue
		}

		var totalAM, totalPM float64
		var pmValid bool

		// Compute AM hours if available
		if dtrEntry.TimeInAM != "" && dtrEntry.TimeOutAM != "" {
			tInAM, err1 := time.Parse("15:04:05", dtrEntry.TimeInAM)
			tOutAM, err2 := time.Parse("15:04:05", dtrEntry.TimeOutAM)
			if err1 == nil && err2 == nil {
				totalAM = tOutAM.Sub(tInAM).Hours()
			}
		}

		// Compute PM hours
		tInPM, err1 := time.Parse("15:04:05", dtrEntry.TimeInPM)
		tOutPM, err2 := time.Parse("15:04:05", timeOutPM)
		if err1 == nil && err2 == nil {
			totalPM = tOutPM.Sub(tInPM).Hours()
			pmValid = true
		} else {
			errors = append(errors, fmt.Sprintf("Invalid PM time format for intern ID %d", dtrEntry.InternID))
			continue
		}

		if totalAM == 0 && !pmValid {
			errors = append(errors, fmt.Sprintf("No valid time records for intern ID %d", dtrEntry.InternID))
			continue
		}

		totalHours := totalAM + totalPM
		hours := int(totalHours)
		minutes := int((totalHours - float64(hours)) * 60)
		seconds := int(((totalHours-float64(hours))*60 - float64(minutes)) * 60)
		totalHoursFormatted := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

		tx := middleware.DBConn.Begin()

		err = tx.Model(&dtrEntry).Updates(map[string]interface{}{
			"time_out_pm": timeOutPM,
			"total_hours": totalHoursFormatted,
		}).Error
		if err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("Failed to update DTR for intern ID %d", dtrEntry.InternID))
			continue
		}

		// Recalculate all rendered hours
		var entries []model.DTREntry
		err = tx.Where("intern_id = ?", dtrEntry.InternID).Find(&entries).Error
		if err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("Failed to fetch DTRs for intern ID %d", dtrEntry.InternID))
			continue
		}

		var totalRendered time.Duration
		for _, entry := range entries {
			parts := strings.Split(entry.TotalHours, ":")
			if len(parts) != 3 {
				continue
			}
			h, _ := strconv.Atoi(parts[0])
			m, _ := strconv.Atoi(parts[1])
			s, _ := strconv.Atoi(parts[2])
			totalRendered += time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second
		}

		renderedStr := fmt.Sprintf("%02d:%02d:%02d", int(totalRendered.Hours()), int(totalRendered.Minutes())%60, int(totalRendered.Seconds())%60)
		err = tx.Model(&intern).Update("ojt_hours_rendered", renderedStr).Error
		if err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("Failed to update OJT hours for intern ID %d", dtrEntry.InternID))
			continue
		}

		tx.Commit()
	}

	if len(errors) > 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Some entries failed to update.",
			"errors":  errors,
		})
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Successfully updated TimeOutPM to %s for all valid interns", timeOutPM),
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
