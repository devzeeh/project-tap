// Package signup provides helper functions for user signup operations.
// It includes functions for hashing passwords and checking email existence in the database.
// These functions are designed to be used in the signup process to ensure data integrity and security.
// The package relies on the "database/sql" package for database interactions
// and "golang.org/x/crypto/bcrypt" for secure password hashing.
// It is intended to be imported and used by other packages within the application.
package account

import (
	"database/sql"
	"log"

	"golang.org/x/crypto/bcrypt"
)

// # HashPassword (Helper function).
//
// This function takes a plain-text password and returns its bcrypt hash.
// It uses bcrypt's GenerateFromPassword function with a default cost.
// The hashed password is returned as a string.
// If an error occurs during hashing, it is returned along with an empty string.sdsd
func HashPassword(password string) (string, error) {
	// Generate bcrypt hash
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

// # Checking email existance (Helper function).
//
// This function checks if a given email already exists in the database.
// It executes a SQL query to search for the email in the users table.
// If the email is found, it returns true. If not found, it returns false.
// If an error occurs during the query, it returns the error.
func IsEmailExist(db *sql.DB, email string) (bool, error) {
	// Hold the existing email
	var existingEmail string

	// Check query
	query := "SELECT email FROM users WHERE email = ?"
	err := db.QueryRow(query, email).Scan(&existingEmail)
	if err == sql.ErrNoRows {
		log.Println("Email does not exist, Proceed")
		return false, nil
	}
	if err != nil {
		log.Printf("Email check error: %v", err)
		return false, err
	}
	log.Println("Email already exists.")
	return true, nil
}
