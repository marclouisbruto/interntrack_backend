package controller

import (
	"fmt"
	"log"
	"strings"
	"time"

	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"

	"github.com/gofiber/fiber/v2"
)

// Struct to hold school name and intern count
type SchoolAnalytics struct {
	SchoolName string `json:"school_name"`
	Count      int    `json:"count"`
}

// GET: /analytics/school-count?school_name=SomeSchool
func DataAnalyticsSchoolCount(c *fiber.Ctx) error {
	schoolName := c.Params("school_name") // param, optional
	var results []SchoolAnalytics

	query := middleware.DBConn.
		Model(&model.Intern{}).
		Select("school_name, COUNT(*) as count").
		Group("school_name")

	// Optional filter (search)
	if schoolName != "" {
		query = query.Where("school_name ILIKE ?", "%"+schoolName+"%")
	}

	if err := query.Scan(&results).Error; err != nil {
		log.Println("Error fetching school analytics:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to get school analytics",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "School analytics fetched successfully.",
		Data:    results,
	})
}

// GET: /interns/by-school?school_name=SomeSchool
func ShowDataInside(c *fiber.Ctx) error {
	schoolName := c.Params("school_name") // changed from Query to Params

	if schoolName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "School name is required.",
		})
	}

	var interns []model.Intern

	if err := middleware.DBConn.
		Preload("User").
		Where("school_name ILIKE ?", "%"+schoolName+"%").
		Find(&interns).Error; err != nil {
		log.Println("Error fetching interns by school:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to get intern data",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Intern data fetched successfully.",
		Data:    interns,
	})
}

func checkLate(timeIn string) bool {
	layout := "15:04:05"
	parsedTime, err := time.Parse(layout, timeIn)
	if err != nil {
		log.Println("Error parsing time:", err)
		return false
	}

	lateStart, _ := time.Parse(layout, "08:01:00")
	lateEnd, _ := time.Parse(layout, "12:00:00")

	return parsedTime.After(lateStart) && parsedTime.Before(lateEnd)
}

// Helper function to determine if intern is half-day
func checkHalfDay(timeInAM, timeInPM string) string {
	if timeInAM != "" && timeInPM == "" {
		return "Half-Day (AM)"
	}
	if timeInAM == "" && timeInPM != "" {
		return "Half-Day (PM)"
	}
	return "Full Day"
}

// Struct for simplified response
type InternStatus struct {
	InternID uint   `json:"intern_id"`
	Name     string `json:"name"`
}

// Route to check attendance status (late, half-day, full day)
func CheckStatus(c *fiber.Ctx) error {
	status := c.Params("status")
	var dtrEntries []model.DTREntry

	// Fetch all DTR entries with Intern and User data
	if err := middleware.DBConn.Preload("Intern.User").Find(&dtrEntries).Error; err != nil {
		log.Println("Error fetching DTR entries:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to get DTR entries",
			"error":   err.Error(),
		})
	}

	var filteredInterns []InternStatus
	for _, dtrEntry := range dtrEntries {
		var include bool

		switch status {
		case "late":
			include = checkLate(dtrEntry.TimeInAM)
		case "half-day-am":
			include = checkHalfDay(dtrEntry.TimeInAM, dtrEntry.TimeInPM) == "Half-Day (AM)"
		case "half-day-pm":
			include = checkHalfDay(dtrEntry.TimeInAM, dtrEntry.TimeInPM) == "Half-Day (PM)"
		case "full-day":
			include = checkHalfDay(dtrEntry.TimeInAM, dtrEntry.TimeInPM) == "Full Day"
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid status. Use one of: late, half-day-am, half-day-pm, full-day.",
			})
		}

		if include {
			user := dtrEntry.Intern.User

			// Optional: Add middle initial only
			middleInitial := ""
			if len(user.MiddleName) > 0 {
				middleInitial = string(user.MiddleName[0]) + "."
			}

			// Build formatted name
			fullName := fmt.Sprintf("%s %s %s %s",
				user.FirstName,
				middleInitial,
				user.LastName,
				user.SuffixName,
			)

			filteredInterns = append(filteredInterns, InternStatus{
				InternID: dtrEntry.InternID,
				Name:     strings.TrimSpace(fullName),
			})
		}
	}

	// Custom response with total count
	return c.JSON(fiber.Map{
		"data":      filteredInterns,
		"retCode":   "200",
		"message":   "Attendance status fetched successfully.",
		"total_of":  len(filteredInterns),
	})
}