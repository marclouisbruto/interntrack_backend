package config

import (
	"context"
	"log"

	firebase "firebase.google.com/go"
	"google.golang.org/api/option"
)

// FirebaseApp is a global variable for the Firebase app instance
var FirebaseApp *firebase.App

// InitializeFirebase initializes Firebase app
func InitializeFirebase() {
	opt := option.WithCredentialsFile("config/service-account.json") // Update path if needed
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatalf("Failed to initialize Firebase: %v", err)
	}

	FirebaseApp = app
	log.Println("Firebase initialized successfully!")
}
