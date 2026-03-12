package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/f1bonacc1/ha-store/config"
	"github.com/f1bonacc1/ha-store/handlers"
	"github.com/f1bonacc1/ha-store/store"
)

func main() {
	// Setup zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg := config.Load()

	log.Info().Msgf("Connecting to NATS at %s with %d replicas", cfg.NATSURL, cfg.Replicas)
	s, err := store.New(cfg.NATSURL, cfg.Replicas)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to NATS")
	}
	defer s.Close()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		// Custom logger middleware using zerolog
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		log.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Str("duration", duration.String()).
			Msg("Request processed")
	})

	fileHandler := &handlers.FileHandler{
		Store:          s,
		ThrottleSpeed:  cfg.ThrottleSpeed,
		UploadDeadline: cfg.UploadDeadline,
		DeleteDeadline: cfg.DeleteDeadline,
	}

	// Routes (mTLS authentication is handled at TLS layer)
	r.PUT("/files/*path", fileHandler.HandlePutFile)
	r.GET("/files/*path", fileHandler.HandleGetFile)
	r.DELETE("/files/*path", fileHandler.HandleDeleteFile)

	r.PUT("/dirs/*path", fileHandler.HandlePutDir)
	r.GET("/dirs/*path", fileHandler.HandleListDir)
	r.DELETE("/dirs/*path", fileHandler.HandleDeleteDir)

	// Check if TLS is configured
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" && cfg.TLSCAFile != "" {
		// Load CA cert for client verification
		caCert, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to read CA certificate")
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			log.Fatal().Msg("Failed to parse CA certificate")
		}

		tlsConfig := &tls.Config{
			ClientCAs:  caCertPool,
			ClientAuth: tls.RequireAndVerifyClientCert,
			MinVersion: tls.VersionTLS12,
		}

		srv := &http.Server{
			Addr:      ":" + cfg.Port,
			Handler:   r,
			TLSConfig: tlsConfig,
		}

		go func() {
			log.Info().Msgf("Starting HTTPS server with mTLS on port %s", cfg.Port)
			if err := srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile); err != nil && err != http.ErrServerClosed {
				log.Fatal().Err(err).Msg("Failed to start server")
			}
		}()

		// Graceful shutdown
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Info().Msg("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal().Err(err).Msg("Server forced to shutdown")
		}
	} else {
		log.Warn().Msg("TLS not configured - running in insecure mode (for development only)")
		log.Warn().Msg("Set -tls-cert, -tls-key, and -tls-ca flags to enable mTLS")

		srv := &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: r,
		}

		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatal().Err(err).Msg("Failed to start server")
			}
		}()

		log.Info().Msgf("Server started on port %s (insecure mode)", cfg.Port)

		// Graceful shutdown
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Info().Msg("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal().Err(err).Msg("Server forced to shutdown")
		}
	}

	log.Info().Msg("Server exiting")
}
