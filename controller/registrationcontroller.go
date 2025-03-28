package controller

import (
	//"errors"
	"errors"
	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"
	"strconv"

	//"regexp"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

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
	//pamfetch ng role_id
	if err := middleware.DBConn.Preload("Role").First(&user, user.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve user with role",
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "User successfully added.",
		Data:    user,
	})
}

// WORKING
func CreateIntern(c *fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid user ID",
		})
	}

	intern := new(model.Intern)
	if err := c.BodyParser(intern); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": err,
		})
	}

	// Confirm if the user exists and is an intern role
	var user model.User
	if err := middleware.DBConn.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "User not found",
		})
	}
	if user.RoleID != 4 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "This user is not an Intern",
		})
	}

	intern.UserID = uint(userID)

	if err := middleware.DBConn.Create(intern).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to create Intern info",
			"error":   err.Error(),
		})
	}

	// Reload with Preload to load relations
	if err := middleware.DBConn.Preload("Adviser").Preload("Supervisor").Preload("Handler").First(&intern, intern.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to load related data",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Intern successfully created",
		Data:    intern,
	})
}

func CreateSuperVisor(c *fiber.Ctx) error {
	superId, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid user ID",
		})
	}

	supervisor := new(model.Supervisor)
	if err := c.BodyParser(supervisor); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": err,
		})
	}

	// Validate that the user exists and has role_id = 1 (Super Visor)
	var user model.User
	if err := middleware.DBConn.First(&user, superId).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "User not found",
		})
	}
	if user.RoleID != 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "This user is not a Super Visor!",
		})
	}

	var existingSupervisor model.Supervisor
	if err := middleware.DBConn.Where("user_id = ?", superId).First(&existingSupervisor).Error; err == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "This supervisor already has a department assigned",
		})
	}

	// Assign the userID from params to handler
	supervisor.UserID = uint(superId)

	if err := middleware.DBConn.Create(supervisor).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to create Super Visor info",
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Super Visor successfully created",
		Data:    supervisor,
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
