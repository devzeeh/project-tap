package auth

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"regexp"
	"strings"
	"time"
)

// CurrentTimestamp returns the current time formatted as "YYYY-MM-DD HH:MM:SS"
// in the Asia/Manila timezone.
func CurrentTimestamp() (string, error) {
	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		log.Printf("CurrentTimestamp: failed to load timezone: %v", err)
		return "", err
	}
	// Use .In(loc) rather than mutating the global time.Local
	return time.Now().In(loc).Format("2006-01-02 15:04:05"), nil
}

// GenerateUserID produces a cryptographically random 12-digit integer that
// does not already exist in the users table. It retries on collision.
func GenerateUserID(repo *Repository) (int64, error) {
	const min int64 = 100_000_000_000
	const max int64 = 999_999_999_999

	diff := new(big.Int).Sub(big.NewInt(max), big.NewInt(min))
	diff.Add(diff, big.NewInt(1))

	for {
		n, err := rand.Int(rand.Reader, diff)
		if err != nil {
			return 0, err
		}
		id := n.Int64() + min

		exists, err := repo.IsUserIDExist(id)
		if err != nil {
			return 0, err
		}
		if !exists {
			return id, nil
		}
		log.Println("GenerateUserID: collision detected, retrying...")
	}
}

// GenerateCustomerUsername derives a unique, clean alphanumeric username from
// the user's first name. If the username is already taken, a random numerical
// suffix is appended to prevent duplicate key collisions.
func GenerateCustomerUsername(repo *Repository, firstName string) (string, error) {
	reg := regexp.MustCompile(`[^a-zA-Z0-9]`)
	clean := strings.ToLower(reg.ReplaceAllString(firstName, ""))
	if clean == "" {
		clean = "user"
	}
	if len(clean) > 30 {
		clean = clean[:30]
	}

	exists, err := repo.IsUsernameExist(clean)
	if err != nil {
		return "", err
	}
	if !exists {
		return clean, nil
	}

	for i := 0; i < 10; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(9000))
		if err != nil {
			return "", err
		}
		candidate := fmt.Sprintf("%s%d", clean, n.Int64()+1000)
		exists, err := repo.IsUsernameExist(candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return fmt.Sprintf("%s%d", clean, time.Now().Unix()%100000), nil
}
