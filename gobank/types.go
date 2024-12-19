package main

import (
	"math/rand"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Account struct {
	ID                int       `json:"id"`
	FirstName         string    `json:"first_name"`
	LastName          string    `json:"last_name"`
	Number            int64     `json:"account_number"`
	EncryptedPassword string    `json:"-"`
	Balance           int64     `json:"balance"`
	CreatedAt         time.Time `json:"created_at"`
}

func (a *Account) ValidatePassword(pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(a.EncryptedPassword), []byte(pw)) == nil

}

func NewAccount(firstName string, lastName string, password string) (*Account, error) {
	encpw, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return &Account{
		FirstName:         firstName,
		LastName:          lastName,
		Number:            int64(rand.Intn(1000000)),
		EncryptedPassword: string(encpw),
		CreatedAt:         time.Now().UTC(),
	}, nil
}

type CreateAccountRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Password  string `json:"password"`
}

// type TransferRequest struct {
// 	ToAccount int `json:"toAccount"`
// 	Amount    int `json:"amount"`
// }

type LoginRequest struct {
	Number   int64  `json:"number"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Number int64  `json:"number"`
	Token  string `json:"token"`
}

type PublicAccount struct {
	ID            int       `json:"id"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	AccountNumber int64     `json:"account_number"`
	CreatedAt     time.Time `json:"created_at"`
}

type TransferRequest struct {
	FromAccountNumber int64   `json:"fromAccount"`
	ToAccountNumber   int64   `json:"toAccount"`
	Amount            float64 `json:"amount"`
}
