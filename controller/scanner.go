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

//make the timeinam default to 08:00AM
func DefaultTime(c *fiber.Ctx) error {
	// Define a struct to parse JSON input
	type RequestBody struct {
		Interns []struct {
			User   model.User   `json:"user"`
			Intern model.Intern `json:"intern"`
		} `json:"interns"`
	}

	var req RequestBody

	// Parse JSON body
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid JSON format",
			"error":   err.Error(),
		})
	}

	// Set default TimeInAM to "08:00:00"
	defaultTimeInAM := "08:00:00"

	// This will store successfully processed intern data
	var processedInterns []fiber.Map

	// Iterate over each intern to handle their data
	for _, intern := range req.Interns {
		// Validate required field
		if intern.Intern.ID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Intern ID is required",
			})
		}

		// Fetch the intern data along with associated user details
		var internData model.Intern
		if err := middleware.DBConn.Preload("User").First(&internData, intern.Intern.ID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Intern not found",
				"error":   err.Error(),
			})
		}

		// Validate if the user details from the request match the data in the database
		if intern.User.FirstName != internData.User.FirstName ||
			intern.User.MiddleName != internData.User.MiddleName ||
			intern.User.LastName != internData.User.LastName {
			// Return an error if the names don't match
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Provided data does not match the database values",
				"error":   "First Name, Middle Name, and Last Name mismatch",
			})
		}

		// Get the current month in "MM-YY" format
		currentMonth := time.Now().Format("01-02-06") // MM-DD-YY format

		// Set SupervisorID (Modify if needed)
		supervisorID := uint(1)

		// Create a new DTR entry (Only storing TimeInAM as default)
		dtrEntry := model.DTREntry{
			UserID:       internData.UserID,
			InternID:     intern.Intern.ID, // Reference InternID properly
			SupervisorID: supervisorID,
			Month:        currentMonth,
			TimeInAM:     defaultTimeInAM, // Default TimeInAM set to "08:00:00"
			// Other fields can be left blank or null if not required
			TimeOutAM:  "", // Optional: set to blank or null as needed
			TimeInPM:   "", // Optional: set to blank or null as needed
			TimeOutPM:  "", // Optional: set to blank or null as needed
			TotalHours: "", // Optional: set to 0 or null, depending on your needs
		}

		// Insert into the database for this intern
		if err := middleware.DBConn.Create(&dtrEntry).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to save DTR entry for intern ID " + fmt.Sprint(intern.Intern.ID),
				"error":   err.Error(),
			})
		}

		// Append processed intern info to the response
		processedInterns = append(processedInterns, fiber.Map{
			"user": fiber.Map{
				"first_name":  internData.User.FirstName,
				"middle_name": internData.User.MiddleName,
				"last_name":   internData.User.LastName,
			},
			"intern": fiber.Map{
				"id":            internData.ID,
				"supervisor_id": internData.SupervisorID,
				"status":        internData.Status,
				"address":       internData.Address,
			},
		})
	}

	// Return success response for all interns
	return c.JSON(fiber.Map{
		"message": "DTR entries for interns successfully saved",
		"data":    processedInterns,
	})
}

// ScanQRCode handles scanning QR code and saving data to PostgreSQL
func ScanQRCode(c *fiber.Ctx) error {
	// Define a struct to parse JSON input
	type RequestBody struct {
		Interns []struct {
			User   model.User  ` json:"user"`
			Intern model.Intern `json:"intern"`
		} `json:"interns"`
	}

	var req RequestBody

	// Parse JSON body
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid JSON format",
			"error":   err.Error(),
		})
	}

	// Get current time in Asia/Manila
	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc).Format("15:04:05") // HH:mm:ss format
	fmt.Println("Current time:", currentTime)            // Optional: Log the current time

	// This will store successfully scanned intern data
	var scannedInterns []fiber.Map

	// Iterate over each intern to handle their data
	for _, intern := range req.Interns {
		// Validate required field
		if intern.Intern.ID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Intern ID is required",
			})
		}

		// Fetch the intern data along with associated user details
		var internData model.Intern
		if err := middleware.DBConn.Preload("User").First(&internData, intern.Intern.ID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Intern not found",
				"error":   err.Error(),
			})
		}

		// Validate if the user details from the request match the data in the database
		if intern.User.FirstName != internData.User.FirstName ||
			intern.User.MiddleName != internData.User.MiddleName ||
			intern.User.LastName != internData.User.LastName {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Provided data does not match the database values",
				"error":   "First Name, Middle Name, and Last Name mismatch",
			})
		}

		// Get the current month in "MM-DD-YY" format
		currentMonth := time.Now().In(loc).Format("01-02-06")

		// Set SupervisorID (Modify this if needed dynamically)
		supervisorID := uint(1)

		// Ensure TimeInAM is between 8:00 AM and 11:59 AM (current time as placeholder)
		timeInAM := currentTime

		// Create a new DTR entry (Only storing TimeInAM)
		dtrEntry := model.DTREntry{
			UserID:       internData.UserID,
			InternID:     internData.ID,
			SupervisorID: supervisorID,
			Month:        currentMonth,
			TimeInAM:     timeInAM,
			TimeOutAM:    "",
			TimeInPM:     "",
			TimeOutPM:    "",
			TotalHours:   "",
		}

		// Insert into the database for this intern
		if err := middleware.DBConn.Create(&dtrEntry).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to save DTR entry for intern ID " + fmt.Sprint(intern.Intern.ID),
				"error":   err.Error(),
			})
		}

		// Append scanned intern info to the response
		scannedInterns = append(scannedInterns, fiber.Map{
			"user": fiber.Map{
				"first_name":  internData.User.FirstName,
				"middle_name": internData.User.MiddleName,
				"last_name":   internData.User.LastName,
			},
			"intern": fiber.Map{
				"id":            internData.ID,
				"supervisor_id": internData.SupervisorID,
				"status":        internData.Status,
				"address":       internData.Address,
			},
		})
	}

	// Return success response with all scanned interns
	return c.JSON(fiber.Map{
		"message": "DTR entries for interns successfully saved",
		"data":    scannedInterns,
	})
}

