package controller

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"intern_template_v1/middleware"
	"intern_template_v1/model"

	"github.com/gofiber/fiber/v2"
)

// Struct to hold school name and intern count
type SchoolAnalytics struct {
	SchoolName string `json:"school_name"`
	Count      int    `json:"count"`
}

type InternDetail struct {
	SchoolName string `json:"school_name"`
	Name       string `json:"name"`
	ID         uint   `json:"id"`
}

// GET: /analytics/school-count?school_name=SomeSchool
func DataAnalyticsSchoolCount(c *fiber.Ctx) error {
	schoolName := c.Params("school_name")
	var results []SchoolAnalytics

	query := middleware.DBConn.
		Model(&model.Intern{}).
		Select("school_name, COUNT(*) as count").
		Group("school_name")

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

	return c.JSON(fiber.Map{
		"message":      "School analytics fetched successfully.",
		"total_school": len(results),
		"data":         results,
	})
}

// GET: /interns/by-school?school_name=SomeSchool
func ShowDataInside(c *fiber.Ctx) error {
	schoolName := c.Params("school_name")

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

	var responseData []InternDetail
	for _, intern := range interns {
		user := intern.User

		// Optional: format name with middle initial and suffix
		middleInitial := ""
		if len(user.MiddleName) > 0 {
			middleInitial = string(user.MiddleName[0]) + "."
		}

		fullName := fmt.Sprintf("%s %s %s %s",
			user.FirstName,
			middleInitial,
			user.LastName,
			user.SuffixName,
		)

		responseData = append(responseData, InternDetail{
			SchoolName: intern.SchoolName,
			Name:       strings.TrimSpace(fullName),
			ID:         intern.ID,
		})
	}

	return c.JSON(fiber.Map{
		"data":    responseData,
		"retCode": "200",
		"message": "School analytics fetched successfully.",
		"count":   len(responseData),
	})
}

// Struct for simplified response
type InternStatus struct {
	InternID         uint   `json:"intern_id"`
	Name             string `json:"name"`
	OjtHoursRendered string `json:"OjtHoursRendered"`
	RemainingHours   string `json:"Remaining Hours"`
}

