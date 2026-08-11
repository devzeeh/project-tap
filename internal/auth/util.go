package auth

import (
	"crypto/rand"
	"log"
	"math/big"
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