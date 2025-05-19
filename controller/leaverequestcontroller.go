package controller

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)


func UploadExcuseLetter(c *fiber.Ctx) error {
	// Parse leave request ID from URL param
	leaveRequestIDStr := c.Params("id")
	leaveRequestID, err := strconv.Atoi(leaveRequestIDStr)
	if err != nil || leaveRequestID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid leave request ID",
		})
	}

	// Get file from form
	file, err := c.FormFile("excuse_letter")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Excuse letter file is required",
		})
	}

	// Validate file extension
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

	// Open and read file
	fileData, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to open file",
		})
	}
	defer fileData.Close()

	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, fileData); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Error reading file content",
		})
	}

	// Encode file as base64
	base64Str := base64.StdEncoding.EncodeToString(buffer.Bytes())

	// Update the leave request with excuse letter
	if err := middleware.DBConn.
		Table("leave_requests").
		Where("id = ?", leaveRequestID).
		Update("excuse_letter", base64Str).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to upload excuse letter",
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Excuse letter uploaded successfully",
	})
}


// PANG APPROVE NG LEAVE REQUEST
func ApproveLeaveRequest(c *fiber.Ctx) error {
	leaveRequestID := c.Params("id")

	// Step 1: Fetch the leave request
	var leaveRequest model.LeaveRequest
	if err := middleware.DBConn.
		Where("id = ?", leaveRequestID).
		First(&leaveRequest).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Leave request not found",
		})
	}

	// Step 2: Check if status is "Pending"
	if leaveRequest.Status != "Pending" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Leave request status is not pending",
		})
	}

	// Step 3: Fetch the DTR entry for that leave date
	var dtr model.DTREntry
	if err := middleware.DBConn.
		Where("intern_id = ? AND DATE(created_at) = ?", leaveRequest.InternID, leaveRequest.LeaveDate).
		First(&dtr).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "DTR not found for that date",
		})
	}

	// Step 4: Parse Leave Hours
	leaveParts := strings.Split(leaveRequest.LeaveHours, ":")
	if len(leaveParts) != 3 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid leave hours format",
		})
	}
	leaveHours, _ := strconv.Atoi(leaveParts[0])
	leaveMins, _ := strconv.Atoi(leaveParts[1])
	leaveSecs, _ := strconv.Atoi(leaveParts[2])
	leaveTotalSecs := (leaveHours * 3600) + (leaveMins * 60) + leaveSecs

	// Step 5: Parse DTR total_hours
	dtrTotalSecs := 0
	if dtr.TotalHours != "" {
		dtrParts := strings.Split(dtr.TotalHours, ":")
		if len(dtrParts) == 3 {
			h, _ := strconv.Atoi(dtrParts[0])
			m, _ := strconv.Atoi(dtrParts[1])
			s, _ := strconv.Atoi(dtrParts[2])
			dtrTotalSecs = (h * 3600) + (m * 60) + s
		}
	}

	// Step 6: Update total hours
	newTotalSecs := dtrTotalSecs - leaveTotalSecs
	if newTotalSecs < 0 {
		newTotalSecs = 0
	}
	dtr.TotalHours = fmt.Sprintf("%02d:%02d:%02d",
		newTotalSecs/3600, (newTotalSecs%3600)/60, newTotalSecs%60)

	if err := middleware.DBConn.Save(&dtr).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update DTR",
		})
	}

	// Step 7: Mark as Approved
	if err := middleware.DBConn.Model(&leaveRequest).
		Update("status", "Approved").Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update leave request status",
		})
	}

	// Step 8: Notify intern via FCM
	var fcmData struct {
		Token     string
		FirstName string
	}
	err := middleware.DBConn.Table("interns").
		Select("token_requests.token, users.first_name").
		Joins("JOIN users ON users.id = interns.id").
		Joins("JOIN token_requests ON token_requests.id = interns.id").
		Where("interns.id = ?", leaveRequest.InternID).
		Scan(&fcmData).Error

	if err != nil || fcmData.Token == "" {
		fmt.Printf("⚠️ FCM token not found for intern ID %d, skipping notification\n", leaveRequest.InternID)
	} else {
		go func() {
			err := SendPushNotification(
				fcmData.Token,
				"Leave Request Approved",
				fmt.Sprintf("Hi %s, your leave request has been approved.", fcmData.FirstName),
			)
			if err != nil {
				fmt.Printf("⚠️ Failed to send notification to intern ID %d: %v\n", leaveRequest.InternID, err)
			}
		}()
	}

	// Final response
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

	// Check if a leave request already exists for today
	var existingLeave model.LeaveRequest
	if err := middleware.DBConn.
		Where("intern_id = ? AND leave_date = ?", internID, currentDate).
		First(&existingLeave).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"message": "A leave request already exists for today.",
		})
	}

	// Check if DTR entry exists
	var dtr model.DTREntry
	if err := middleware.DBConn.
		Where("intern_id = ? AND DATE(created_at) = ?", internID, currentDate).
		First(&dtr).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "DTR entry not found for today",
		})
	}

	// Parse leave and return times
	leaveTime, err1 := time.Parse("15:04:05", body.LeaveRequestTime)
	returnTime, err2 := time.Parse("15:04:05", body.ReturnInOJT)
	if err1 != nil || err2 != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid time format (should be HH:MM:SS)",
		})
	}

	// Calculate duration
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

	// Create new leave request
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