func CheckStatus(c *fiber.Ctx) error {
	status := c.Params("status")
	var dtrEntries []model.DTREntry

	// Get current date in Asia/Manila
	loc, _ := time.LoadLocation("Asia/Manila")
	today := time.Now().In(loc).Format("2006-01-02")

	// Fetch only today's entries with Intern and User data
	if err := middleware.DBConn.
		Preload("Intern.User").
		Where("DATE(created_at) = ?", today).
		Find(&dtrEntries).Error; err != nil {
		log.Println("Error fetching DTR entries:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to get DTR entries",
			"error":   err.Error(),
		})
	}

	var filteredInterns []InternStatus
	var presentInternIDs []uint

	for _, dtrEntry := range dtrEntries {
		presentInternIDs = append(presentInternIDs, dtrEntry.InternID)

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

			middleInitial := ""
			if len(user.MiddleName) > 0 {
				middleInitial = string(user.MiddleName[0]) + "."
			}

			fullName := fmt.Sprintf("%s %s %s %s",
				user.FirstName,
				middleInitial,
				user.LastName,
				user.SuffixName,
			)

			// Get OJT Hours Required
			ojtHoursRequired := dtrEntry.Intern.OjtHoursRequired

			// Fetch all today's DTR entries for this intern
			var internEntries []model.DTREntry
			if err := middleware.DBConn.
				Where("intern_id = ? AND DATE(created_at) = ?", dtrEntry.InternID, today).
				Find(&internEntries).Error; err != nil {
				log.Println("Error fetching intern's DTR entries:", err)
				continue
			}

			// Sum today's TotalHours
			var totalTodayDuration time.Duration
			for _, e := range internEntries {
				parts := strings.Split(e.TotalHours, ":")
				if len(parts) == 3 {
					h, _ := strconv.Atoi(parts[0])
					m, _ := strconv.Atoi(parts[1])
					s, _ := strconv.Atoi(parts[2])
					totalTodayDuration += time.Duration(h)*time.Hour +
						time.Duration(m)*time.Minute +
						time.Duration(s)*time.Second
				}
			}

			// Convert OjtHoursRendered from string to duration
			ojtRenderedParts := strings.Split(dtrEntry.Intern.OjtHoursRendered, ":")
			var overallRenderedDuration time.Duration
			if len(ojtRenderedParts) == 3 {
				h, _ := strconv.Atoi(ojtRenderedParts[0])
				m, _ := strconv.Atoi(ojtRenderedParts[1])
				s, _ := strconv.Atoi(ojtRenderedParts[2])
				overallRenderedDuration = time.Duration(h)*time.Hour +
					time.Duration(m)*time.Minute +
					time.Duration(s)*time.Second
			}

			// Add today's TotalHours to previously rendered hours
			totalRendered := overallRenderedDuration + totalTodayDuration

			// Compute remaining hours
			remaining := time.Duration(ojtHoursRequired)*time.Hour - totalRendered
			remainingStr := fmt.Sprintf("%02d:%02d:%02d",
				int(remaining.Hours()),
				int(remaining.Minutes())%60,
				int(remaining.Seconds())%60,
			)

			renderedStr := fmt.Sprintf("%02d:%02d:%02d",
				int(totalRendered.Hours()),
				int(totalRendered.Minutes())%60,
				int(totalRendered.Seconds())%60,
			)

			// Add intern data to response
			filteredInterns = append(filteredInterns, InternStatus{
				InternID:         dtrEntry.InternID,
				Name:             strings.TrimSpace(fullName),
				OjtHoursRendered: renderedStr,
				RemainingHours:   remainingStr,
			})
		}
	}

	// Handle no result cases
	if len(filteredInterns) == 0 {
		var message string

		switch status {
		case "late":
			message = "None of the interns is late."
		case "half-day-am":
			message = "No interns are on half-day (AM)."
		case "half-day-pm":
			message = "No interns are on half-day (PM)."
		case "full-day":
			// Get all intern IDs from database
			var allInterns []model.Intern
			if err := middleware.DBConn.Find(&allInterns).Error; err != nil {
				log.Println("Error fetching all interns:", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"message": "Failed to get intern data",
					"error":   err.Error(),
				})
			}

			var absentIDs []uint
			for _, intern := range allInterns {
				found := false
				for _, id := range presentInternIDs {
					if id == intern.ID {
						found = true
						break
					}
				}
				if !found {
					absentIDs = append(absentIDs, intern.ID)
				}
			}

			return c.JSON(fiber.Map{
				"message":  "Absent interns",
				"absentID": absentIDs,
				"retCode":  "200",
			})
		}

		return c.JSON(fiber.Map{
			"message": message,
			"retCode": "200",
		})
	}

	// Return filtered results
	return c.JSON(fiber.Map{
		"data":     filteredInterns,
		"retCode":  "200",
		"message":  "Attendance status fetched successfully.",
		"total_of": len(filteredInterns),
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

// Function to check the attendance for the first week of the current month
func CheckWeeklyLateInterns(c *fiber.Ctx) error {
	loc, _ := time.LoadLocation("Asia/Manila")
	now := time.Now().In(loc)

	// Get the start of the current week (Monday)
	offset := (int(now.Weekday()) + 6) % 7 // Make Monday = 0, Sunday = 6
	startOfWeek := now.AddDate(0, 0, -offset)
	startDate := time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, loc)

	// End of week = Sunday 23:59:59
	endDate := startDate.AddDate(0, 0, 7).Add(-time.Second)

	var dtrEntries []model.DTREntry
	if err := middleware.DBConn.
		Preload("Intern.User").
		Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Find(&dtrEntries).Error; err != nil {
		log.Println("Error fetching weekly DTR entries:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to get weekly DTR entries",
			"error":   err.Error(),
		})
	}

	// Count late occurrences per intern ID
	lateCount := make(map[uint]int)
	internInfo := make(map[uint]string)

	for _, entry := range dtrEntries {
		if checkLate(entry.TimeInAM) {
			lateCount[entry.InternID]++

			// Construct full name if not already stored
			if _, exists := internInfo[entry.InternID]; !exists {
				user := entry.Intern.User
				middleInitial := ""
				if len(user.MiddleName) > 0 {
					middleInitial = string(user.MiddleName[0]) + "."
				}
				fullName := fmt.Sprintf("%s %s %s %s",
					user.FirstName,
					middleInitial,
					user.LastName,
					user.SuffixName,
				)
				internInfo[entry.InternID] = strings.TrimSpace(fullName)
			}
		}
	}

	var result []fiber.Map
	for id, count := range lateCount {
		result = append(result, fiber.Map{
			"intern_id": id,
			"name":      internInfo[id],
			"late_days": count,
		})
	}

	if len(result) == 0 {
		return c.JSON(fiber.Map{
			"message": "No interns were late this week.",
			"retCode": "200",
		})
	}

	return c.JSON(fiber.Map{
		"message":  "Weekly late interns fetched successfully.",
		"retCode":  "200",
		"data":     result,
		"total_of": len(result),
	})
}

