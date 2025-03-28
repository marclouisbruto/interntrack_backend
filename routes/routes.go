package routes

import (
	"intern_template_v1/controller"

	"github.com/gofiber/fiber/v2"
)

func AppRoutes(app *fiber.App) {
	// SAMPLE ENDPOINT
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello Golang World!")
	})

	app.Post("/role/insert", controller.CreateRole)

	//FOR USER REGISTRATION
	app.Post("/user/insert", controller.CreateUser)
	// CREATE YOUR ENDPOINTS HERE

	// --------------------------
}
