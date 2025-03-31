package controller

import (
	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"

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
			"error": err.Error()})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Intern and User successfully added.",
		Data:    req,
	})
}

func EditIntern(c *fiber.Ctx) error {
	id := c.Params("id")

	req := new(InternRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body", 
			"error": err.Error()})
	}

	if req.User.Password != "" {
		hashedPassword, err := hashPassword(req.User.Password)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to hash password", 
				"error": err.Error()})
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
			"error": err.Error()})
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
			"error": err.Error()})
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
			"error": err.Error()})
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
			 "error": err.Error()})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Intern successfully archived.",
	})
}
