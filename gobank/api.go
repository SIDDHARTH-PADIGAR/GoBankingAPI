package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

type apiFunc func(http.ResponseWriter, *http.Request) error

type ApiError struct {
	Error string `json:"error"`
}

func makeHTTPHandle(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			//handle error
			WriteJSON(w, http.StatusBadRequest, ApiError{Error: err.Error()})
		}
	}
}

type APIServer struct {
	listenAddr string
	store      Storage
}

func NewAPIServer(listenAddr string, store Storage) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
	}
}

func (s *APIServer) Run() {
	router := mux.NewRouter()

	router.HandleFunc("/login", makeHTTPHandle(s.handleLogin))
	router.HandleFunc("/account", makeHTTPHandle(s.handleAccount))
	router.HandleFunc("/account/{id}", http.HandlerFunc(withJWTAuth(makeHTTPHandle(s.handleGetAccountByID), s.store).ServeHTTP))
	router.HandleFunc("/transfer", makeHTTPHandle(s.handleTransfer))

	log.Println("JSON API server running on port:", s.listenAddr)

	if err := http.ListenAndServe(s.listenAddr, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// 885978
func (s *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "POST" {
		return fmt.Errorf("Method not allowed %s", r.Method)
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	acc, err := s.store.GetAccountByNumber(int64(req.Number))
	if err != nil {
		return err
	}

	if !acc.ValidatePassword(req.Password) {
		return fmt.Errorf("User not authenticated.")
	}

	token, err := createJWT(acc)
	if err != nil {
		return err
	}

	resp := LoginResponse{
		Number: acc.Number,
		Token:  token,
	}

	return WriteJSON(w, http.StatusOK, resp)
}

func (s *APIServer) handleAccount(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		return s.handleGetAccount(w, r)
	}

	if r.Method == "POST" {
		return s.handleCreateAccount(w, r)
	}

	return fmt.Errorf("method not allowed %s", r.Method)
}

func santizeAccount(account *Account) PublicAccount {
	return PublicAccount{
		ID:            account.ID,
		FirstName:     account.FirstName,
		LastName:      account.LastName,
		AccountNumber: account.Number,
		CreatedAt:     account.CreatedAt,
	}
}

// GET /acccount
func (s *APIServer) handleGetAccount(w http.ResponseWriter, r *http.Request) error {
	accounts, err := s.store.GetAccounts()
	if err != nil {
		return err
	}

	publicAccounts := make([]PublicAccount, len(accounts))
	for i, account := range accounts {
		publicAccounts[i] = santizeAccount(account)
	}

	return WriteJSON(w, http.StatusOK, publicAccounts)
}

func (s *APIServer) handleGetAccountByID(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {

		id, err := getID(r)
		if err != nil {
			return err
		}

		account, err := s.store.GetAccountbyID(id)
		if err != nil {
			return err
		}

		//db.get(id)

		return WriteJSON(w, http.StatusOK, account)
	}

	if r.Method == "DELETE" {
		return s.handleDeleteAccount(w, r)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (s *APIServer) handleCreateAccount(w http.ResponseWriter, r *http.Request) error {
	req := new(CreateAccountRequest)
	if err := json.NewDecoder((r.Body)).Decode(req); err != nil {
		return err
	}

	account, err := NewAccount(req.FirstName, req.LastName, req.Password)

	if err != nil {
		return err
	}

	// Extensive logging
	fmt.Printf("Account Creation Details:\n")
	fmt.Printf("First Name: %s\n", account.FirstName)
	fmt.Printf("Last Name: %s\n", account.LastName)
	fmt.Printf("Account Number: %d\n", account.Number)
	fmt.Printf("Created At: %v\n", account.CreatedAt)

	if err := s.store.CreateAccount(account); err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, account)
}

func (s *APIServer) handleDeleteAccount(w http.ResponseWriter, r *http.Request) error {
	id, err := getID(r)
	if err != nil {
		return err
	}
	if err := s.store.DeleteAccount(id); err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, map[string]int{"deleted": id})
}

func (s *APIServer) handleTransfer(w http.ResponseWriter, r *http.Request) error {
	//Validate request method
	if r.Method != "POST" {
		return fmt.Errorf("Method not allowed %s", r.Method)
	}

	//Parse transfer request
	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fmt.Errorf("Invalid request payload")
	}
	defer r.Body.Close()

	//Validate transfer request
	if err := s.validateTransfer(req); err != nil {
		return err
	}

	//Transaction execution
	transferResult, err := s.performTransfer(req)
	if err != nil {
		return err
	}

	//Transaction result
	return WriteJSON(w, http.StatusOK, transferResult)
}

func (s *APIServer) validateTransfer(req TransferRequest) error {
	// Validate if amount is positive
	if req.Amount <= 0 {
		return fmt.Errorf("transfer amount must be positive")
	}

	// Fetch source account
	fromAccount, err := s.store.GetAccountByNumber(req.FromAccountNumber)
	if err != nil {
		return fmt.Errorf("invalid source account")
	}

	// Fetch destination account
	toAccount, err := s.store.GetAccountByNumber(req.ToAccountNumber)
	if err != nil {
		return fmt.Errorf("invalid destination account")
	}

	// Prevent transfers to the same account
	if fromAccount.Number == toAccount.Number {
		return fmt.Errorf("cannot transfer to the same account")
	}

	// Check for sufficient balance
	if fromAccount.Balance < int64(req.Amount) {
		return fmt.Errorf("insufficient balance")
	}

	return nil
}

// Performing the actual transfer
func (s *APIServer) performTransfer(req TransferRequest) (map[string]interface{}, error) {
	log.Printf("Transfer Request - From: %d, To: %d, Amount: %f",
		req.FromAccountNumber, req.ToAccountNumber, req.Amount)
	// Fetch source and destination accounts by number
	fromAccount, err := s.store.GetAccountByNumber(int64(req.FromAccountNumber))
	if err != nil {
		return nil, fmt.Errorf("source account not found")
	}

	toAccount, err := s.store.GetAccountByNumber(int64(req.ToAccountNumber))
	if err != nil {
		return nil, fmt.Errorf("destination account not found")
	}

	// Begin database transaction
	tx, err := s.store.BeginTransaction()
	if err != nil {
		return nil, fmt.Errorf("could not begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Deduct from source account using its ID
	if err := s.store.UpdateAccountBalance(
		fromAccount.ID,
		-req.Amount,
		tx,
	); err != nil {
		return nil, fmt.Errorf("failed to deduct from source account: %v", err)
	}

	// Add to destination account using its ID
	if err := s.store.UpdateAccountBalance(
		toAccount.ID,
		req.Amount,
		tx,
	); err != nil {
		return nil, fmt.Errorf("failed to credit destination account: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transfer: %v", err)
	}

	// Prepare transfer receipt
	return map[string]interface{}{
		"status":         "success",
		"from_account":   req.FromAccountNumber,
		"to_account":     req.ToAccountNumber,
		"amount":         req.Amount,
		"transferred_at": time.Now(),
	}, nil
}

func getID(r *http.Request) (int, error) {
	idStr := mux.Vars(r)["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return id, fmt.Errorf("Invalid account ID %s", idStr)
	}

	return id, nil
}

func permissionDenied(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusForbidden, ApiError{Error: "Permission denied"})
}

func withJWTAuth(handler http.HandlerFunc, s Storage) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Calling withJWTAuth middleware")

		// Get the token from header
		tokenString := r.Header.Get("x-jwt-token")
		if tokenString == "" {
			permissionDenied(w, r)
			return
		}

		// Validate the token
		token, err := validateJWT(tokenString)
		if err != nil {
			fmt.Printf("JWT Validation Error: %v\n", err)
			permissionDenied(w, r)
			return
		}

		// Ensure token is valid
		if !token.Valid {
			fmt.Println("Token is not valid")
			permissionDenied(w, r)
			return
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			fmt.Println("Failed to parse claims")
			permissionDenied(w, r)
			return
		}

		// Get the account number from token claims
		tokenAccountNumber, ok := claims["accountNumber"].(float64)
		if !ok {
			fmt.Println("Failed to extract account number from claims")
			permissionDenied(w, r)
			return
		}

		// Get the requested account ID
		requestedID, err := getID(r)
		if err != nil {
			permissionDenied(w, r)
			return
		}

		// Find the account by ID
		account, err := s.GetAccountbyID(requestedID)
		if err != nil {
			permissionDenied(w, r)
			return
		}

		// Verify the account number matches the token's account number
		if int64(tokenAccountNumber) != account.Number {
			fmt.Printf("Token Account Number: %v, Requested Account Number: %v\n",
				tokenAccountNumber, account.Number)
			permissionDenied(w, r)
			return
		}

		// If all checks pass, proceed with the handler
		handler(w, r)
	})
}

