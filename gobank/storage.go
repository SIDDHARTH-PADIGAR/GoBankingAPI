package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type Storage interface {
	CreateAccount(*Account) error
	DeleteAccount(int) error
	UpdateAccount(*Account) error
	GetAccounts() ([]*Account, error)
	GetAccountbyID(int) (*Account, error)
	GetAccountByNumber(int64) (*Account, error)
	BeginTransaction() (Transaction, error)
	UpdateAccountBalance(accountID int, amount float64, tx Transaction) error
}

type Transaction interface {
	Exec(qyeru string, args ...interface{}) (sql.Result, error)
	Commit() error
	Rollback() error
}

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage() (*PostgresStorage, error) {
	connStr := "user=postgres password=siddharth_22 dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &PostgresStorage{
		db: db,
	}, nil
}

func (s *PostgresStorage) init() error {
	if err := s.createAccountTable(); err != nil {
		return err
	}
	if err := s.ensureAccountNumberColumn(); err != nil {
		return err
	}
	return nil
}

func (s *PostgresStorage) createAccountTable() error {
	query := `create table if not exists account (
		id serial primary key,
		first_name varchar(100),
		last_name varchar(100),
		account_number serial,
		encrypted_password varchar(100),
		balance serial,
		created_at timestamp
	)`
	_, err := s.db.Exec(query)
	if err != nil {
		log.Printf("Error creating account table: %v", err)
	} else {
		log.Println("Account table created successfully or already exists.")
	}
	return err
}

func (s *PostgresStorage) ensureAccountNumberColumn() error {
	query := `
	DO $$ BEGIN
		IF NOT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_name = 'account' AND column_name = 'account_number'
		) THEN
			ALTER TABLE account ADD COLUMN account_number serial;
		END IF;
	END $$;
	`
	_, err := s.db.Exec(query)
	if err != nil {
		log.Printf("Error ensuring account_number column: %v", err)
	} else {
		log.Println("Account_number column exists or was added successfully.")
	}
	return err
}

func (s *PostgresStorage) CreateAccount(acc *Account) error {

	if acc.CreatedAt.IsZero() {
		acc.CreatedAt = time.Now()
	}

	query := `insert into account 
	(first_name, last_name, account_number, encrypted_password, balance, created_at)
	values ($1, $2, $3, $4, $5, $6)`

	_, err := s.db.Query(
		query,
		acc.FirstName,
		acc.LastName,
		acc.Number,
		acc.EncryptedPassword,
		acc.Balance,
		acc.CreatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (s *PostgresStorage) GetAccountByNumber(number int64) (*Account, error) {
	log.Printf("Attempting to find account with number: %d", number)

	// Use QueryRow instead of Query to ensure single row
	row := s.db.QueryRow("SELECT id, first_name, last_name, account_number, encrypted_password, balance, created_at FROM account WHERE account_number = $1", number)

	account := &Account{}

	// Explicitly declare variables for each column
	var (
		id                int
		firstName         string
		lastName          string
		accountNumber     int64
		encryptedPassword string
		balance           int64
		createdAt         time.Time
	)

	// Scan into explicit variables
	err := row.Scan(
		&id,
		&firstName,
		&lastName,
		&accountNumber,
		&encryptedPassword,
		&balance,
		&createdAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No account found with number: %d", number)
			return nil, fmt.Errorf("account with number [%d] not found", number)
		}

		log.Printf("Error scanning account: %v", err)
		return nil, err
	}

	// Manually construct the account
	account.ID = int(id)
	account.FirstName = firstName
	account.LastName = lastName
	account.Number = accountNumber
	account.EncryptedPassword = encryptedPassword
	account.Balance = balance
	account.CreatedAt = createdAt

	log.Printf("Found account: ID=%d, Number=%d", account.ID, account.Number)

	return account, nil
}

func (s *PostgresStorage) UpdateAccount(*Account) error {
	return nil
}

func (s *PostgresStorage) DeleteAccount(id int) error {
	_, err := s.db.Query("DELETE FROM account WHERE id = $1", id)

	return err
}

func (s *PostgresStorage) GetAccountbyID(id int) (*Account, error) {
	row := s.db.QueryRow("SELECT id, first_name, last_name, account_number, encrypted_password, balance, created_at FROM account WHERE id = $1", id)

	account := &Account{}
	err := row.Scan(
		&account.ID,
		&account.FirstName,
		&account.LastName,
		&account.Number,
		&account.EncryptedPassword,
		&account.Balance,
		&account.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account with id %d not found", id)
		}
		log.Printf("Get Account by ID Scan Error: %v", err)
		return nil, err
	}

	return account, nil
}

func (s *PostgresStorage) GetAccounts() ([]*Account, error) {
	rows, err := s.db.Query("SELECT id, first_name, last_name, account_number, encrypted_password, balance, created_at FROM account")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := []*Account{}
	for rows.Next() {
		account := &Account{}
		err := rows.Scan(
			&account.ID,
			&account.FirstName,
			&account.LastName,
			&account.Number,
			&account.EncryptedPassword,
			&account.Balance,
			&account.CreatedAt,
		)

		if err != nil {
			log.Printf("Individual Account Scan Error: %v", err)
			continue
		}
		accounts = append(accounts, account)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return accounts, nil
}

func scanIntoAccount(rows *sql.Rows) (*Account, error) {
	account := new(Account)
	err := rows.Scan(
		&account.ID,
		&account.FirstName,
		&account.LastName,
		&account.Number,
		&account.EncryptedPassword,
		&account.Balance,
		&account.CreatedAt,
	)

	if err != nil {
		log.Printf("Scan Error Details: %+v", err)
		log.Printf("Error Type: %T", err)
		return nil, fmt.Errorf("scan error: %v", err)
	}

	return account, nil
}

func (s *PostgresStorage) BeginTransaction() (Transaction, error) {
	return s.db.Begin()
}

func (s *PostgresStorage) UpdateAccountBalance(accountID int, amount float64, tx Transaction) error {
	// Convert float64 to int64 cents to avoid floating point precision issues
	amountInCents := int64(amount * 100)

	query := "UPDATE account SET balance = balance + $1 WHERE id = $2"

	var err error
	if tx != nil {
		_, err = tx.Exec(query, amountInCents, accountID)
	} else {
		_, err = s.db.Exec(query, amountInCents, accountID)
	}

	if err != nil {
		return fmt.Errorf("failed to update account balance: %v", err)
	}

	return nil
}
