package admin

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"time"

	"project-tap/internal/pkg/database"
)

// Repository handles simple, reusable DB queries for the admin package.
type Repository struct {
	store database.Store
}

// NewRepository creates a new instance of Repository connected to the given store.
func NewRepository(store database.Store) *Repository {
	return &Repository{store: store}
}

// CardUIDExist returns true if the given card UID is already in the cards table.
func (r *Repository) CardUIDExist(uid string) (bool, error) {
	var existing string
	err := r.store.QueryRow("SELECT card_uid FROM cards WHERE card_uid = ?", uid).Scan(&existing)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GenerateCardNumber produces a card number in the format YYSS + 10 random
// digits + DD, localised to Asia/Manila.
func (r *Repository) GenerateCardNumber() string {
	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		loc = time.Local
	}
	now := time.Now().In(loc)

	yy := now.Format("06")
	ss := now.Format("05")
	dd := now.Format("02")

	max10 := big.NewInt(10_000_000_000)
	random10, errRand := rand.Int(rand.Reader, max10)

	digits := "0000000000"
	if errRand == nil {
		digits = fmt.Sprintf("%010d", random10.Int64())
	}
	return fmt.Sprintf("%s%s%s%s", yy, ss, digits, dd)
}