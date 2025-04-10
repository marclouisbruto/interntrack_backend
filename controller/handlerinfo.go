package controller

import (
	//"errors"

	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"
	"strconv"

	//"regexp"

	"github.com/gofiber/fiber/v2"
)

// ADD NEW SUPERVISOR
func CreateHandler(c *fiber.Ctx) error {
	handlerId, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid user ID",
		})
	}

	handler := new(model.Handler)
	if err := c.BodyParser(handler); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": err.Error(),
		})
	}

	// Validate that the user exists and has role_id = 1 (Supervisor) or 3 (Adviser_OJT)
	var user model.User
	if err := middleware.DBConn.First(&user, handlerId).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "User not found",
		})
	}
	if user.RoleID == 1 || user.RoleID == 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "This user is not allowed to be a intern or supervisor!",
		})
	}

	// Check if supervisor already exists
	var existingHandler model.Handler
	if err := middleware.DBConn.Where("user_id = ?", handlerId).First(&existingHandler).Error; err == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "This handler already has a department assigned",
		})
	}

	// Assign values
	handler.UserID = uint(handlerId)
	handler.Status = "Approved" // Default status

	if err := middleware.DBConn.Create(handler).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to create Handler info",
		})
	}

	var createdHandler model.Handler
	if err := middleware.DBConn.
		Preload("User").
		Preload("User.Role").
		Preload("Supervisor").
		Preload("Supervisor.User").
		Preload("Supervisor.User.Role").
		First(&createdHandler, handler.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to fetch created Handler info",
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Handler successfully created",
		Data:    createdHandler,
	})
}
