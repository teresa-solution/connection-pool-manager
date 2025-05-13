package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

func main() {
	// Konfigurasi logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Parse flag command line
	var (
		port = flag.Int("port", 50052, "Port gRPC server")
	)
	flag.Parse()

	log.Info().Msgf("Starting Connection Pool Manager on port %d", *port)

	// Setup gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to listen")
	}

	server := grpc.NewServer()

	// Daftarkan service gRPC (akan diimplementasikan nanti)
	// pb.RegisterConnectionPoolServiceServer(server, &service.ConnectionPoolServiceServer{})

	// Start server in goroutine
	go func() {
		log.Info().Msgf("gRPC server listening at %v", lis.Addr())
		if err := server.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("Failed to start gRPC server")
		}
	}()

	// Setup HTTP server for health checks and metrics
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		httpServer := &http.Server{
			Addr:    ":8082",
			Handler: mux,
		}

		log.Info().Msg("HTTP server for health checks started on port 8082")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP server error")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	server.GracefulStop()
	log.Info().Msg("Server exiting")
}
