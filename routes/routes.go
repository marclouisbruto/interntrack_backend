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

	app.Post("/role/insert", controller.CreateRole)

	//Additonal info for interns and supervisors/ handlers
	internTrack.Post("/user/:id/create-intern", controller.CreateIntern)
	internTrack.Post("/user/:id/create-supervisor", controller.CreateSuperVisor)
	internTrack.Post("/user/:id/create-handler", controller.CreateHandler)

	//FOR USER REGISTRATION
	internTrack.Post("/user/insert", controller.CreateUser)

	//Edit intern and supersivor
	internTrack.Put("/user/:id/edit-supervisor", controller.EditSupervisor)
	internTrack.Put("/user/:id/edit-intern", controller.EditIntern)
	internTrack.Put("/user/:id/edit-handler", controller.EditHandler)
	internTrack.Put("/change-password/:id", controller.ChangePassword) //change password

	//GET DATA
	internTrack.Get("/user/get/allinterns", controller.GetAllInterns)           //get all interns
	internTrack.Get("/user/get/intern/:id", controller.GetSingleIntern)         //get single intern
	internTrack.Get("/user/get/allsupervisors", controller.GetAllSupervisor)    //get all interns
	internTrack.Get("/user/get/supervisor/:id", controller.GetSingleSupervisor) //get single intern
	internTrack.Get("/api/interns/approved", controller.GetAllApprovedInterns)  // get all approved interns
	internTrack.Get("/api/interns/pending", controller.GetAllPendingInterns)    // get all pending interns
	internTrack.Get("/api/interns/archived", controller.GetAllArchivedInterns)  // get all archived interns

	//archive data
	internTrack.Put("/user/archive/intern/:ids", controller.ArchiveInterns)       //INTERNS
	internTrack.Put("/user/archive/supervisor/:id", controller.ArchiveSupervisor) //SUPERVISORS


	//LOGIN PAGE
	app.Post("/login", controller.Login)
	internTrack.Post("/logout/", controller.Logout)
	app.Post("/register-intern", controller.RegisterIntern)

	// FORGOT PASSWORD
	app.Post("/forgot-password", controller.ForgotPassword)
	app.Post("/verify-code", controller.VerifyResetCode)
	app.Post("/reset-password", controller.ResetPassword)

	//Add leave request
	internTrack.Post("/leave-request/upload", controller.CreateLeaveRequest)
	internTrack.Static("/uploads/excuse_letters", "./uploads/excuse_letters") //tumutulong sa pang view or pathing ng image
	internTrack.Get("/view-excuse-letter/:filename", controller.ViewExcuseLetter)

	//SEARCH
	internTrack.Get("/interns/search/:value", controller.SearchInternsByParam)

	//APPROVE STATUS
	internTrack.Put("/user/status/intern/:ids", controller.ApproveInterns)       //INTERNS
	internTrack.Put("/leave-request/status/:id", controller.ApproveLeaveRequest) //LEAVE REQUESTS

	//PROFILE PICTURE
	internTrack.Post("/user/upload/profile-picture/:id", controller.UploadProfilePicture) //UPLOAD PROFILE PICTURE
	internTrack.Get("/user/profile-picture/:id", controller.GetInternProfilePicture)      //VIEW PROFILE PICTURE
	internTrack.Put("/user/update/profile-picture/:id", controller.UpdateProfilePicture)  //UPDATE PROFILE

	//SORT BY SUPERVISOR
	internTrack.Get("/getinterns/handler/:supervisor_id", controller.GetInternsBySupervisorID)

	//QR CODE
	internTrack.Post("/generate-qr", controller.InsertAllDataQRCode)
	internTrack.Post("/scan-qrcode", controller.ScanQRCode)
	internTrack.Post("/default/scan-qrcode", controller.DefaultTime)
	internTrack.Put("/update-dtr-update_out_am/:id", controller.UpdateTimeOutAM)
	internTrack.Put("/update-dtr-update_in_pm/:id", controller.UpdateTimeInPM)
	internTrack.Put("/update-dtr-update_out_pm/:id", controller.UpdateTimeOutPM)



	//EXORT
	internTrack.Get("/export/info", controller.ExportDataToPDF) // Export data to PDF
	internTrack.Get("/export/attendance", controller.ExportInternAttendanceToPDF) // Export attendance to PDF
	internTrack.Get("/printdtr/:id", controller.ExportDTRSheetToPDF)

	// Route for attendance status
	internTrack.Get("/attendance/check-status/:status", controller.CheckStatus)
	internTrack.Get("/analytics/frequent-late", controller.CheckWeeklyLateInterns)
	internTrack.Get("/analytics/frequent-late/:week", controller.CheckWeeklyLateInterns)
	internTrack.Get("/dtr/monthly-status", controller.CheckMonthlyAttendance)
	internTrack.Get("/dtr/monthly-status/:week", controller.CheckMonthlyAttendance)
	internTrack.Get("/dtr-entries", controller.GetAllDTREntries)
	internTrack.Get("/analytics/school-count", controller.DataAnalyticsSchoolCount)


	internTrack.Get("/getallattendance/:date", controller.GetInternAttendanceSummary)
}
