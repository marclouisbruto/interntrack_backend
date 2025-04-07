package middleware

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found")
	}
}

var (
	SecretKey       = os.Getenv("SECRET_KEY")
	tokenBlacklist  = make(map[string]bool)
	mu              sync.Mutex
)

// GenerateJWT creates a JWT token for the given user ID
func GenerateJWT(userID uint) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = userID
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix() // Token expires in 72 hours

	tokenString, err := token.SignedString([]byte(SecretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// VerifyJWT parses and validates a JWT token
func VerifyJWT(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure the token is using the correct signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(SecretKey), nil
	})
}

// BlacklistToken adds a JWT token to the in-memory blacklist (optional if using logout via cookie clear)
func BlacklistToken(token string) {
	mu.Lock()
	defer mu.Unlock()
	tokenBlacklist[token] = true
}

// IsTokenBlacklisted checks if the JWT is blacklisted
func IsTokenBlacklisted(token string) bool {
	mu.Lock()
	defer mu.Unlock()
	return tokenBlacklist[token]
}

// JWTMiddleware checks the cookie for a valid JWT and sets the user ID in context
func JWTMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Read the token from the "jwt" cookie
		tokenString := c.Cookies("jwt")

		if tokenString == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized: No token provided",
			})
		}

		// Optional: Check if the token is blacklisted
		if IsTokenBlacklisted(tokenString) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized: Token has been invalidated",
			})
		}

		token, err := VerifyJWT(tokenString)
		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized: Invalid token",
			})
		}

		// Set user_id to Fiber context
		claims := token.Claims.(jwt.MapClaims)
		c.Locals("user", claims["user_id"])

		return c.Next()
	}
}
