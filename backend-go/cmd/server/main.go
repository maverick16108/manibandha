package main

import (
	"log"
	"net/http"

	"manibandha/internal/config"
	"manibandha/internal/database"
	"manibandha/internal/security"
	"manibandha/internal/web"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}

	srv := &web.Server{
		DB:  db,
		Cfg: cfg,
		JWT: security.NewJWT(cfg.SecretKey),
	}

	addr := ":" + cfg.Port
	log.Printf("%s (Go) listening on %s, prefix %s", cfg.AppName, addr, cfg.APIPrefix)
	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Fatal(err)
	}
}
