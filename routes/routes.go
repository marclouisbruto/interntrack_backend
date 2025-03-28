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

	//Fill up all data in intern
	app.Post("/user/:id/create-intern", controller.CreateIntern)
	app.Post("/user/:id/create-supervisor", controller.CreateSuperVisor)
	app.Post("/user/intern/create", controller.InsertAllDataIntern)

	//FOR USER REGISTRATION
	app.Post("/user/insert", controller.CreateUser)
	// CREATE YOUR ENDPOINTS HERE

	// --------------------------
}
