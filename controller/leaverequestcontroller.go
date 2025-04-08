package controller

import (
	"fmt"
	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

//PANGGAWA LEAVE REQUEST
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

	file, err := c.FormFile("excuse_letter")
	var filePathStr string

	if err == nil {
		allowedExtensions := map[string]bool{
			".pdf":  true,
			".docx": true,
			".jpg":  true,
			".png":  true,
		}

		ext := filepath.Ext(file.Filename)

		if !allowedExtensions[ext] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid file format. Only PDF, DOCX, JPG, and PNG allowed.",
			})
		}

		filename := fmt.Sprintf("excuse_%d_%d%s", time.Now().Unix(), internId, ext)
		filePathStr = filepath.Join("uploads/excuse_letters", filename)

		if err := c.SaveFile(file, filePathStr); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Unable to save file.",
			})
		}

	}

	leaveRequest := model.LeaveRequest{
		InternID:     uint(internId),
		Reason:       reason,
		ExcuseLetter: filePathStr,
		Status:       "Pending",
	}

	if err := middleware.DBConn.Debug().Table("leave_requests").Create(&leaveRequest).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Failed to add leave request",
		})
	}

	// Preload Intern details
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

// View excuse letter
func ViewExcuseLetter(c *fiber.Ctx) error {
	// Get the filename from the URL parameter
	filename := c.Params("filename")

	// Construct the file path
	filePath := filepath.Join("uploads/excuse_letters", filename)

	// Check if the file exists
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "File not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Error checking file existence",
		})
	}

	// Return the file
	return c.SendFile(filePath)
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
		FCMToken   string
		FirstName  string
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

