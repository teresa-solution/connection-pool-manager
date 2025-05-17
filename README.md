# Connection Pool Manager

A secure, multi-tenant connection pool management service built in Go that provides connection pooling capabilities for PostgreSQL databases over gRPC.

## Overview

Connection Pool Manager is a service that manages database connection pools across multiple tenants. It provides a secure gRPC API for requesting and releasing database connections and retrieving pool statistics.

Key features:
- Multi-tenant connection pooling
- Secure gRPC API with TLS
- HTTP health checks and Prometheus metrics
- Graceful shutdown handling
- Connection configuration with sensible defaults

## Architecture

The service consists of the following components:

- **gRPC Server**: Handles client requests for connection management
- **HTTP Server**: Provides health check endpoint and Prometheus metrics
- **Connection Pool Manager**: Core logic for creating and managing connection pools
- **PGX Pool Integration**: Uses the Postgres pgx/pgxpool library for robust connection pooling

## Prerequisites

- Go 1.20 or higher
- PostgreSQL database
- TLS certificates for secure communication

## Installation

```bash
# Clone the repository
git clone https://github.com/teresa-solution/connection-pool-manager.git
cd connection-pool-manager

# Install dependencies
go mod download

# Build the application
go build -o connection-pool-manager ./main.go
```

## Configuration

The application can be configured using command-line flags:

| Flag      | Description                 | Default Value |
|-----------|-----------------------------| ------------- |
| `--port`  | Port for the gRPC server    | 50052         |

Additional configuration parameters (not exposed as flags):

| Parameter   | Description                    | Default Value      |
|-------------|--------------------------------|--------------------|
| CertFile    | Path to TLS certificate        | certs/cert.pem     |
| KeyFile     | Path to TLS key                | certs/key.pem      |
| HTTPPort    | Port for HTTP metrics/health   | :8082              |

### Connection Pool Settings

Each connection pool is configured with the following defaults:

- Max Connections: 20
- Min Connections: 5
- Max Connection Lifetime: 30 minutes
- Max Connection Idle Time: 5 minutes

## Usage

### Starting the server

```bash
# Run with default configuration
./connection-pool-manager

# Run with custom port
./connection-pool-manager --port 50053
```

### API Endpoints

#### gRPC Endpoints

The service implements the following gRPC methods:

- `GetConnection`: Request a connection for a tenant
- `ReleaseConnection`: Release a connection back to the pool
- `GetPoolStats`: Get statistics about a tenant's connection pool

#### HTTP Endpoints

- `/health`: Health check endpoint that returns HTTP 200 if the service is running
- `/metrics`: Prometheus metrics endpoint

### Example Client Usage

```go
package main

import (
	"context"
	"log"
	"time"

	pb "github.com/teresa-solution/connection-pool-manager/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	// Set up a connection to the server with TLS
	creds, err := credentials.NewClientTLSFromFile("certs/cert.pem", "")
	if err != nil {
		log.Fatalf("Failed to load credentials: %v", err)
	}
	
	conn, err := grpc.Dial("localhost:50052", grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	client := pb.NewConnectionPoolServiceClient(conn)
	
	// Get a connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	
	resp, err := client.GetConnection(ctx, &pb.ConnectionRequest{
		TenantId: "tenant123",
		Dsn:      "host=localhost port=5432 user=admin password=securepassword dbname=tenant_registry",
	})
	
	if err != nil {
		log.Fatalf("Could not get connection: %v", err)
	}
	
	log.Printf("Connection ID: %s", resp.ConnectionId)
	
	// Use the connection...
	
	// Release the connection
	releaseResp, err := client.ReleaseConnection(ctx, &pb.ConnectionRelease{
		ConnectionId: resp.ConnectionId,
	})
	
	if err != nil {
		log.Fatalf("Could not release connection: %v", err)
	}
	
	log.Printf("Connection released successfully: %v", releaseResp.Success)
}
```

## TLS Configuration

The service requires TLS certificates for secure communication. Generate self-signed certificates for development:

```bash
mkdir -p certs
openssl req -x509 -newkey rsa:4096 -keyout certs/key.pem -out certs/cert.pem -days 365 -nodes
```

For production, use certificates from a trusted certificate authority.

## Development

### Directory Structure

```
connection-pool-manager/
├── main.go                 # Application entry point
├── internal/
│   └── service/            # gRPC service implementation
├── pkg/
│   └── pool/               # Connection pool management logic
├── proto/                  # Protocol buffer definitions
└── certs/                  # TLS certificates
```

### Running Tests

```bash
go test ./...
```

## Monitoring

The service exposes Prometheus metrics at the `/metrics` endpoint. Key metrics include:

- Active connections per tenant
- Idle connections per tenant
- Total connections per tenant
- Request latency
- Error rates

## License

[MIT License](LICENSE)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.