package controller

import (
	//"errors"
	// "errors"
	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"

	//"strconv"

	//"regexp"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	//"gorm.io/gorm"
)

type InternRequest struct {
	User   model.User   `json:"user"`
	Intern model.Intern `json:"intern"`
}

func InsertAllDataIntern(c *fiber.Ctx) error {
	req := new(InternRequest) // assuming you have a combined struct for binding
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": err,
		})
	}

	if req.User.Password != req.User.ConfirmPassword {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Passwords do not match",
		})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.User.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to hash password",
			"error":   err.Error(),
		})
	}
	req.User.Password = string(hashedPassword)

	err = middleware.DBConn.Transaction(func(tx *gorm.DB) error {
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
