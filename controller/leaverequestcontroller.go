package controller

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"
	"io"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// PANGGAWA LEAVE REQUEST
func CreateLeaveRequest(c *fiber.Ctx) error {
	internIDStr := c.FormValue("intern_id")
	internId, err := strconv.Atoi(internIDStr)
	if err != nil {
		fmt.Println("Error: Invalid intern ID, must be a number")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid intern",
		})
	}
	reason := c.FormValue("reason")
	leaveDateStr := c.FormValue("leave_date")

	if internId == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Intern is required",
		})
	}

	if reason == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Reason is required",
		})
	}

	if leaveDateStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Leave date is required",
		})
	}

	// Parse leaveDate with time zeroed (midnight)
	leaveDate, err := time.Parse("2006-01-02", leaveDateStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid leave date format. Use YYYY-MM-DD.",
		})
	}

	// Format the date to MM-DD-YYYY
	formattedDate := leaveDate.Format("01-02-2006")

	file, err := c.FormFile("excuse_letter")
	var base64Str string

	if err == nil {
		allowedExtensions := map[string]bool{
			".pdf":  true,
			".docx": true,
			".jpg":  true,
			".png":  true,
			".jpeg": true,
		}

		ext := filepath.Ext(file.Filename)
		if !allowedExtensions[ext] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid file format. Only PDF, DOCX, JPG, and PNG allowed.",
			})
		}

		fileData, err := file.Open()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Failed to open file.",
			})
		}
		defer fileData.Close()

		buffer := bytes.NewBuffer(nil)
		if _, err := io.Copy(buffer, fileData); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Error reading file content.",
			})
		}

		base64Str = base64.StdEncoding.EncodeToString(buffer.Bytes())
	}

	leaveRequest := model.LeaveRequest{
		InternID:     uint(internId),
		Reason:       reason,
		LeaveDate:    formattedDate, // Save date as formatted string
		ExcuseLetter: base64Str,     // Save as base64 string
		Status:       "Pending",
	}

	if err := middleware.DBConn.Debug().Table("leave_requests").Create(&leaveRequest).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Failed to add leave request",
		})
	}

	if err := middleware.DBConn.Preload("Intern").Preload("Intern.User").
		Preload("Intern.User.Role").
		Preload("Intern.Supervisor").Preload("Intern.Supervisor.User").Preload("Intern.Supervisor.User.Role").
		First(&leaveRequest, leaveRequest.ID).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Failed to load intern details",
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Leave request successfully added",
		Data:    leaveRequest,
	})
}