func validateJWT(tokenString string) (*jwt.Token, error) {
	secret := os.Getenv("JWT_SECRET")
	fmt.Printf("Validating with secret: %s\n", secret)

	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Check signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})
}

func createJWT(account *Account) (string, error) {
	claims := jwt.MapClaims{
		"accountNumber": float64(account.Number),
		"expiresAt":     15000,
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET is not set")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	fmt.Printf("Created JWT Token:\n")
	fmt.Printf("Account Number: %d\n", account.Number)
	fmt.Printf("Token: %s\n", tokenString)

	return tokenString, nil
}

func seedAccountWithBalance(store Storage, accountNumber int64, initialBalance float64) error {
	// First, find the account by number
	account, err := store.GetAccountByNumber(accountNumber)
	if err != nil {
		return fmt.Errorf("account not found: %v", err)
	}

	// Begin a transaction
	tx, err := store.BeginTransaction()
	if err != nil {
		return fmt.Errorf("could not begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Update the account balance
	err = store.UpdateAccountBalance(account.ID, initialBalance, tx)
	if err != nil {
		return fmt.Errorf("failed to update account balance: %v", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit balance update: %v", err)
	}

	fmt.Printf("Successfully added %.2f to account %d\n", initialBalance, accountNumber)
	return nil
}
