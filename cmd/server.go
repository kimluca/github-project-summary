// Command server starts the Codebase Archaeologist HTTP+GraphQL API.
//
// Env vars:
//
//	DATABASE_URL  Postgres DSN, e.g. postgres://user:pass@localhost:5432/archaeologist?sslmode=disable
//	PORT          defaults to 8080
package main

import (
	"log"
	"net/http"
	"os"

	"archaeologist/internal/db"
	"archaeologist/internal/graphql"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/archaeologist?sslmode=disable"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store, err := db.Open(dsn)
	if err != nil {
		log.Fatalf("could not connect to postgres: %v", err)
	}
	log.Println("connected to postgres and ensured schema")

	srv := &graphql.Server{Store: store}
	http.HandleFunc("/graphql", srv.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Printf("listening on :%s (POST /graphql)\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
