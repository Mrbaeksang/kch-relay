package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Mrbaeksang/kch-relay/backend/internal/auth"
	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"os"
)

func main() {
	email := flag.String("email", "", "exact email of the pending HR application")
	flag.Parse()
	path := os.Getenv("DATABASE_PATH")
	if path == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_PATH is required")
		os.Exit(1)
	}
	db, err := database.Open(context.Background(), path)
	if err == nil {
		defer db.Close()
		err = auth.BootstrapAdmin(context.Background(), db, *email)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("First administrator application approved.")
}
