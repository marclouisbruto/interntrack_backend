package controller

import (
	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"
	
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

	// Check roleid and return appropriate response
	if user.RoleID == 1 {
		// Admin login
		return c.JSON(response.ResponseModel{
			RetCode: "200",
			Message: "Admin login successful",
			Data: fiber.Map{
				"token": token,
				"user": fiber.Map{
					"id":       user.ID,
					"email":    user.Email,
					"role":     "Supervisor",
				},
			},
		})
	} else if user.RoleID == 2 {
		// Regular user login
		return c.JSON(response.ResponseModel{
			RetCode: "200",
			Message: "User login successful",
			Data: fiber.Map{
				"token": token,
				"user": fiber.Map{
					"id":           user.ID,
					"email":        user.Email,
					"role":         "Intern",
				},
			},
		})
	}

	// If roleid is neither 1 nor 2, return unauthorized
	return c.Status(fiber.StatusUnauthorized).JSON(response.ResponseModel{
		RetCode: "401",
		Message: "Unauthorized role",
	})
}

func Logout(c *fiber.Ctx) error {
	token := c.Get("Authorization")
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "No token provided",
		})
	}

	// Remove "Bearer " prefix
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	// Blacklist the token
	middleware.BlacklistToken(token)

	return c.JSON(fiber.Map{
		"message": "Logout successful. Token is now invalidated.",
	})
}




