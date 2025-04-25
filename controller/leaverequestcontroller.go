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
	leaveRequestID := c.Params("id") // Extract leave request ID from URL param

	// Get the leave request details including intern ID and status
	var leaveRequest struct {
		ID       uint
		InternID uint
		Status   string
	}
	if err := middleware.DBConn.Table("leave_requests").
		Select("id, intern_id, status").
		Where("id = ?", leaveRequestID).
		First(&leaveRequest).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Leave request not found",
		})
	}

	// Check status
	if leaveRequest.Status != "Pending" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Leave request status is not pending",
		})
	}

	// Approve the request
	if err := middleware.DBConn.Table("leave_requests").
		Where("id = ?", leaveRequestID).
		Update("status", "Approved").Error; err != nil {
		log.Println("Failed to update leave request status:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update status",
		})
	}

	// 🔔 Fetch FCM token and intern's name
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
		log.Printf("⚠️ FCM token not found for intern ID %d, skipping notification\n", leaveRequest.InternID)
	} else {
		// 🔥 Send Firebase notification
		title := "Leave Request Approved"
		body := fmt.Sprintf("Hi %s, your leave request has been approved.", fcmData.FirstName)
		if err := SendPushNotification(fcmData.FCMToken, title, body); err != nil {
			log.Printf("⚠️ Failed to send notification to intern ID %d: %v\n", leaveRequest.InternID, err)
		}
	}

	return c.JSON(fiber.Map{
		"message": "Leave request status updated and notification sent",
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
