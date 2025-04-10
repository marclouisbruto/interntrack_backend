package controller

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
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

// Convert image file to base64
func fileToBase64(fileName string) (string, error) {
	// Open the file
	file, err := os.Open(fileName)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read the file content
	fileContent, err := os.ReadFile(fileName)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Encode the file content to base64
	base64Str := base64.StdEncoding.EncodeToString(fileContent)

	return base64Str, nil
}

// GenerateQRCode generates a QR Code and saves it in the database
// GenerateQRCode generates a QR Code and saves it in the database
func GenerateQRCode(db *gorm.DB, internID uint, firstName, middleName, lastName, advisory string) (string, string, error) {
	// Ensure environment variables are loaded
	if err := loadEnv(); err != nil {
		return "", "", err
	}

	// Combine user data into a single string for QR code content
	data := fmt.Sprintf(
		"InternID: %d\nFirstName: %s\nMiddleName: %s\nLastName: %s\nAdvisory: %s",
		internID, firstName, middleName, lastName, advisory,
	)

	// Debugging: Print the data before generating QR Code
	log.Println("Generating QR Code with data:", data)

	// Check if the QR code already exists in the database
	var existingQRCode model.QRCode
	if err := db.Where("intern_id = ?", internID).First(&existingQRCode).Error; err == nil {
		// QR code already exists for this intern
		return "", "", fmt.Errorf("QRCode already exists")
	}

	// Generate QR code and save it as a PNG file
	fileName := fmt.Sprintf("%d_qrcode.png", internID)
	err := qrcode.WriteFile(data, qrcode.Medium, 512, fileName)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Debugging: Check if the file exists before converting to base64
	log.Println("Checking if file exists at:", fileName)
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		log.Println("QR Code file was not created at:", fileName) // Debugging log
		return "", "", errors.New("QR code file was not created")
	} else {
		log.Println("QR Code file exists at:", fileName) // Debugging log
	}

	// Convert the QR code image file to base64
	base64QRCode, err := fileToBase64(fileName)
	if err != nil {
		return "", "", fmt.Errorf("failed to convert QR code to base64: %w", err)
	}

	// Save QR data to the database using GORM
	qrCode := model.QRCode{
		InternID:     internID,
		QRCode:       data,         // Store QR code content
		Base64QRCode: base64QRCode, // Store base64 string
	}
	if err := db.Create(&qrCode).Error; err != nil {
		return "", "", fmt.Errorf("failed to insert QR code data into the database: %w", err)
	}

	return data, base64QRCode, nil
}

// InsertAllDataQRCode handles the QR code generation and storage for intern data
func InsertAllDataQRCode(c *fiber.Ctx) error {
	req := new(InternQR)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
	}

	// Validate required fields
	if req.Intern.ID == 0 || req.User.FirstName == "" || req.User.LastName == "" || req.Intern.SupervisorID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Missing required fields"})
	}

	// Validate InternID is not zero or empty
	if req.Intern.ID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "InternID cannot be empty"})
	}

	var base64QRCode string

	err := middleware.DBConn.Transaction(func(tx *gorm.DB) error {
		// Fetch the intern and preload the associated User data
		var intern model.Intern
		if err := tx.Preload("User").First(&intern, req.Intern.ID).Error; err != nil {
			return fmt.Errorf("intern with ID %d does not exist: %w", req.Intern.ID, err)
		}

		// Check if QR code already exists for this intern
		var existingQRCode model.QRCode
		if err := tx.Where("intern_id = ?", req.Intern.ID).First(&existingQRCode).Error; err == nil {
			// QR code already exists for this intern
			return errors.New("QRCode already exists")
		}

		// Generate and save the QR code
		qrCodeContent := fmt.Sprintf(
			"FirstName: %s\nMiddleName: %s\nLastName: %s\nSupervisorID: %d\nStatus: %s\nAddress: %s",
			req.User.FirstName,
			req.User.MiddleName,
			req.User.LastName,
			req.Intern.SupervisorID,
			req.Intern.Status,
			req.Intern.Address,
		)

		// Generate and save the QR code to file
		fileName := fmt.Sprintf("qrs/%d_qrcode.png", req.Intern.ID)
		err := qrcode.WriteFile(qrCodeContent, qrcode.Medium, 512, fileName)
		if err != nil {
			return fmt.Errorf("failed to generate QR code: %w", err)
		}

		// Convert the QR code to base64
		base64QRCode, err = fileToBase64(fileName)
		if err != nil {
			return fmt.Errorf("failed to convert QR code to base64: %w", err)
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

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Transaction failed", "error": err.Error()})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "QR code and intern data successfully added.",
		Data: fiber.Map{
			"qr_code": req.Intern.ID,
			"base64":  base64QRCode,
		},
	})
}