// Helper function to get the start and end of a given week in the current month
func getWeekRange(week int, month time.Month, year int) (time.Time, time.Time) {
	loc, _ := time.LoadLocation("Asia/Manila")

	// Start from March 31, 2025 for April 2025's 1st week
	start := time.Date(year, time.March, 31, 0, 0, 0, 0, loc)

	// Add (week - 1) * 7 days
	startOfWeek := start.AddDate(0, 0, (week-1)*7)

	// Default end is 4 days after (Mon to Fri)
	endOfWeek := startOfWeek.AddDate(0, 0, 4)

	// Ensure it doesn't go beyond April 30
	lastDayOfMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, loc)
	if endOfWeek.After(lastDayOfMonth) {
		endOfWeek = lastDayOfMonth
	}

	return startOfWeek, endOfWeek
}

// Function to check the attendance for a given week in the current month
func CheckMonthlyAttendance(c *fiber.Ctx) error {
	loc, _ := time.LoadLocation("Asia/Manila")
	now := time.Now().In(loc)
	year, month, _ := now.Date()

	weekParam := c.Params("week")
	var result []fiber.Map

	adjustToWeekdays := func(start, end time.Time) (time.Time, time.Time) {
		for start.Weekday() == time.Saturday || start.Weekday() == time.Sunday {
			start = start.AddDate(0, 0, 1)
		}
		for end.Weekday() == time.Saturday || end.Weekday() == time.Sunday {
			end = end.AddDate(0, 0, -1)
		}
		return start, end
	}

	weekNameMap := map[int]string{
		1: "first_week",
		2: "second_week",
		3: "third_week",
		4: "fourth_week",
		5: "fifth_week",
	}

	// Handle specific week request
	if weekParam != "" {
		var week int
		switch weekParam {
		case "first_week":
			week = 1
		case "second_week":
			week = 2
		case "third_week":
			week = 3
		case "fourth_week":
			week = 4
		case "fifth_week":
			week = 5
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid week parameter. Use first_week to fifth_week.",
			})
		}

		startOfWeek, endOfWeek := getWeekRange(week, month, year)
		startOfWeek, endOfWeek = adjustToWeekdays(startOfWeek, endOfWeek)

		var dtrEntries []model.DTREntry
		if err := middleware.DBConn.
			Preload("Intern.User").
			Where("created_at BETWEEN ? AND ?", startOfWeek, endOfWeek).
			Find(&dtrEntries).Error; err != nil {
			log.Println("Error fetching DTR entries:", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to get DTR entries",
				"error":   err.Error(),
			})
		}

		absentCount := 0
		alwaysLateCount := 0
		halfDayAMCount := 0
		halfDayPMCount := 0

		for _, entry := range dtrEntries {
			if checkLate(entry.TimeInAM) {
				alwaysLateCount++
			}
			switch checkHalfDay(entry.TimeInAM, entry.TimeInPM) {
			case "Half-Day (AM)":
				halfDayAMCount++
			case "Half-Day (PM)":
				halfDayPMCount++
			}
			if entry.TimeInAM == "" && entry.TimeInPM == "" {
				absentCount++
			}
		}

		result = append(result, fiber.Map{
			"absent":      absentCount,
			"always_late": alwaysLateCount,
			"half_day_am": halfDayAMCount,
			"half_day_pm": halfDayPMCount,
			"week":        weekParam,
			"week_range":  fmt.Sprintf("%s to %s", startOfWeek.Format("2006-01-02"), endOfWeek.Format("2006-01-02")),
		})

	} else {
		// Loop through 5 weeks (March 31 to April 30)
		for week := 1; week <= 5; week++ {
			startOfWeek, endOfWeek := getWeekRange(week, month, year)
			startOfWeek, endOfWeek = adjustToWeekdays(startOfWeek, endOfWeek)

			var dtrEntries []model.DTREntry
			if err := middleware.DBConn.
				Preload("Intern.User").
				Where("created_at BETWEEN ? AND ?", startOfWeek, endOfWeek).
				Find(&dtrEntries).Error; err != nil {
				log.Println("Error fetching DTR entries:", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"message": "Failed to get DTR entries",
					"error":   err.Error(),
				})
			}

			absentCount := 0
			alwaysLateCount := 0
			halfDayAMCount := 0
			halfDayPMCount := 0

			for _, entry := range dtrEntries {
				if checkLate(entry.TimeInAM) {
					alwaysLateCount++
				}
				switch checkHalfDay(entry.TimeInAM, entry.TimeInPM) {
				case "Half-Day (AM)":
					halfDayAMCount++
				case "Half-Day (PM)":
					halfDayPMCount++
				}
				if entry.TimeInAM == "" && entry.TimeInPM == "" {
					absentCount++
				}
			}

			result = append(result, fiber.Map{
				"absent":      absentCount,
				"always_late": alwaysLateCount,
				"half_day_am": halfDayAMCount,
				"half_day_pm": halfDayPMCount,
				"week":        weekNameMap[week],
				"week_range":  fmt.Sprintf("%s to %s", startOfWeek.Format("2006-01-02"), endOfWeek.Format("2006-01-02")),
			})
		}
	}

	return c.JSON(fiber.Map{
		"message": "Weekly attendance fetched successfully.",
		"data":    result,
		"retCode": "200",
	})
}

