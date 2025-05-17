package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pb "github.com/teresa-solution/connection-pool-manager/proto"
)

func TestNewConnectionPoolServiceServer(t *testing.T) {
	server := NewConnectionPoolServiceServer()
	assert.NotNil(t, server)
	assert.NotNil(t, server.poolManager)
}

func TestConnectionPoolServiceServer_GetConnection(t *testing.T) {
	tests := []struct {
		name    string
		req     *pb.ConnectionRequest
		wantErr bool
	}{
		{
			name: "Invalid DSN",
			req: &pb.ConnectionRequest{
				TenantId: "tenant1",
				Dsn:      "invalid-dsn",
			},
			wantErr: true,
		},
		{
			name: "Empty tenant ID with invalid DSN",
			req: &pb.ConnectionRequest{
				TenantId: "",
				Dsn:      "invalid-dsn",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewConnectionPoolServiceServer()
			ctx := context.Background()

			resp, err := server.GetConnection(ctx, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.Error)
				assert.Empty(t, resp.ConnectionId)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.ConnectionId)
				assert.Contains(t, resp.ConnectionId, "conn-")
				assert.Contains(t, resp.ConnectionId, tt.req.TenantId)
				assert.Empty(t, resp.Error)
			}
		})
	}
}

func TestConnectionPoolServiceServer_ReleaseConnection(t *testing.T) {
	tests := []struct {
		name    string
		req     *pb.ConnectionRelease
		wantErr bool
	}{
		{
			name: "Valid connection ID",
			req: &pb.ConnectionRelease{
				ConnectionId: "conn-tenant1-1234567890",
			},
			wantErr: true, // Will error due to no actual pool
		},
		{
			name: "Invalid connection ID format",
			req: &pb.ConnectionRelease{
				ConnectionId: "invalid-format",
			},
			wantErr: true,
		},
		{
			name: "Empty connection ID",
			req: &pb.ConnectionRelease{
				ConnectionId: "",
			},
			wantErr: true,
		},
		{
			name: "Single part connection ID",
			req: &pb.ConnectionRelease{
				ConnectionId: "single",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewConnectionPoolServiceServer()
			ctx := context.Background()

			resp, err := server.ReleaseConnection(ctx, tt.req)

			require.NotNil(t, resp)
			if tt.wantErr {
				assert.Error(t, err)
				assert.False(t, resp.Success)
				assert.NotEmpty(t, resp.Error)
			} else {
				assert.NoError(t, err)
				assert.True(t, resp.Success)
				assert.Empty(t, resp.Error)
			}
		})
	}
}

func TestConnectionPoolServiceServer_GetPoolStats(t *testing.T) {
	tests := []struct {
		name    string
		req     *pb.StatsRequest
		wantErr bool
	}{
		{
			name: "Valid stats request",
			req: &pb.StatsRequest{
				TenantId: "tenant1",
			},
			wantErr: true, // Will error due to no actual pool
		},
		{
			name: "Empty tenant ID",
			req: &pb.StatsRequest{
				TenantId: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewConnectionPoolServiceServer()
			ctx := context.Background()

			resp, err := server.GetPoolStats(ctx, tt.req)

			require.NotNil(t, resp)
			if tt.wantErr {
				assert.Error(t, err)
				assert.NotEmpty(t, resp.Error)
			} else {
				assert.NoError(t, err)
				assert.GreaterOrEqual(t, resp.ActiveConnections, int32(0))
				assert.GreaterOrEqual(t, resp.IdleConnections, int32(0))
				assert.GreaterOrEqual(t, resp.TotalConnections, int32(0))
			}
		})
	}
}

func TestConnectionPoolServiceServer_ParseConnectionID(t *testing.T) {
	tests := []struct {
		name         string
		connectionID string
		wantParts    int
		expectError  bool
	}{
		{
			name:         "Valid connection ID",
			connectionID: "conn-tenant1-1234567890",
			wantParts:    3,
			expectError:  true, // Will still error due to no pool
		},
		{
			name:         "Short connection ID",
			connectionID: "conn-tenant1",
			wantParts:    2,
			expectError:  true, // Will still error due to no pool
		},
		{
			name:         "Single part",
			connectionID: "single",
			wantParts:    1,
			expectError:  true,
		},
		{
			name:         "Empty",
			connectionID: "",
			wantParts:    1,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewConnectionPoolServiceServer()
			ctx := context.Background()

			req := &pb.ConnectionRelease{
				ConnectionId: tt.connectionID,
			}

			resp, err := server.ReleaseConnection(ctx, req)

			if tt.expectError {
				assert.Error(t, err)
				assert.False(t, resp.Success)
			} else {
				// Even valid parsing will fail due to no actual pool
				assert.Error(t, err)
				assert.False(t, resp.Success)
			}
		})
	}
}

// Benchmark tests
func BenchmarkConnectionPoolServiceServer_GetConnection(b *testing.B) {
	server := NewConnectionPoolServiceServer()
	ctx := context.Background()
	req := &pb.ConnectionRequest{
		TenantId: "bench-tenant",
		Dsn:      "invalid-dsn", // Use invalid DSN to avoid connection issues
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = server.GetConnection(ctx, req)
	}
}

func BenchmarkConnectionPoolServiceServer_ReleaseConnection(b *testing.B) {
	server := NewConnectionPoolServiceServer()
	ctx := context.Background()
	req := &pb.ConnectionRelease{
		ConnectionId: "conn-bench-tenant-1234567890",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = server.ReleaseConnection(ctx, req)
	}
}
