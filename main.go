package main

import (
	"fmt"
	"intern_template_v1/middleware"
	"intern_template_v1/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func init() {
	fmt.Println("STARTING SERVER...")
	fmt.Println("INITIALIZE DB CONNECTION...")
	if middleware.ConnectDB() {
		fmt.Println("DB CONNECTION FAILED!")
	} else {
		fmt.Println("DB CONNECTION SUCCESSFUL!")
	}
}

func main() {
	app := fiber.New(fiber.Config{
		AppName: middleware.GetEnv("PROJ_NAME"),
	})

	// API ROUTES
	// Sample Endpoint
	// localhost:5566/check

	// Do not remove this endpoint
	app.Get("/favicon.ico", func(c *fiber.Ctx) error {
		return c.SendStatus(204) // No Content
	})

	routes.AppRoutes(app)
	// CORS CONFIG
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// LOGGER
	app.Use(logger.New())

	// Start Server
	app.Listen(fmt.Sprintf(":%s", middleware.GetEnv("PROJ_PORT")))
}