func GetAllDTREntries(c *fiber.Ctx) error {
	var dtrEntries []model.DTREntry

	// Fetch DTR entries with relations (Intern → User and Supervisor)
	if err := middleware.DBConn.
		Preload("Intern.User").
		Preload("Supervisor").
		Find(&dtrEntries).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve DTR entries",
			"error":   err.Error(),
		})
	}

	// Convert to response format
	var response []fiber.Map
	for _, entry := range dtrEntries {
		response = append(response, fiber.Map{
			"dtr_entry": fiber.Map{
				"id":          entry.ID,
				"month":       entry.Month,
				"total_hours": entry.TotalHours,
				"created_at":  entry.CreatedAt,
			},
			"user": fiber.Map{
				"first_name":  entry.Intern.User.FirstName,
				"middle_name": entry.Intern.User.MiddleName,
				"last_name":   entry.Intern.User.LastName,
				"suffix_name": entry.Intern.User.SuffixName,
			},
			"intern": fiber.Map{
				"id":           entry.InternID,
				"ojt_required": entry.Intern.OjtHoursRequired,
				"ojt_rendered": entry.Intern.OjtHoursRendered,
			},
		})
	}

	return c.JSON(fiber.Map{
		"message": "DTR entries fetched successfully",
		"data":    response,
	})
}