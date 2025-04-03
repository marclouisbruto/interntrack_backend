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

// Secret key for signing tokens (should be stored in env variables)
var SecretKey = os.Getenv("SECRET_KEY")

// Mutex to handle concurrent writes to tokenBlacklist
var mu sync.Mutex

// Token blacklist map
var tokenBlacklist = make(map[string]bool)

// GenerateJWT generates a new JWT token
func GenerateJWT(userID uint) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = userID
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix() // Expires in 72 hours

	tokenString, err := token.SignedString([]byte(SecretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// BlacklistToken adds a token to the blacklist
func BlacklistToken(token string) {
	mu.Lock()
	defer mu.Unlock()
	tokenBlacklist[token] = true
}

// IsTokenBlacklisted checks if a token is in the blacklist
func IsTokenBlacklisted(token string) bool {
	mu.Lock()
	defer mu.Unlock()
	return tokenBlacklist[token]
}

// JWTMiddleware is used to protect routes
func JWTMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenString := c.Get("Authorization")

		if tokenString == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized: No token provided",
			})
		}

		// Remove "Bearer " prefix if present
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		// Check if token is blacklisted
		if IsTokenBlacklisted(tokenString) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized: Token has been invalidated",
			})
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(SecretKey), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized: Invalid token",
			})
		}

		c.Locals("user", token.Claims.(jwt.MapClaims)["user_id"])
		return c.Next()
	}
}
