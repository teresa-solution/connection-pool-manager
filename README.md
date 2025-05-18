# 🌊 Teresa Connection Pool Manager

[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=for-the-badge&logo=go)](https://golang.org/doc/go1.20)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge)](LICENSE)
[![gRPC](https://img.shields.io/badge/gRPC-supported-brightgreen?style=for-the-badge&logo=google)](https://grpc.io/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-supported-blue?style=for-the-badge&logo=postgresql&logoColor=white)](https://www.postgresql.org/)

A high-performance, multi-tenant connection pool manager for PostgreSQL databases with gRPC interface. Perfect for applications that need to manage database connections across multiple tenants efficiently.

## ✨ Features

- **🏢 Multi-tenant support**: Manage database connections for multiple tenants efficiently
- **🔄 Connection pooling**: Utilize pgx's connection pooling for optimized database access
- **🔒 TLS encryption**: Secure communication with TLS by default
- **🔍 Connection metrics**: Track active, idle, and total connections per pool
- **🏥 Health checks**: Easily monitor service health via HTTP endpoint
- **📊 Prometheus integration**: Built-in metrics exposed for Prometheus scraping

## 🏗️ Architecture

```
          ┌─────────────────────┐
          │                     │
 gRPC     │  Connection Pool    │
 Clients  │     Manager         │
   ───────►                     │    ┌─────────┐
          │  ┌────────────────┐ │    │         │
          │  │Pool Manager    │ │    │         │
          │  │                ├─┼────► Postgres│
          │  │tenant1:pool1   │ │    │Databases│
          │  │tenant2:pool1   │ │    │         │
          │  │tenant3:pool1   │ │    │         │
          │  └────────────────┘ │    └─────────┘
          │                     │
          │  ┌────────────────┐ │    ┌─────────┐
HTTP      │  │Health & Metrics│ │    │         │
Clients   │  │                ├─┼────►Prometheus│
   ───────►  │/health         │ │    │         │
          │  │/metrics        │ │    │         │
          │  └────────────────┘ │    └─────────┘
          │                     │
          └─────────────────────┘
```

## 🚀 Getting Started

### Prerequisites

- Go 1.20+
- PostgreSQL database
- TLS certificates (for secure communication)

### Installation

```bash
# Clone the repository
git clone https://github.com/teresa-solution/connection-pool-manager.git
cd connection-pool-manager

# Build the application
go build -o conn-pool-manager ./cmd/server

# Run with default settings
./conn-pool-manager
```

### Configuration

The service can be configured with command-line flags:

```bash
./conn-pool-manager --port=50052
```

Default configuration:
- gRPC server port: `50052`
- HTTPS metrics/health port: `8082`
- TLS certificates path: `certs/cert.pem` and `certs/key.pem`

## 🔧 Usage

### Using the gRPC API

The service exposes a gRPC API for connection pool management:

```go
// Get a connection from the pool
connection, err := client.GetConnection(ctx, &pb.ConnectionRequest{
    TenantId: "tenant123",
    Dsn: "host=localhost port=5432 user=admin password=securepassword dbname=tenant_registry",
})

// Get pool statistics
stats, err := client.GetPoolStats(ctx, &pb.StatsRequest{
    TenantId: "tenant123",
})

// Release a connection back to the pool
result, err := client.ReleaseConnection(ctx, &pb.ConnectionRelease{
    ConnectionId: connection.ConnectionId,
})
```

### Health Check

The service exposes an HTTP health check endpoint:

```bash
curl -k https://localhost:8082/health
```

### Metrics

Prometheus metrics are available at:

```bash
curl -k https://localhost:8082/metrics
```

## 💡 Key Components

1. **Pool Manager**: Handles connection pooling logic for multiple tenants
2. **gRPC Service**: Exposes connection management capabilities via gRPC
3. **Metrics Server**: Provides health checks and Prometheus metrics
4. **TLS Security**: Ensures secure communication with clients

## 📊 Pool Configuration

Each connection pool is configured with the following default parameters:

- Max connections: `20`
- Min connections: `5`
- Max connection lifetime: `30 minutes`
- Max idle time: `5 minutes`

## 🔐 Security

The service uses TLS for both gRPC and HTTP servers. Make sure to:

1. Generate proper TLS certificates
2. Store them in the `certs/` directory as `cert.pem` and `key.pem`
3. Distribute the public certificate to clients

## 📦 Project Structure

```
connection-pool-manager/
├── cmd/
│   └── server/           # Main application entry point
├── internal/
│   └── service/          # gRPC service implementation
├── pkg/
│   └── pool/             # Connection pool management logic
├── proto/                # Protocol Buffers definitions
├── certs/                # TLS certificates
└── README.md             # This file
```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.
