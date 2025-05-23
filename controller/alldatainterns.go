package controller

import (
	"encoding/base64"
	"errors"
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

// CUSTOM INTERNID GENERATOR
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

// REGISTER INTERNS
func RegisterIntern(c *fiber.Ctx) error {
	req := new(InternRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
	}

	if req.User.Password != req.User.ConfirmPassword {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Passwords do not match"})
	}

	hashedPassword, err := hashPassword(req.User.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to hash password",
			"error":   err.Error(),
		})
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

		// Set user_id on intern
		req.Intern.UserID = req.User.ID

		// ✅ Set default status
		// ✅ Set default status and ojt_hours_rendered
		req.Intern.Status = "Pending"
		if req.Intern.OjtHoursRendered == "" {
			req.Intern.OjtHoursRendered = "0"
		}

		// Insert intern
		if err := tx.Create(&req.Intern).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Transaction failed",
			"error":   err.Error(),
		})
	}

	if err := middleware.DBConn.Preload("Role").First(&req.User, req.User.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve user with role",
			"error":   err.Error(),
		})
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
			"error":   err.Error(),
		})
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
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Intern and User data successfully updated.",
		Data:    req,
	})
}

type PasswordUpdateRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

func checkPassword(hashedPassword, plainPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}

// CHANGE PASSWORD
func ChangePassword(c *fiber.Ctx) error {
	id := c.Params("id")

	req := new(PasswordUpdateRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Check if new password matches confirmation
	if req.NewPassword != req.ConfirmPassword {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "New password and confirm password do not match",
		})
	}

	var user model.User
	if err := middleware.DBConn.First(&user, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "User not found",
			"error":   err.Error(),
		})
	}

	// Compare current password with hashed password in DB
	if err := checkPassword(user.Password, req.CurrentPassword); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Current password is incorrect",
		})
	}

	// Hash new password
	hashedPassword, err := hashPassword(req.NewPassword)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to hash new password",
			"error":   err.Error(),
		})
	}

	// Update password in DB
	if err := middleware.DBConn.Model(&user).Update("password", hashedPassword).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to update password",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Password updated successfully",
	})
}

//####################################
//==========RETRIEVE INTERNS==========
//####################################

// GET ALL INTERNS
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

// GET ALL APPROVED INTERNS
func GetAllApprovedInterns(c *fiber.Ctx) error {
	getAllInterns := []model.Intern{}
	if err := middleware.DBConn.Preload("User").Where("status = ?", "Approved").Find(&getAllInterns).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve approved interns",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Approved interns retrieved successfully.",
		Data:    getAllInterns,
	})
}

// GET ALL PENDING INTERNS
func GetAllPendingInterns(c *fiber.Ctx) error {
	getAllInterns := []model.Intern{}
	if err := middleware.DBConn.Preload("User").Where("status = ?", "Pending").Find(&getAllInterns).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve pending interns",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Pending interns retrieved successfully.",
		Data:    getAllInterns,
	})
}

// GET ALL ARCHIVED INTERNS
func GetAllArchivedInterns(c *fiber.Ctx) error {
	getAllInterns := []model.Intern{}
	if err := middleware.DBConn.Preload("User").Where("status = ?", "Archived").Find(&getAllInterns).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve archived interns",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Archived interns retrieved successfully.",
		Data:    getAllInterns,
	})
}

// SINGLE INTERN
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

//####################################
//==========CHANGE STATUS============
//####################################

// ARCHIVE MULTIPLE INTERNS
func ArchiveInterns(c *fiber.Ctx) error {
	internIDs := c.Params("ids") // Example: "1,2,3"
	if internIDs == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Intern IDs are required",
		})
	}

	idList := strings.Split(internIDs, ",")

	err := middleware.DBConn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Intern{}).
			Where("id IN ?", idList).
			Update("status", "Archived").Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to archive interns",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: fmt.Sprintf("Interns with IDs [%s] successfully archived.", internIDs),
	})
}

// PANG APPROVE NG INTERN
func ApproveInterns(c *fiber.Ctx) error {
	internIDs := c.Params("ids") // Example: "1,2,3"
	if internIDs == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Intern IDs are required",
		})
	}

	idList := strings.Split(internIDs, ",")

	// Get interns with pending status
	var interns []model.Intern
	if err := middleware.DBConn.Table("interns").
		Where("id IN ? AND status = ?", idList, "Pending").
		Find(&interns).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Some interns not found or not in 'Pending' status",
		})
	}

	for _, intern := range interns {
		// Generate custom intern ID
		internID, err := generateInternID(middleware.DBConn)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to generate intern ID",
			})
		}

		// Update intern status and custom intern ID
		if err := middleware.DBConn.Model(&intern).
			Updates(map[string]interface{}{
				"status":           "Approved",
				"custom_intern_id": internID,
			}).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to update intern with ID %d", intern.ID),
			})
		}

		// ✅ Fetch FCM token (adjust query if stored in separate table)
		var fcmToken string
		err = middleware.DBConn.Table("token_requests").
			Select("token").
			Where("id = ?", intern.ID).
			Scan(&fcmToken).Error

		if err != nil || fcmToken == "" {
			fmt.Printf("⚠️ FCM token not found for intern ID %d, skipping notification\n", intern.ID)
			continue
		}

		// ✅ Send Firebase notification
		title := "Internship Approved"
		body := fmt.Sprintf("Congratulations %s! Your internship request has been approved.", intern.User.FirstName)
		if err := SendPushNotification(fcmToken, title, body); err != nil {
			fmt.Printf("⚠️ Failed to send notification to intern ID %d: %v\n", intern.ID, err)
		}
	}

	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("Interns with IDs [%s] approved and notified successfully", internIDs),
	})
}