// PANG APPROVE NG LEAVE REQUEST
func ApproveLeaveRequest(c *fiber.Ctx) error {
	leaveRequestID := c.Params("id")

	var leaveRequest model.LeaveRequest
	if err := middleware.DBConn.
		Where("id = ?", leaveRequestID).
		First(&leaveRequest).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Leave request not found",
		})
	}

	if leaveRequest.Status != "Pending" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Leave request status is not pending",
		})
	}

	// ✅ Deduct from DTR total_hours now
	var dtr model.DTREntry
	if err := middleware.DBConn.
		Where("intern_id = ? AND DATE(created_at) = ?", leaveRequest.InternID, leaveRequest.LeaveDate).
		First(&dtr).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "DTR not found for that date",
		})
	}

	// Parse leaveHours from string to seconds
	parts := strings.Split(leaveRequest.LeaveHours, ":")
	hours, _ := strconv.Atoi(parts[0])
	mins, _ := strconv.Atoi(parts[1])
	secs, _ := strconv.Atoi(parts[2])
	leaveSeconds := (hours * 3600) + (mins * 60) + secs

	// Parse DTR total_hours
	totalSeconds := 0
	if dtr.TotalHours != "" {
		dtrParts := strings.Split(dtr.TotalHours, ":")
		if len(dtrParts) == 3 {
			h, _ := strconv.Atoi(dtrParts[0])
			m, _ := strconv.Atoi(dtrParts[1])
			s, _ := strconv.Atoi(dtrParts[2])
			totalSeconds = (h * 3600) + (m * 60) + s
		}
	}

	newTotal := totalSeconds - leaveSeconds
	if newTotal < 0 {
		newTotal = 0
	}
	dtr.TotalHours = fmt.Sprintf("%02d:%02d:%02d", newTotal/3600, (newTotal%3600)/60, newTotal%60)

	if err := middleware.DBConn.Save(&dtr).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update DTR",
		})
	}

	// Mark leave as approved
	if err := middleware.DBConn.Model(&leaveRequest).
		Update("status", "Approved").Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update leave request status",
		})
	}

	// Notify user
	var fcmData struct {
		FCMToken  string
		FirstName string
	}
	err := middleware.DBConn.Table("interns").
		Select("fcm_token, users.first_name").
		Joins("JOIN users ON users.users_id = interns.id").
		Where("interns.intern_id = ?", leaveRequest.InternID).
		Scan(&fcmData).Error

	if err != nil || fcmData.FCMToken == "" {
		log.Printf("⚠️ FCM token not found for intern ID %d", leaveRequest.InternID)
	} else {
		SendPushNotification(fcmData.FCMToken, "Leave Request Approved",
			fmt.Sprintf("Hi %s, your leave request has been approved.", fcmData.FirstName))
	}

	return c.JSON(fiber.Map{
		"message": "Leave request approved and total hours updated.",
	})
}


func GetLeaveRequests(c *fiber.Ctx) error {
	var leaveRequests []model.LeaveRequest

	status := c.Params("status")
	internIDStr := c.Params("intern_id")

	query := middleware.DBConn.
		Preload("Intern").
		Preload("Intern.User").
		Preload("Intern.User.Role").
		Preload("Intern.Supervisor").
		Preload("Intern.Supervisor.User").
		Preload("Intern.Supervisor.User.Role")

	// Apply filters based on params
	if status != "" && status != "intern" {
		query = query.Where("status ILIKE ?", status)
	}

	if internIDStr != "" {
		internID, err := strconv.Atoi(internIDStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid intern ID",
			})
		}
		query = query.Where("intern_id = ?", internID)
	}

	if err := query.Find(&leaveRequests).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to fetch leave requests",
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Leave requests fetched successfully",
		Data:    leaveRequests,
	})
}

func LeaveRequestOnDay(c *fiber.Ctx) error {
	loc, _ := time.LoadLocation("Asia/Manila")
	currentDate := time.Now().In(loc).Format("2006-01-02")

	internIDParam := c.Params("intern_id")
	internID, err := strconv.Atoi(internIDParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid intern ID",
		})
	}

	type LeaveRequestBody struct {
		LeaveRequestTime string `json:"leave_request_time"`
		ReturnInOJT      string `json:"return_in_ojt"`
		Reason           string `json:"reason"`
	}

	var body LeaveRequestBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	var dtr model.DTREntry
	if err := middleware.DBConn.
		Where("intern_id = ? AND DATE(created_at) = ?", internID, currentDate).
		First(&dtr).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "DTR entry not found for today",
		})
	}

	leaveTime, err1 := time.Parse("15:04:05", body.LeaveRequestTime)
	returnTime, err2 := time.Parse("15:04:05", body.ReturnInOJT)
	if err1 != nil || err2 != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid time format (should be HH:MM:SS)",
		})
	}

	duration := returnTime.Sub(leaveTime)
	if duration < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Return time must be after leave request time",
		})
	}

	leaveSeconds := int(duration.Seconds())
	lh := leaveSeconds / 3600
	lm := (leaveSeconds % 3600) / 60
	ls := leaveSeconds % 60
	leaveHoursStr := fmt.Sprintf("%02d:%02d:%02d", lh, lm, ls)

	// 🔒 Do NOT deduct from totalHours while status is pending

	// Create Leave Request
	newLeave := model.LeaveRequest{
		InternID:   uint(internID),
		LeaveDate:  currentDate,
		LeaveHours: leaveHoursStr,
		Reason:     body.Reason,
		Status:     "Pending",
	}

	if err := middleware.DBConn.Create(&newLeave).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to save leave request",
		})
	}

	return c.JSON(fiber.Map{
		"message":     "Leave request recorded successfully (pending approval)",
		"leave_hours": leaveHoursStr,
		"reason":      body.Reason,
		"leave_date":  currentDate,
	})
}


