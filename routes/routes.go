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
	internTrack.Post("/user/insert", controller.CreateUser)

	//Edit intern and supersivor
	internTrack.Put("/user/:id/edit-supervisor", controller.EditSupervisor)
	internTrack.Put("/user/:id/edit-intern", controller.EditIntern)

	//GET DATA
	internTrack.Get("/user/get/allinterns", controller.GetAllInterns)           //get all interns
	internTrack.Get("/user/get/intern/:id", controller.GetSingleIntern)         //get single intern
	internTrack.Get("/user/get/allsupervisors", controller.GetAllSupervisor)    //get all interns
	internTrack.Get("/user/get/supervisor/:id", controller.GetSingleSupervisor) //get single intern

	//archive data
	internTrack.Put("/user/archive/intern/:id", controller.ArchiveIntern)         //INTERNS
	internTrack.Put("/user/archive/supervisor/:id", controller.ArchiveSupervisor) //SUPERVISORS

	//Intern Registrtation
	app.Post("/user/intern/create", controller.RegisterIntern)

	//LOGIN PAGE
	app.Post("/login", controller.Login)
	internTrack.Post("/logout/", controller.Logout)


	// FORGOT PASSWORD
	app.Post("/forgot-password", controller.ForgotPassword)
	app.Post("/verify-code", controller.VerifyResetCode)
	app.Post("/reset-password", controller.ResetPassword)

	
	//Add leave request
	internTrack.Post("/leave-request/upload", controller.CreateLeaveRequest)
	internTrack.Static("/uploads/excuse_letters", "./uploads/excuse_letters") //tumutulong sa pang view or pathing ng image
	internTrack.Get("/view-excuse-letter/:filename", controller.ViewExcuseLetter)

	internTrack.Get("/interns/search/:value", controller.SearchInternsByParam)

	//change status
	internTrack.Put("/user/status/intern/:ids", controller.ApproveInterns)       //INTERNS
	internTrack.Put("/leave-request/status/:id", controller.ApproveLeaveRequest) //LEAVE REQUESTS

	//PROFILE PICTURE
	internTrack.Post("/user/upload/profile-picture/:id", controller.UploadProfilePicture) //UPLOAD PROFILE PICTURE
	internTrack.Get("/user/profile-picture/:id", controller.GetInternProfilePicture)      //VIEW PROFILE PICTURE
	internTrack.Put("/user/update/profile-picture/:id", controller.UpdateProfilePicture)  //UPDATE PROFILE


	//try endpoint
	internTrack.Get("/getinterns/handler/:supervisor_id", controller.GetInternsBySupervisorID)
}
