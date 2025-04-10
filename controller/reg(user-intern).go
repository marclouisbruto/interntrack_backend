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

type SupervisorRequest struct {
	User       model.User       `json:"user"`
	Supervisor model.Supervisor `json:"supervisor"`
}

type HandlerRequest struct {
	User    model.User    `json:"user"`
	Handler model.Handler `json:"handler"`
}

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

//####################################
//==========ADD NEW DATA===========
//####################################

// ADD NEW USER
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

// ADD NEW INTERNS
func CreateIntern(c *fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid user ID",
		})
	}

	// Parse the body into intern
	intern := new(model.Intern)
	if err := c.BodyParser(intern); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": err.Error(),
		})
	}

	// Validate required fields
	if intern.SchoolName == "" || intern.Course == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "School Name and Course are required",
		})
	}

	// Check if user exists and has the correct role
	var user model.User
	if err := middleware.DBConn.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "User not found",
		})
	}
	if user.RoleID != 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "This user is not an Intern",
		})
	}

	// Set user_id
	intern.UserID = uint(userID)

	// Set default status to Approved
	intern.Status = "Approved"

	// Generate custom intern ID
	customID, err := generateInternID(middleware.DBConn)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to generate custom intern ID",
			"error":   err.Error(),
		})
	}
	intern.CustomInternID = &customID

	// Insert intern data
	if err := middleware.DBConn.Create(intern).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to create intern record",
			"error":   err.Error(),
		})
	}

	// Load with relations if needed (Supervisor)
	if err := middleware.DBConn.Preload("Supervisor").First(&intern, intern.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to load related data",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Intern successfully created with Approved status",
		Data:    intern,
	})
}

// ADD NEW SUPERVISOR
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
			"message": err.Error(),
		})
	}

	// Validate that the user exists and has role_id = 1 (Supervisor)
	var user model.User
	if err := middleware.DBConn.First(&user, superId).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "User not found",
		})
	}
	if user.RoleID != 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "This user is not allowed to be a supervisor!",
		})
	}

	// Check if supervisor already exists
	var existingSupervisor model.Supervisor
	if err := middleware.DBConn.Where("user_id = ?", superId).First(&existingSupervisor).Error; err == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "This supervisor already has a department assigned",
		})
	}

	// Assign values
	supervisor.UserID = uint(superId)
	supervisor.Status = "Approved" // Default status

	if err := middleware.DBConn.Create(supervisor).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to create Supervisor info",
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Supervisor successfully created",
		Data:    supervisor,
	})
}

// EDIT SUPERVISOR'S DATA
func EditSupervisor(c *fiber.Ctx) error {
	id := c.Params("id")

	req := new(SupervisorRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	err := middleware.DBConn.Transaction(func(tx *gorm.DB) error {
		// Update basic user data (excluding password)
		if err := tx.Model(&model.User{}).Where("id = ?", id).Omit("password").Updates(req.User).Error; err != nil {
			return err
		}

		// Update supervisor-specific data
		if err := tx.Model(&model.Supervisor{}).Where("user_id = ?", id).Updates(req.Supervisor).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Transaction failed",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Supervisor and User data successfully updated.",
		Data:    req,
	})
}

// EDIT HANDLER'S DATA
func EditHandler(c *fiber.Ctx) error {
	id := c.Params("id")

	req := new(HandlerRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	err := middleware.DBConn.Transaction(func(tx *gorm.DB) error {
		// Update basic user data (excluding password)
		if err := tx.Model(&model.User{}).Where("id = ?", id).Omit("password").Updates(req.User).Error; err != nil {
			return err
		}

		// Update handler-specific data
		if err := tx.Model(&model.Handler{}).Where("user_id = ?", id).Updates(req.Handler).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Transaction failed",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Handler and User data successfully updated.",
		Data:    req,
	})
}



//####################################
//========RETRIEVE SUPERVISOR========
//####################################

// GET ALL DATA OF SUPERVISOR
func GetAllSupervisor(c *fiber.Ctx) error {
	getAllSupervisor := []model.Intern{}

	err := middleware.DBConn.Preload("User").Find(&getAllSupervisor).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve Supervisor",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Supervisors retrieved successfully.",
		Data:    getAllSupervisor,
	})
}

// GET SINGLE SUPERVISOR
func GetSingleSupervisor(c *fiber.Ctx) error {
	id := c.Params("id") // Retrieve the ID from the route parameters

	singleSupervisor := new(model.Supervisor)

	err := middleware.DBConn.Preload("User").First(&singleSupervisor, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Supervisor not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve Supervisor",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Supervisor retrieved successfully.",
		Data:    singleSupervisor,
	})
}

// SORT BY HANDLE NG SUPERVISORS/HANDLERS
func GetInternsBySupervisorID(c *fiber.Ctx) error {
	supervisorID := c.Params("supervisor_id")

	if supervisorID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Supervisor ID is required",
		})
	}

	var interns []model.Intern
	if err := middleware.DBConn.
		Where("supervisor_id = ?", supervisorID).
		Preload("User").
		Find(&interns).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to retrieve interns for supervisor",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Interns retrieved successfully for the supervisor.",
		Data:    interns,
	})
}

// Archive Supervisor
func ArchiveSupervisor(c *fiber.Ctx) error {
	id := c.Params("id") // Retrieve the ID from the route parameters

	err := middleware.DBConn.Transaction(func(tx *gorm.DB) error {
		// Update Supervisor status to "archived"
		if err := tx.Model(&model.Supervisor{}).Where("id = ?", id).Update("status", "Archived").Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to archive Supervisor",
			"error":   err.Error(),
		})
	}

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Supervisor successfully archived.",
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
