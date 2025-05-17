package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/teresa-solution/connection-pool-manager/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Config holds the application configuration
type Config struct {
	Port     int
	CertFile string
	KeyFile  string
	HTTPPort string
}

// parseFlags parses command line flags and returns configuration
func parseFlags() Config {
	// Only define and parse flags if they haven't already been parsed
	// This prevents conflicts with test flags
	if !flag.Parsed() {
		var port = flag.Int("port", 50052, "Port gRPC server")
		flag.Parse()

		return Config{
			Port:     *port,
			CertFile: "certs/cert.pem",
			KeyFile:  "certs/key.pem",
			HTTPPort: ":8082",
		}
	}

	// If flags are already parsed (e.g. during testing), return defaults
	return Config{
		Port:     50052,
		CertFile: "certs/cert.pem",
		KeyFile:  "certs/key.pem",
		HTTPPort: ":8082",
	}
}

// setupLogger configures the zerolog logger
func setupLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

// loadTLSCredentials loads TLS credentials from certificate files
func loadTLSCredentials(certFile, keyFile string) (credentials.TransportCredentials, error) {
	return credentials.NewServerTLSFromFile(certFile, keyFile)
}

// createTCPListener creates a TCP listener on the specified port
func createTCPListener(port int) (net.Listener, error) {
	return net.Listen("tcp", fmt.Sprintf(":%d", port))
}

// setupGRPCServer creates and configures the gRPC server
func setupGRPCServer(creds credentials.TransportCredentials) *grpc.Server {
	server := grpc.NewServer(grpc.Creds(creds))
	service.RegisterServer(server, service.NewConnectionPoolServiceServer())
	return server
}

// setupHTTPMux creates and configures the HTTP mux with health and metrics endpoints
func setupHTTPMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}

// healthHandler handles health check requests
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// createHTTPServer creates the HTTP server for health checks and metrics
func createHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: handler,
	}
}

// startGRPCServer starts the gRPC server in a goroutine
func startGRPCServer(server *grpc.Server, listener net.Listener) chan error {
	errChan := make(chan error, 1)
	go func() {
		log.Info().Msgf("gRPC server listening at %v with TLS", listener.Addr())
		if err := server.Serve(listener); err != nil {
			errChan <- err
		}
	}()
	return errChan
}

// startHTTPServer starts the HTTP server in a goroutine
func startHTTPServer(server *http.Server, certFile, keyFile string) chan error {
	errChan := make(chan error, 1)
	go func() {
		log.Info().Msgf("HTTPS server for health checks and metrics started on %s", server.Addr)
		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()
	return errChan
}

// setupSignalHandler sets up signal handling for graceful shutdown
func setupSignalHandler() chan os.Signal {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	return quit
}

// Application encapsulates the application state and behavior
type Application struct {
	config     Config
	grpcServer *grpc.Server
	httpServer *http.Server
	listener   net.Listener
}

// NewApplication creates a new application instance
func NewApplication() (*Application, error) {
	config := parseFlags()

	// Load TLS credentials
	creds, err := loadTLSCredentials(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
	}

	// Create TCP listener
	listener, err := createTCPListener(config.Port)
	if err != nil {
		return nil, fmt.Errorf("failed to create TCP listener: %w", err)
	}

	// Setup servers
	grpcServer := setupGRPCServer(creds)
	httpMux := setupHTTPMux()
	httpServer := createHTTPServer(config.HTTPPort, httpMux)

	return &Application{
		config:     config,
		grpcServer: grpcServer,
		httpServer: httpServer,
		listener:   listener,
	}, nil
}

// Run starts the application
func (app *Application) Run() error {
	// Start gRPC server
	grpcErrChan := startGRPCServer(app.grpcServer, app.listener)

	// Start HTTP server
	httpErrChan := startHTTPServer(app.httpServer, app.config.CertFile, app.config.KeyFile)

	// Setup signal handling
	signalChan := setupSignalHandler()

	// Wait for shutdown signal or error
	select {
	case err := <-grpcErrChan:
		return fmt.Errorf("gRPC server error: %w", err)
	case err := <-httpErrChan:
		return fmt.Errorf("HTTP server error: %w", err)
	case <-signalChan:
		log.Info().Msg("Shutting down server...")
		app.grpcServer.GracefulStop()
		log.Info().Msg("Server exiting")
		return nil
	}
}

func main() {
	setupLogger()

	app, err := NewApplication()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create application")
	}

	log.Info().Msgf("Starting Connection Pool Manager on port %d", app.config.Port)

	if err := app.Run(); err != nil {
		log.Fatal().Err(err).Msg("Application error")
	}
}