// SEARCH NG INTERNS
func SearchInternsByParam(c *fiber.Ctx) error {
	value := c.Params("value")

	if value == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Search value is required",
		})
	}

	var interns []model.Intern

	if err := middleware.DBConn.
		Preload("User").
		Where("school_name ILIKE ? OR id IN (?)",
			"%"+value+"%",
			middleware.DBConn.
				Model(&model.User{}).
				Select("id").
				Where("CONCAT(first_name, ' ', last_name) ILIKE ?", "%"+value+"%"),
		).
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

//SORT PER SCHOOL
func GetInternsBySchoolName(c *fiber.Ctx) error {
	schoolName := c.Params("school_name")

	if schoolName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "School name is required",
		})
	}

	var interns []model.Intern

	if err := middleware.DBConn.
		Preload("User").
		Where("school_name ILIKE ?", "%"+schoolName+"%").
		Order("school_name ASC").
		Find(&interns).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve interns by school name",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Interns filtered by school_name successfully.",
		Data:    interns,
	})
}




//####################################
//==========PROFILE PICTURE===========
//####################################

// PANG UPLOAD NG PROFILE PICTURE
func UploadProfilePicture(c *fiber.Ctx) error {
	internId := c.Params("id")

	file, err := c.FormFile("profile_picture")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Profile picture is required",
		})
	}

	// Validate file type (only PNG and JPG allowed)
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid file format. Only PNG and JPG allowed.",
		})
	}

	// Open the uploaded file
	fileContent, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to read file",
		})
	}
	defer fileContent.Close()

	// Read the file into a byte slice
	imageBytes, err := io.ReadAll(fileContent)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to read file content",
		})
	}

	// Encode file content to base64 once
	base64Image := base64.StdEncoding.EncodeToString(imageBytes)

	// Update the interns table with the base64 encoded image
	if err := middleware.DBConn.Table("interns").Where("id = ?", internId).Update("profile_picture", base64Image).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to upload profile picture",
		})
	}

	return c.JSON(fiber.Map{
		"message":   "Profile picture uploaded successfully",
		"intern_id": internId,
	})
}

// UPDATE PROFILE
func UpdateProfilePicture(c *fiber.Ctx) error {
	internId := c.Params("id")

	file, err := c.FormFile("profile_picture")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Profile picture is required",
		})
	}

	// Validate file type (only PNG and JPG allowed)
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid file format. Only PNG and JPG allowed.",
		})
	}

	// Open the uploaded file
	fileContent, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to read file",
		})
	}
	defer fileContent.Close()

	// Read the file into a byte slice
	imageBytes, err := io.ReadAll(fileContent)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to read file content",
		})
	}

	// Encode file content to base64
	base64Image := base64.StdEncoding.EncodeToString(imageBytes)

	// Update the interns table with the new profile picture
	if err := middleware.DBConn.Table("interns").Where("id = ?", internId).Update("profile_picture", base64Image).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to update profile picture",
		})
	}

	return c.JSON(fiber.Map{
		"message":   "Profile picture updated successfully",
		"intern_id": internId,
	})
}

// PANG RETRIEVE NG PROFILE PICTURE AS BASE64
func GetInternProfilePicture(c *fiber.Ctx) error {
	id := c.Params("id") // Kunin ang intern ID mula sa URL parameter

	// Struct para sa intern profile
	var intern model.Intern

	// Hanapin ang intern sa database at kunin ang profile_picture
	err := middleware.DBConn.Model(&model.Intern{}).Select("profile_picture").Where("id = ?", id).First(&intern).Error
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Intern not found or no profile picture available",
			"error":   err.Error(),
		})
	}

	// Kung walang profile picture, magbalik ng error
	if intern.ProfilePicture == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "No profile picture found",
		})
	}

	// Ibalik ang Base64 na image bilang JSON response
	return c.JSON(fiber.Map{
		"message": "Profile picture retrieved successfully",
		"data": fiber.Map{
			"base64": intern.ProfilePicture,
		},
	})
}
