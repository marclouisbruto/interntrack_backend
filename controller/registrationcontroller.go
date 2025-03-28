package controller

import (
	//"errors"
	"errors"
	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"

	//"regexp"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SampleController is an example endpoint which returns a
// simple string message.
func SampleController(c *fiber.Ctx) error {
	return c.SendString("Hello, Golang World!")
}

// INSERT NEW ROLE
func CreateRole(c *fiber.Ctx) error {
	role := new(model.Role)
	if err := c.BodyParser(role); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": err,
		})
	}

	if err := middleware.DBConn.Debug().Table("roles").Create(role).Error; err != nil {
		return c.JSON(response.ResponseModel{
			RetCode: "500",
			Message: "Failed to add role",
			Data:    err,
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Role successfully added.",
		Data:    role,
	})
}

func CreateUser(c *fiber.Ctx) error {
	user := new(model.User)
	if err := c.BodyParser(user); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": err,
		})
	}

	if user.Password != user.ConfirmPassword {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Passwords do not match",
		})
	}

	//Ginamit si validatePassword function to set rules in creating password
	// if err := validatePassword(user.Password); err != nil {
	// 	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"message": err.Error(),
	// 	})
	// }

	//Check if may existing Phone number
	// var existingPhoneNumber model.User
	// if err := middleware.DBConn.Debug().Table("users").
	// 	Where("phone_number = ?", user.PhoneNumber).
	// 	First(&existingPhoneNumber).Error; err == nil {
	// 	return c.Status(fiber.StatusConflict).JSON(fiber.Map{
	// 		"message": "Phone number already exists",
	// 	})
	// }

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to hash password",
			"error":   err.Error(),
		})
	}
	user.Password = string(hashedPassword)

	var existingUser model.User
	result := middleware.DBConn.Debug().Table("users").Order("id DESC").First(&existingUser)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(response.ResponseModel{
			RetCode: "500",
			Message: "User already exists!",
			Data:    result.Error,
		})
	}

	if result.RowsAffected > 0 {
		user.ID = existingUser.ID + 1
	}

	if err := middleware.DBConn.Debug().Table("users").Create(user).Error; err != nil {
		return c.JSON(response.ResponseModel{
			RetCode: "500",
			Message: "Failed to add user",
			Data:    err,
		})
	}

	var newRole model.Role
	if err := middleware.DBConn.
		Preload("Role").
		First(&newRole, user.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve role details",
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "User successfully added.",
		Data:    user,
	})
}

// func validatePassword(password string) error {
// 	var passwordRegex = `^[A-Za-z\d@$!%*?&]{8,}$`
// 	matched, _ := regexp.MatchString(passwordRegex, password)

// 	if len(password) < 8 {
// 		return errors.New("password must be at least 8 characters long")
// 	}
// 	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
// 		return errors.New("password must include at least one uppercase letter")
// 	}
// 	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
// 		return errors.New("password must include at least one lowercase letter")
// 	}
// 	if !regexp.MustCompile(`\d`).MatchString(password) {
// 		return errors.New("password must include at least one number")
// 	}
// 	if !regexp.MustCompile(`[@$!%*?&]`).MatchString(password) {
// 		return errors.New("password must include at least one special character")
// 	}

// 	if !matched {
// 		return errors.New("password contains invalid characters")
// 	}
// 	return nil
// }
