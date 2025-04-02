package controller

import (
	"fmt"
	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

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