// func GetAllLeaveRequests(c *fiber.Ctx) error {
// 	var leaveRequests []model.LeaveRequest
// 	if err := middleware.DBConn.
// 		Preload("Intern").
// 		Preload("Intern.User").
// 		Preload("Intern.User.Role").
// 		Preload("Intern.Supervisor").
// 		Preload("Intern.Supervisor.User").
// 		Preload("Intern.Supervisor.User.Role").
// 		Find(&leaveRequests).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": "Failed to fetch leave requests",
// 		})
// 	}
// 	return c.JSON(response.ResponseModel{
// 		RetCode: "200",
// 		Message: "Leave requests fetched successfully",
// 		Data:    leaveRequests,
// 	})
// }

// func GetLeaveRequestsByStatus(c *fiber.Ctx) error {
// 	status := c.Params("status")
// 	var leaveRequests []model.LeaveRequest

// 	if err := middleware.DBConn.
// 		Where("status = ?", status).
// 		Preload("Intern").Preload("Intern.User").
// 		Preload("Intern.User.Role").
// 		Preload("Intern.Supervisor").Preload("Intern.Supervisor.User").
// 		Preload("Intern.Supervisor.User.Role").
// 		Find(&leaveRequests).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": "Failed to fetch leave requests",
// 		})
// 	}

// 	return c.JSON(response.ResponseModel{
// 		RetCode: "200",
// 		Message: "Leave requests fetched successfully",
// 		Data:    leaveRequests,
// 	})
// }

// func GetLeaveRequestsByIntern(c *fiber.Ctx) error {
// 	internIDStr := c.Params("intern_id")
// 	internID, err := strconv.Atoi(internIDStr)
// 	if err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"message": "Invalid intern ID",
// 		})
// 	}

// 	var leaveRequests []model.LeaveRequest

// 	if err := middleware.DBConn.
// 		Where("intern_id = ?", internID).
// 		Preload("Intern").Preload("Intern.User").
// 		Preload("Intern.User.Role").
// 		Preload("Intern.Supervisor").Preload("Intern.Supervisor.User").
// 		Preload("Intern.Supervisor.User.Role").
// 		Find(&leaveRequests).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": "Failed to fetch leave requests",
// 		})
// 	}

// 	return c.JSON(response.ResponseModel{
// 		RetCode: "200",
// 		Message: "Leave requests fetched successfully",
// 		Data:    leaveRequests,
// 	})
// }

// func GetLeaveRequestsByStatusAndIntern(c *fiber.Ctx) error {
// 	status := c.Params("status")
// 	internIDStr := c.Params("intern_id")
// 	internID, err := strconv.Atoi(internIDStr)
// 	if err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"message": "Invalid intern ID",
// 		})
// 	}

// 	var leaveRequests []model.LeaveRequest

// 	if err := middleware.DBConn.
// 		Where("status = ? AND intern_id = ?", status, internID).
// 		Preload("Intern").Preload("Intern.User").
// 		Preload("Intern.User.Role").
// 		Preload("Intern.Supervisor").Preload("Intern.Supervisor.User").
// 		Preload("Intern.Supervisor.User.Role").
// 		Find(&leaveRequests).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"message": "Failed to fetch leave requests",
// 		})
// 	}

// 	return c.JSON(response.ResponseModel{
// 		RetCode: "200",
// 		Message: "Leave requests fetched successfully",
// 		Data:    leaveRequests,
// 	})
// }
