package controller

import (
	"encoding/base64"
	"errors"
	"fmt"

	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"github.com/skip2/go-qrcode"
	"gorm.io/gorm"
)

type InternQR struct {
	User   model.User   `json:"user"`
	Intern model.Intern `json:"intern"`
}

// Load environment variables
func loadEnv() error {
	if err := godotenv.Load(".env"); err != nil {
		return errors.New("error loading .env file")
	}
	return nil
}

// Convert QR code to base64 directly
func generateQRCodeBase64(data string) (string, error) {
	// Generate the QR code and encode it to base64
	qrCode, err := qrcode.Encode(data, qrcode.Medium, 512)
	if err != nil {
		return "", fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Encode the QR code image to base64
	base64QRCode := base64.StdEncoding.EncodeToString(qrCode)

	return base64QRCode, nil
}

// InsertAllDataQRCode handles the QR code generation and storage for intern data
func InsertAllDataQRCode(c *fiber.Ctx) error {
	req := new(InternQR)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid input, unable to parse request body", "error": err.Error()})
	}

	// Validate required fields with more checks
	if req.Intern.ID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Intern ID is required"})
	}
	if req.User.FirstName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "First Name is required"})
	}
	if req.User.LastName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Last Name is required"})
	}
	if req.Intern.SupervisorID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Supervisor ID is required"})
	}

	var base64QRCode string

	// Transaction block
	err := middleware.DBConn.Transaction(func(tx *gorm.DB) error {
		// Fetch the intern and preload the associated User data
		var intern model.Intern
		if err := tx.Preload("User").First(&intern, req.Intern.ID).Error; err != nil {
			return fmt.Errorf("intern with ID %d does not exist: %w", req.Intern.ID, err)
		}

		// Validate that the SupervisorID in the Intern record matches an existing supervisor
		var supervisor model.Supervisor
		if err := tx.First(&supervisor, req.Intern.SupervisorID).Error; err != nil {
			return fmt.Errorf("supervisor with ID %d does not exist: %w", req.Intern.SupervisorID, err)
		}

		// Additional validation to ensure data matches
		if intern.User.FirstName != req.User.FirstName {
			return fmt.Errorf("first name mismatch for intern ID %d", req.Intern.ID)
		}
		if intern.User.MiddleName != req.User.MiddleName {
			return fmt.Errorf("middle name mismatch for intern ID %d", req.Intern.ID)
		}
		if intern.User.LastName != req.User.LastName {
			return fmt.Errorf("last name mismatch for intern ID %d", req.Intern.ID)
		}
		if intern.User.SuffixName != req.User.SuffixName {
			return fmt.Errorf("suffix name mismatch for intern ID %d", req.Intern.ID)
		}
		if intern.Status != req.Intern.Status {
			return fmt.Errorf("status mismatch for intern ID %d", req.Intern.ID)
		}
		if intern.Address != req.Intern.Address {
			return fmt.Errorf("address mismatch for intern ID %d", req.Intern.ID)
		}
		if intern.SupervisorID != req.Intern.SupervisorID {
			return fmt.Errorf("supervisor ID mismatch for intern ID %d", req.Intern.ID)
		}

		// Check if QR code already exists for this intern
		var existingQRCode model.QRCode
		if err := tx.Where("intern_id = ?", req.Intern.ID).First(&existingQRCode).Error; err == nil {
			// QR code already exists for this intern
			return fmt.Errorf("QRCode already exists for intern ID %d", req.Intern.ID)
		}

		// Conditionally append SuffixName
		suffixStr := ""
		if req.User.SuffixName != "" {
			suffixStr = fmt.Sprintf("SuffixName: %s\n", req.User.SuffixName)
		}

		// Generate QR code content
		qrCodeContent := fmt.Sprintf(
			"FirstName: %s\nMiddleName: %s\nLastName: %s\n%sSupervisorID: %d\nStatus: %s\nAddress: %s",
			req.User.FirstName,
			req.User.MiddleName,
			req.User.LastName,
			suffixStr,
			req.Intern.SupervisorID,
			req.Intern.Status,
			req.Intern.Address,
		)

		// Generate base64 encoded QR code
		base64QRCode, err := generateQRCodeBase64(qrCodeContent) // Declare err locally here
		if err != nil {
			return fmt.Errorf("failed to generate QR code: %w", err)
		}

		// Save the QR code data into the database
		qrCode := model.QRCode{
			InternID:     req.Intern.ID,
			QRCode:       qrCodeContent,
			Base64QRCode: base64QRCode,
		}
		if err := tx.Create(&qrCode).Error; err != nil {
			return fmt.Errorf("failed to insert QR code data into the database: %w", err)
		}

		return nil
	})

	// Handle any error during the transaction
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Transaction failed", "error": err.Error()})
	}

	// Return the response with the generated QR code
	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "QR code and intern data successfully added.",
		Data: fiber.Map{
			"qr_code": req.Intern.ID,
			"base64":  base64QRCode,
		},
	})
}
