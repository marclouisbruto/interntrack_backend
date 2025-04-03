package controller

import (
	"errors"
	"fmt"
	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type InternRequest struct {
	User   model.User   `json:"user"`
	Intern model.Intern `json:"intern"`
}

func hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func InsertAllDataIntern(c *fiber.Ctx) error {
	req := new(InternRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
	}

	if req.User.Password != req.User.ConfirmPassword {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Passwords do not match"})
	}

	hashedPassword, err := hashPassword(req.User.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to hash password", "error": err.Error()})
	}
	req.User.Password = hashedPassword

	err = middleware.DBConn.Transaction(func(tx *gorm.DB) error {
		// Check if user with the same email already exists
		existingUser := model.User{}
		if err := tx.Where("email = ?", req.User.Email).First(&existingUser).Error; err == nil {
			return fiber.NewError(fiber.StatusConflict, "User with this email already exists")
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		// Insert user
		if err := tx.Create(&req.User).Error; err != nil {
			return err
		}

		// Link intern to user
		req.Intern.UserID = req.User.ID

		// Insert intern
		if err := tx.Create(&req.Intern).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Transaction failed", "error": err.Error()})
	}

	if err := middleware.DBConn.Preload("Role").First(&req.User, req.User.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve user with role",
			"error":   err.Error()})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Intern and User successfully added.",
		Data:    req,
	})
}

// EDIT INTERNS' DATA
func EditIntern(c *fiber.Ctx) error {
	id := c.Params("id")

	req := new(InternRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body",
			"error":   err.Error()})
	}

	if req.User.Password != "" {
		hashedPassword, err := hashPassword(req.User.Password)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to hash password",
				"error":   err.Error()})
		}
		req.User.Password = hashedPassword
	}

	err := middleware.DBConn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", id).Updates(req.User).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Intern{}).Where("user_id = ?", id).Updates(req.Intern).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Transaction failed",
			"error":   err.Error()})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Intern and User data successfully updated.",
		Data:    req,
	})
}

func GetAllInterns(c *fiber.Ctx) error {
	getAllInterns := []model.Intern{}
	if err := middleware.DBConn.Preload("User").Find(&getAllInterns).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve interns",
			"error":   err.Error()})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Interns retrieved successfully.",
		Data:    getAllInterns,
	})
}

func GetSingleIntern(c *fiber.Ctx) error {
	id := c.Params("id")
	singleIntern := new(model.Intern)
	if err := middleware.DBConn.Preload("User.Role").Preload("Supervisor").First(&singleIntern, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Intern not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve intern",
			"error":   err.Error()})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Intern retrieved successfully.",
		Data:    singleIntern,
	})
}

func ArchiveIntern(c *fiber.Ctx) error {
	id := c.Params("id")

	err := middleware.DBConn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Intern{}).Where("id = ?", id).Update("status", "Archived").Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to archive intern",
			"error":   err.Error()})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Intern successfully archived.",
	})
}

// PANG APPROVE NG INTERN
func ApproveInterns(c *fiber.Ctx) error {
	// Extract intern IDs from URL param or body (assuming comma-separated values)
	internIDs := c.Params("ids") // Example: "1,2,3" for multiple IDs
	if internIDs == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Intern IDs are required",
		})
	}

	// Split intern IDs into a slice
	idList := strings.Split(internIDs, ",")

	// Check if the interns exist and their status is pending
	var interns []model.Intern
	if err := middleware.DBConn.Table("interns").
		Where("id IN ? AND status = ?", idList, "Pending").
		Find(&interns).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Some interns not found or not in 'Pending' status",
		})
	}

	// Loop through each intern and update their status and intern ID
	for _, intern := range interns {
		// Generate custom intern ID
		internID, err := generateInternID(middleware.DBConn)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to generate intern ID",
			})
		}

		// Update intern status and assign generated intern ID
		if err := middleware.DBConn.Model(&intern).
			Updates(map[string]interface{}{
				"status":           "Approved",
				"custom_intern_id": internID,
			}).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to update intern with ID %d", intern.ID),
			})
		}
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Interns with IDs [%s] approved and assigned IDs successfully", internIDs),
	})
}

// SEARCH NG INTERNS
func SearchInternByName(c *fiber.Ctx) error {
	name := c.Params("name") // Get the name from the URL parameter

	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Name parameter is required",
		})
	}

	var interns []model.User // Searching in the users table

	// Filter users with role = 2 (Intern) and match name
	if err := middleware.DBConn.
		Where("role_id = ?", 2). // Ensure only interns are retrieved
		Where("CONCAT(first_name, ' ', last_name) ILIKE ?", "%"+name+"%").Preload("Role").
		Find(&interns).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to search interns",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Interns retrieved successfully.",
		Data:    interns,
	})
}

func generateInternID(db *gorm.DB) (string, error) {
	var lastIntern model.Intern
	var lastID int
	currentYear := time.Now().Year() // Get the current year

	// Find the latest custom intern ID for the current year
	if err := db.Table("interns").
		Select("custom_intern_id").
		Where("custom_intern_id LIKE ?", fmt.Sprintf("Intern-%d-%%", currentYear)).
		Order("custom_intern_id DESC").
		First(&lastIntern).Error; err != nil {
		// If no previous ID exists for this year, start from 1
		if errors.Is(err, gorm.ErrRecordNotFound) {
			lastID = 1
		} else {
			return "", err
		}
	} else if lastIntern.CustomInternID != nil { // Check if not nil before processing
		// Extract the last number (XXX part) and increment
		parts := strings.Split(*lastIntern.CustomInternID, "-")
		if len(parts) < 3 {
			return "", fmt.Errorf("invalid ID format: %s", *lastIntern.CustomInternID)
		}

		lastDigit, err := strconv.Atoi(parts[2]) // Extract the XXX part
		if err != nil {
			return "", fmt.Errorf("failed to parse last sequence number: %v", err)
		}
		lastID = lastDigit + 1
	} else {
		lastID = 1
	}

	// Format the new custom intern ID
	newInternID := fmt.Sprintf("Intern-%d-%03d", currentYear, lastID)

	return newInternID, nil
}
