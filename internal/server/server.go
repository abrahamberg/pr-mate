package server

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"prmate/internal/config"

	"github.com/gin-gonic/gin"
)

// Server wraps the HTTP server and provides lifecycle management
type Server struct {
	router *gin.Engine
	config *config.Config
	server *http.Server
}

// NewServer creates a new HTTP server with Gin
func NewServer(cfg *config.Config) *Server {
	gin.SetMode(cfg.GinMode)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	return &Server{
		router: router,
		config: cfg,
	}
}

// Router returns the Gin router for registering handlers
func (s *Server) Router() *gin.Engine {
	return s.router
}

// Start begins listening for HTTP requests
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%s", s.config.Port),
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	log.Printf("Starting server on port %s", s.config.Port)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down server...")
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}
