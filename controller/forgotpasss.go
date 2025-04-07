package controller

import (
	"fmt"
	"log"
	"math/rand"
	"net/smtp"
	"os"
	"sync"
	"time"

	"intern_template_v1/middleware"
	"intern_template_v1/model"
	"intern_template_v1/model/response"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func init() {
	// Seed random number generator
	rand.Seed(time.Now().UnixNano())
}

// GenerateRandomCode creates a random 6-digit verification code
func GenerateRandomCode() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

// SendEmail sends a verification code to the specified email with expiration notice
func SendEmail(toEmail string, code string) error {
	from := os.Getenv("EMAIL_ADDRESS")
	password := os.Getenv("EMAIL_PASSWORD")

	auth := smtp.PlainAuth("", from, password, "smtp.gmail.com")
	to := []string{toEmail}
	subject := "Password Reset Verification Code"

	// Expiration message
	expirationTime := "5 minutes" // or use time.Now().Add(5*time.Minute).Format(...) for exact time

	body := fmt.Sprintf(`
		Your password reset verification code is: %s
		
		This code will expire in %s.

		If you did not request a password reset, please ignore this email.
	`, code, expirationTime)

	message := []byte("Subject: " + subject + "\r\n\r\n" + body)

	err := smtp.SendMail("smtp.gmail.com:587", auth, from, to, message)
	if err != nil {
		log.Println("Error sending email:", err)
		return err
	}

	return nil
}


// In-memory store for reset codes (email → code + expiry)
var resetCodeStore = struct {
	sync.RWMutex
	codes map[string]struct {
		Code      string
		ExpiresAt time.Time
	}
}{codes: make(map[string]struct {
	Code      string
	ExpiresAt time.Time
})}

// ForgotPassword handles forgot password request and sends a code
func ForgotPassword(c *fiber.Ctx) error {
	type ForgotPasswordRequest struct {
		Email string `json:"email"`
	}

	var req ForgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
		})
	}

	var user model.User

	result := middleware.DBConn.Table("users").Where("email = ?", req.Email).First(&user)
	if result.Error == gorm.ErrRecordNotFound {
		return c.Status(fiber.StatusNotFound).JSON(response.ResponseModel{
			RetCode: "404",
			Message: "Email not found",
		})
	} else if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.ResponseModel{
			RetCode: "500",
			Message: "Database error",
		})
	}

	code := GenerateRandomCode()

	if err := SendEmail(req.Email, code); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.ResponseModel{
			RetCode: "500",
			Message: "Error sending email",
		})
	}

	// Save the code with 5-minute expiration
	resetCodeStore.Lock()
	resetCodeStore.codes[req.Email] = struct {
		Code      string
		ExpiresAt time.Time
	}{
		Code:      code,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	resetCodeStore.Unlock()

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Verification code sent to your email",
	})
}

// VerifyResetCode handles code verification from user
func VerifyResetCode(c *fiber.Ctx) error {
	type VerifyRequest struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}

	var req VerifyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}

	resetCodeStore.RLock()
	data, exists := resetCodeStore.codes[req.Email]
	resetCodeStore.RUnlock()

	if !exists || time.Now().After(data.ExpiresAt) {
		return c.Status(fiber.StatusUnauthorized).JSON(response.ResponseModel{
			RetCode: "401",
			Message: "Code expired or not found",
		})
	}

	if data.Code != req.Code {
		return c.Status(fiber.StatusUnauthorized).JSON(response.ResponseModel{
			RetCode: "401",
			Message: "Incorrect code",
		})
	}

	// Set verified status for password reset (valid for another 15 mins)
	resetCodeStore.Lock()
	resetCodeStore.codes[req.Email] = struct {
		Code      string
		ExpiresAt time.Time
	}{Code: "VERIFIED", ExpiresAt: time.Now().Add(15 * time.Minute)}
	resetCodeStore.Unlock()

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Code verified. You may now reset your password.",
	})
}

// ResetPassword handles final password update after verification
func ResetPassword(c *fiber.Ctx) error {
	type ResetRequest struct {
		Email           string `json:"email"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}

	var req ResetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
		})
	}

	if req.NewPassword != req.ConfirmPassword {
		return c.Status(fiber.StatusBadRequest).JSON(response.ResponseModel{
			RetCode: "400",
			Message: "Passwords do not match",
		})
	}

	resetCodeStore.RLock()
	data, exists := resetCodeStore.codes[req.Email]
	resetCodeStore.RUnlock()

	if !exists || data.Code != "VERIFIED" || time.Now().After(data.ExpiresAt) {
		return c.Status(fiber.StatusUnauthorized).JSON(response.ResponseModel{
			RetCode: "401",
			Message: "Unauthorized or session expired",
		})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.ResponseModel{
			RetCode: "500",
			Message: "Password encryption failed",
		})
	}

	result := middleware.DBConn.Model(&model.User{}).Where("email = ?", req.Email).Update("password", string(hashedPassword))
	if result.Error == gorm.ErrRecordNotFound || result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(response.ResponseModel{
			RetCode: "404",
			Message: "User not found",
		})
	} else if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(response.ResponseModel{
			RetCode: "500",
			Message: "Database error",
		})
	}

	// Clean up the reset code
	resetCodeStore.Lock()
	delete(resetCodeStore.codes, req.Email)
	resetCodeStore.Unlock()

	return c.JSON(response.ResponseModel{
		RetCode: "200",
		Message: "Password successfully reset",
	})
}
