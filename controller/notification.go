package controller

import (
	"context"
	"fmt"
	"log"

	"intern_template_v1/config" // Import Firebase configuration

	"firebase.google.com/go/messaging"
)

// SendPushNotification sends a notification to a specific intern's device
func SendPushNotification(token string, title string, body string) error {
	// Get Firebase Messaging client
	client, err := config.FirebaseApp.Messaging(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get Firebase Messaging client: %w", err)
	}

	// Create notification payload
	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Token: token, // The intern's FCM token
	}

	// Send the notification
	response, err := client.Send(context.Background(), message)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	log.Printf("Successfully sent notification: %s\n", response)
	return nil
}
