package controller

import (
	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Login(c *fiber.Ctx) error {
	type LoginRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}

	var user model.User

	// Check credentials in the user table
	result := middleware.DBConn.Table("users").Where("email = ?", req.Email).First(&user)
	if result.Error == gorm.ErrRecordNotFound {
		return c.Status(fiber.StatusUnauthorized).JSON(response.ResponseModel{
			RetCode: "401",
			Message: "Invalid credentials",
		})
	} else if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.ResponseModel{
			RetCode: "500",
			Message: "Database error",
		})
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(response.ResponseModel{
			RetCode: "401",
			Message: "Invalid credentials",
		})
	}

	// Generate JWT
	token, err := middleware.GenerateJWT(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.ResponseModel{
			RetCode: "500",
			Message: "Error generating token",
		})
	}

	// Set cookie
	c.Cookie(&fiber.Cookie{
		Name:     "jwt",
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true, // Prevent JavaScript access
		Secure:   true, // Use true in production with HTTPS
		SameSite: "Lax",
	})

	// Role-based message
	role := ""
	message := ""
	switch user.RoleID {
	case 1:
		role = "Supervisor"
		message = "Admin login successful"
	case 2:
		role = "Intern"
		message = "User login successful"
	default:
		return c.Status(fiber.StatusUnauthorized).JSON(response.ResponseModel{
			RetCode: "401",
			Message: "Unauthorized role",
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: message,
		Data: fiber.Map{
			"user": fiber.Map{
				"id":    user.ID,
				"email": user.Email,
				"role":  role,
			},
		},
	})
}


func Logout(c *fiber.Ctx) error {
	// Clear the cookie
	c.Cookie(&fiber.Cookie{
		Name:     "jwt",
		Value:    "",
		Expires:  time.Now().Add(-time.Hour), // Expire it
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
	})

	return c.JSON(fiber.Map{
		"message": "Logout successful. Token removed from cookies.",
	})
}