// UpdateTimeOutAndIn updates the TimeOutAM, TimeInPM, and TimeOutPM for an intern
func UpdateTimeOutAM(c *fiber.Ctx) error {
	// Extract intern IDs from URL param (comma-separated)
	internIDs := c.Params("id") // Example: "4,3,2,1"
	if internIDs == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Intern IDs are required",
		})
	}

	// Split the IDs by comma to get a list of intern IDs
	idList := strings.Split(internIDs, ",")
	if len(idList) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid intern IDs format",
		})
	}

	// Get the current time in the Asia/Manila timezone
	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	timeOutAM := currentTime.Format("15:04:05")

	// Prepare to track any errors
	var errors []string

	// Loop through each intern ID
	for _, id := range idList {
		// Validate that the intern exists
		var intern model.Intern
		if err := middleware.DBConn.Table("interns").
			Where("id = ?", id).
			First(&intern).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Intern ID %s not found", id))
			continue // Skip this ID and continue to the next one
		}

		// Fetch the DTR entry for the intern
		var dtrEntry model.DTREntry
		currentMonth := time.Now().In(loc).Format("01-02-06") // MM-DD-YY format

		err := middleware.DBConn.Where("intern_id = ? AND month = ?", intern.ID, currentMonth).First(&dtrEntry).Error
		if err != nil {
			errors = append(errors, fmt.Sprintf("DTR entry not found for intern ID %s", id))
			continue
		}

		// Prepare data for update (only TimeOutAM)
		updateData := map[string]interface{}{
			"time_out_am": timeOutAM, // Automatically set current time
		}

		// Start a transaction
		tx := middleware.DBConn.Begin()

		// Update DTR entry with new TimeOutAM
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
		"message": fmt.Sprintf("DTR entries updated successfully for intern IDs %s with TimeOutAM %s", internIDs, timeOutAM),
	})
}

