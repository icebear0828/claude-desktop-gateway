package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/local/claude-desktop-gateway/internal/config"
	"github.com/local/claude-desktop-gateway/internal/gateway"
)

func main() {
	cfg, err := config.LoadFromOSEnv()
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:    cfg.Address(),
		Handler: gateway.New(cfg, http.DefaultClient),
	}

	log.Printf("Claude Gateway listening on %s://%s", cfg.Scheme(), cfg.Address())
	if cfg.TLSEnabled() {
		err = server.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
	} else {
		err = server.ListenAndServe()
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
