package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"project-tap/internal/auth"
	"project-tap/internal/pkg/database"
	"project-tap/internal/pkg/storage"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	err := godotenv.Load("./.env")
	if err != nil {
		// Fallback: try loading from current directory
		if err := godotenv.Load(); err != nil {
			log.Fatalf("Error loading .env file: %v", err)
		}
	}

	// read .env VALUES
	port := os.Getenv("PORT")
	serverAddress := os.Getenv("SERVER_PORT")

	// Setup Database using the new database package
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	store := database.NewStore(db)

	// Initialize R2 Storage
	r2Storage, err := storage.NewR2Storage()
	if err != nil {
		log.Printf("Warning: Failed to initialize R2 storage (uploads may fail): %v", err)
	}

	// Initialize the Handler from the auth package
	authRepo := auth.NewRepository(store)
	authSvc := auth.NewService(authRepo, r2Storage)
	authHandler := auth.NewHandler(authSvc)

	// create mux
	mux := http.NewServeMux()
	// register routes
	auth.RegisterRoutes(mux, authHandler)

	// Start Server
	fmt.Println("Server started on: http://" + serverAddress + ":" + port)
	if err := http.ListenAndServe(serverAddress+":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
