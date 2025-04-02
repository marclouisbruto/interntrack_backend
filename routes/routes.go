package routes

import (
	"intern_template_v1/controller"
	"intern_template_v1/middleware"

	"github.com/gofiber/fiber/v2"
)

func AppRoutes(app *fiber.App) {
	// SAMPLE ENDPOINT
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello Golang World!")
	})

	//Grouped Routes for InternTrack API (need ng token)
	internTrack := app.Group("/api", middleware.JWTMiddleware())

	internTrack.Post("/role/insert", controller.CreateRole)

	//Additonal info for interns and supervisors
	internTrack.Post("/user/:id/create-intern", controller.CreateIntern)
	internTrack.Post("/user/:id/create-supervisor", controller.CreateSuperVisor)

	//FOR USER REGISTRATION
	app.Post("/user/insert", controller.CreateUser)

	//Edit intern and supersivor
	internTrack.Put("/user/:id/edit-supervisor", controller.EditSupervisor)
	internTrack.Put("/user/:id/edit-intern", controller.EditIntern)

	//GET DATA
	internTrack.Get("/user/get/allinterns", controller.GetAllInterns)           //get all interns
	internTrack.Get("/user/get/intern/:id", controller.GetSingleIntern)         //get single intern
	internTrack.Get("/user/get/allsupervisors", controller.GetAllSupervisor)    //get all interns
	internTrack.Get("/user/get/supervisor/:id", controller.GetSingleSupervisor) //get single intern

	//archive data
	internTrack.Put("/user/archive/intern/:id", controller.ArchiveIntern)         //get single intern
	internTrack.Put("/user/archive/supervisor/:id", controller.ArchiveSupervisor) //get single supervisor

	//Intern Registrtation
	app.Post("/user/intern/create", controller.InsertAllDataIntern)

	//login
	app.Post("/login", controller.Login)

	//Add leave request
	app.Post("/leave-request/upload", controller.CreateLeaveRequest)
	app.Static("/uploads/excuse_letters", "./uploads/excuse_letters")
	app.Get("/view-excuse-letter/:filename", controller.ViewExcuseLetter)

}