func UpdateTimeInPM(c *fiber.Ctx) error {
	// Extract intern IDs from URL param (comma-separated)
	internIDs := c.Params("id")
	if internIDs == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Intern IDs are required",
		})
	}

	// Split the IDs by comma to get a list of intern IDs
	idList := strings.Split(internIDs, ",")
	if len(idList) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid intern IDs format",
		})
	}

	// Get the current time in the Asia/Manila timezone
	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	timeInPM := currentTime.Format("15:04:05")

	// Prepare to track any errors
	var errors []string

	// Loop through each intern ID
	for _, id := range idList {
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
		currentMonth := time.Now().In(loc).Format("01-02-06") // MM-DD-YY format

		err := middleware.DBConn.Where("intern_id = ? AND month = ?", intern.ID, currentMonth).First(&dtrEntry).Error
		if err != nil {
			errors = append(errors, fmt.Sprintf("DTR entry not found for intern ID %s", id))
			continue
		}

		// Prepare data for update (only TimeInPM)
		updateData := map[string]interface{}{
			"time_in_pm": timeInPM, // Automatically set current time
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
	// Extract intern IDs from URL param (comma-separated)
	internIDs := c.Params("id")
	if internIDs == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Intern IDs are required",
		})
	}

	// Split the IDs by comma to get a list of intern IDs
	idList := strings.Split(internIDs, ",")
	if len(idList) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid intern IDs format",
		})
	}

	// Get the current time in the Asia/Manila timezone
	loc, _ := time.LoadLocation("Asia/Manila")
	currentTime := time.Now().In(loc)
	timeOutPM := currentTime.Format("15:04:05")

	// Prepare to track any errors
	var errors []string

	// Loop through each intern ID
	for _, id := range idList {
		// Validate that the intern exists
		var intern model.Intern
		if err := middleware.DBConn.Table("interns").
			Where("id = ?", id).
			First(&intern).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Intern ID %s not found", id))
			continue // Skip this ID and continue to the next one
		}

		// Fetch the DTR entry for the intern
		var dtrEntry model.DTREntry
		currentMonth := time.Now().In(loc).Format("01-02-06") // MM-DD-YY format

		err := middleware.DBConn.Where("intern_id = ? AND month = ?", intern.ID, currentMonth).First(&dtrEntry).Error
		if err != nil {
			errors = append(errors, fmt.Sprintf("DTR entry not found for intern ID %s", id))
			continue
		}

		// Convert the time fields from string to time.Time
		timeInAM, err := time.Parse("15:04:05", dtrEntry.TimeInAM)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Invalid TimeInAM format for intern ID %s", id))
			continue
		}

		timeOutAM, err := time.Parse("15:04:05", dtrEntry.TimeOutAM)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Invalid TimeOutAM format for intern ID %s", id))
			continue
		}

		timeInPM, err := time.Parse("15:04:05", dtrEntry.TimeInPM)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Invalid TimeInPM format for intern ID %s", id))
			continue
		}

		// Parse the TimeOutPM string into time.Time
		timeOutPMParsed, err := time.Parse("15:04:05", timeOutPM)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Invalid TimeOutPM format for intern ID %s", id))
			continue
		}

		// Calculate total hours worked (in decimal)
		totalAM := timeOutAM.Sub(timeInAM).Hours()
		totalPM := timeOutPMParsed.Sub(timeInPM).Hours()

		// Total hours worked in a day (decimal)
		totalHours := totalAM + totalPM

		// Convert decimal total hours into hour:minute:second format
		hours := int(totalHours)
		minutes := int((totalHours - float64(hours)) * 60)
		seconds := int(((totalHours-float64(hours))*60 - float64(minutes)) * 60)

		// Format total hours as "hh:mm:ss"
		totalHoursFormatted := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

		// Prepare data for update (only TimeOutPM)
		updateData := map[string]interface{}{
			"time_out_pm": timeOutPM,
			"total_hours": totalHoursFormatted, // Store the formatted total hours in hh:mm:ss
		}

		// Start a transaction
		tx := middleware.DBConn.Begin()

		// Update DTR entry with new TimeOutPM and totalHours
		if err := tx.Model(&dtrEntry).Updates(updateData).Error; err != nil {
			tx.Rollback() // Rollback if any error occurs
			errors = append(errors, fmt.Sprintf("Failed to update DTR entry for intern ID %s", id))
			continue
		}

		// Fetch all DTREntry records for the intern to sum up TotalHours
		var dtrEntries []model.DTREntry
		err = middleware.DBConn.Where("intern_id = ?", intern.ID).Find(&dtrEntries).Error
		if err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("Failed to fetch DTR entries for intern ID %s", id))
			continue
		}

		// Sum up all TotalHours and update OjtHoursRendered
		var totalRendered time.Duration
		for _, entry := range dtrEntries {
			totalHoursStr := entry.TotalHours
			// Parse TotalHours (hh:mm:ss) into time.Duration
			parts := strings.Split(totalHoursStr, ":")
			if len(parts) != 3 {
				errors = append(errors, fmt.Sprintf("Invalid TotalHours format for intern ID %s", id))
				continue
			}
			h, _ := strconv.Atoi(parts[0])
			m, _ := strconv.Atoi(parts[1])
			s, _ := strconv.Atoi(parts[2])
			totalRendered += time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second
		}

		// Update OjtHoursRendered
		totalRenderedStr := fmt.Sprintf("%02d:%02d:%02d", int(totalRendered.Hours()), int(totalRendered.Minutes())%60, int(totalRendered.Seconds())%60)
		err = middleware.DBConn.Model(&intern).Update("ojt_hours_rendered", totalRenderedStr).Error
		if err != nil {
			tx.Rollback()
			errors = append(errors, fmt.Sprintf("Failed to update OjtHoursRendered for intern ID %s", id))
			continue
		}

		// Commit the transaction if everything goes well
		tx.Commit()
	}

	// If there were errors, return them
	if len(errors) > 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Some updates failed",
			"errors":  errors,
		})
	}

	// Success message
	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("DTR entries updated successfully for intern IDs %s with TimeOutPM %s", internIDs, timeOutPM),
	})
}
