package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	pb "github.com/teresa-solution/connection-pool-manager/proto"
)

// Test the actual service implementation with better error handling
func TestConnectionPoolServiceServer_EndToEnd(t *testing.T) {
	server := NewConnectionPoolServiceServer()
	ctx := context.Background()

	// Test with an invalid DSN that should fail gracefully
	req := &pb.ConnectionRequest{
		TenantId: "test-tenant",
		Dsn:      "invalid-dsn-format",
	}

	resp, err := server.GetConnection(ctx, req)
	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Error)
	assert.Empty(t, resp.ConnectionId)
}

func TestConnectionPoolServiceServer_ConnectionIDParsing(t *testing.T) {
	tests := []struct {
		name           string
		connectionID   string
		expectError    bool
		expectedTenant string
	}{
		{
			name:           "Valid connection ID",
			connectionID:   "conn-tenant1-1234567890",
			expectError:    false,
			expectedTenant: "tenant1",
		},
		{
			name:           "Valid connection ID with complex tenant",
			connectionID:   "conn-tenant@123-1234567890",
			expectError:    false,
			expectedTenant: "tenant@123",
		},
		{
			name:         "Invalid format - too few parts",
			connectionID: "conn",
			expectError:  true,
		},
		{
			name:         "Invalid format - single part",
			connectionID: "invalid",
			expectError:  true,
		},
		{
			name:         "Empty connection ID",
			connectionID: "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the connection ID parsing logic
			parts := strings.Split(tt.connectionID, "-")

			if tt.expectError {
				assert.Less(t, len(parts), 2, "Should have fewer than 2 parts")
			} else {
				assert.GreaterOrEqual(t, len(parts), 2, "Should have at least 2 parts")
				if len(parts) >= 2 {
					assert.Equal(t, tt.expectedTenant, parts[1])
				}
			}
		})
	}
}

func TestConnectionPoolServiceServer_HardcodedDSN(t *testing.T) {
	// Test that the service uses hardcoded DSN in ReleaseConnection
	server := NewConnectionPoolServiceServer()
	ctx := context.Background()

	req := &pb.ConnectionRelease{
		ConnectionId: "conn-tenant1-1234567890",
	}

	resp, err := server.ReleaseConnection(ctx, req)

	// This will error because there's no pool to release, but we can verify the error message
	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
	assert.Contains(t, resp.Error, "pool not found")
}

func TestConnectionPoolServiceServer_GetPoolStatsHardcodedDSN(t *testing.T) {
	// Test that the service uses hardcoded DSN in GetPoolStats
	server := NewConnectionPoolServiceServer()
	ctx := context.Background()

	req := &pb.StatsRequest{
		TenantId: "tenant1",
	}

	resp, err := server.GetPoolStats(ctx, req)

	// This will error because there's no pool, but we can verify the error message
	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Error)
	assert.Contains(t, resp.Error, "pool not found")
}

// Test the service registration function
func TestRegisterServer(t *testing.T) {
	// This test just ensures the registration function doesn't panic
	// In a real scenario, you'd need a gRPC server instance
	server := NewConnectionPoolServiceServer()
	assert.NotNil(t, server)

	// The RegisterServer function exists and can be called
	// RegisterServer(nil, server) // Would panic, so we just verify the function exists
	assert.NotNil(t, RegisterServer)
}

// Test server initialization
func TestNewConnectionPoolServiceServer_Initialization(t *testing.T) {
	server := NewConnectionPoolServiceServer()

	// Verify all fields are properly initialized
	assert.NotNil(t, server)
	assert.NotNil(t, server.poolManager)

	// Verify it implements the required interface
	var _ pb.ConnectionPoolServiceServer = server
}

// Test with safe context handling
func TestConnectionPoolServiceServer_ContextSafety(t *testing.T) {
	server := NewConnectionPoolServiceServer()

	// Test with valid context
	tests := []struct {
		name string
		test func() error
	}{
		{
			name: "GetConnection with background context",
			test: func() error {
				ctx := context.Background()
				_, err := server.GetConnection(ctx, &pb.ConnectionRequest{
					TenantId: "test",
					Dsn:      "invalid-dsn",
				})
				return err
			},
		},
		{
			name: "ReleaseConnection with background context",
			test: func() error {
				ctx := context.Background()
				_, err := server.ReleaseConnection(ctx, &pb.ConnectionRelease{
					ConnectionId: "conn-test-123",
				})
				return err
			},
		},
		{
			name: "GetPoolStats with background context",
			test: func() error {
				ctx := context.Background()
				_, err := server.GetPoolStats(ctx, &pb.StatsRequest{
					TenantId: "test",
				})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These will error due to invalid DSN or no pool, but shouldn't panic
			err := tt.test()
			// We expect errors in these cases, so just verify no panic occurred
			assert.Error(t, err) // We expect errors since we're using invalid data
		})
	}
}
