package main

import (
	"flag"
	"fmt"
	"log"
)

func seedAccount(store Storage, fname, lname, pw string) *Account {
	acc, err := NewAccount(fname, lname, pw)
	if err != nil {
		log.Fatal(err)
	}

	if err := store.CreateAccount(acc); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("New Account Created - ID: %d, Number: %d\n", acc.ID, acc.Number)

	// Add initial balance
	tx, err := store.BeginTransaction()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	initialBalance := 1000.00
	if err := store.UpdateAccountBalance(acc.ID, initialBalance, tx); err != nil {
		log.Fatalf("Failed to update account balance: %v", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}

	// Verify the balance after transaction
	updatedAccount, err := store.GetAccountByNumber(acc.Number)
	if err != nil {
		log.Fatalf("Failed to retrieve updated account: %v", err)
	}

	fmt.Printf("Account Balance After Seeding: $%.2f\n", float64(updatedAccount.Balance)/100)

	return acc
}

func seedAccounts(s Storage) {
	seedAccount(s, "Transfer", "Test", "transfer123")
}

func main() {
	seed := flag.Bool("seed", false, "seed the DB")
	flag.Parse()

	store, err := NewPostgresStorage()
	if err != nil {
		log.Fatal(err)
	}

	if err := store.init(); err != nil {
		log.Fatal(err)
	}

	if *seed {
		fmt.Println("Seeding DB...")
		//Seed stuff
		seedAccounts(store)
	}

	server := NewAPIServer(":8080", store)
	server.Run()
}
