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

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	var port = flag.Int("port", 50052, "Port gRPC server")
	flag.Parse()

	log.Info().Msgf("Starting Connection Pool Manager on port %d", *port)

	// Load TLS credentials
	certFile := "certs/cert.pem"
	keyFile := "certs/key.pem"
	creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load TLS credentials")
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to listen")
	}

	server := grpc.NewServer(grpc.Creds(creds))
	service.RegisterServer(server, service.NewConnectionPoolServiceServer())

	go func() {
		log.Info().Msgf("gRPC server listening at %v with TLS", lis.Addr())
		if err := server.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("Failed to start gRPC server")
		}
	}()

	// HTTP server for health and metrics
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		mux.Handle("/metrics", promhttp.Handler())

		httpServer := &http.Server{
			Addr:    ":8082",
			Handler: mux,
		}
		log.Info().Msg("HTTPS server for health checks and metrics started on port 8082")
		if err := httpServer.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	server.GracefulStop()
	log.Info().Msg("Server exiting")
}
