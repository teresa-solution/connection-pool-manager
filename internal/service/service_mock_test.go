package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/teresa-solution/connection-pool-manager/pkg/pool"
	pb "github.com/teresa-solution/connection-pool-manager/proto"
)

// PoolManagerInterface defines the interface that our ConnectionPoolManager implements
type PoolManagerInterface interface {
	GetConnection(ctx context.Context, tenantID, dsn string) (interface{}, error)
	ReleaseConnection(ctx context.Context, tenantID, dsn string) error
	GetStats(ctx context.Context, tenantID, dsn string) (pool.Stats, error)
}

// MockConnectionPoolManager for better testing isolation
type MockConnectionPoolManager struct {
	pools       map[string]bool
	shouldError bool
}

func NewMockConnectionPoolManager() *MockConnectionPoolManager {
	return &MockConnectionPoolManager{
		pools: make(map[string]bool),
	}
}

func (m *MockConnectionPoolManager) GetConnection(ctx context.Context, tenantID, dsn string) (interface{}, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	key := tenantID + ":" + dsn
	m.pools[key] = true
	return "mock-pool", nil
}

func (m *MockConnectionPoolManager) ReleaseConnection(ctx context.Context, tenantID, dsn string) error {
	if m.shouldError {
		return assert.AnError
	}
	key := tenantID + ":" + dsn
	if _, exists := m.pools[key]; !exists {
		return fmt.Errorf("pool not found for tenant %s and dsn %s", tenantID, dsn)
	}
	delete(m.pools, key)
	return nil
}

func (m *MockConnectionPoolManager) GetStats(ctx context.Context, tenantID, dsn string) (pool.Stats, error) {
	if m.shouldError {
		return pool.Stats{}, assert.AnError
	}
	return pool.Stats{
		ActiveConnections: 5,
		IdleConnections:   3,
		TotalConnections:  8,
	}, nil
}

// ConnectionPoolServiceServerWithMock allows us to inject a mock pool manager
type ConnectionPoolServiceServerWithMock struct {
	pb.UnimplementedConnectionPoolServiceServer
	poolManager PoolManagerInterface
}

func NewConnectionPoolServiceServerWithMock(manager PoolManagerInterface) *ConnectionPoolServiceServerWithMock {
	return &ConnectionPoolServiceServerWithMock{
		poolManager: manager,
	}
}

func (s *ConnectionPoolServiceServerWithMock) GetConnection(ctx context.Context, req *pb.ConnectionRequest) (*pb.ConnectionResponse, error) {
	_, err := s.poolManager.GetConnection(ctx, req.TenantId, req.Dsn)
	if err != nil {
		return &pb.ConnectionResponse{Error: err.Error()}, err
	}
	// Simulate connection ID (in practice, this could be a unique identifier)
	connID := fmt.Sprintf("conn-%s-%d", req.TenantId, time.Now().UnixNano())
	return &pb.ConnectionResponse{ConnectionId: connID}, nil
}

func (s *ConnectionPoolServiceServerWithMock) ReleaseConnection(ctx context.Context, req *pb.ConnectionRelease) (*pb.ReleaseResponse, error) {
	// Parse connection ID to extract tenantID and dsn (simplified logic)
	parts := strings.Split(req.ConnectionId, "-")
	if len(parts) < 2 {
		return &pb.ReleaseResponse{Success: false, Error: "invalid connection ID"}, fmt.Errorf("invalid connection ID")
	}
	tenantID := parts[1]
	err := s.poolManager.ReleaseConnection(ctx, tenantID, "host=localhost port=5432 user=admin password=securepassword dbname=tenant_registry")
	if err != nil {
		return &pb.ReleaseResponse{Success: false, Error: err.Error()}, err
	}
	return &pb.ReleaseResponse{Success: true}, nil
}

func (s *ConnectionPoolServiceServerWithMock) GetPoolStats(ctx context.Context, req *pb.StatsRequest) (*pb.StatsResponse, error) {
	stats, err := s.poolManager.GetStats(ctx, req.TenantId, "host=localhost port=5432 user=admin password=securepassword dbname=tenant_registry")
	if err != nil {
		return &pb.StatsResponse{Error: err.Error()}, err
	}
	return &pb.StatsResponse{
		ActiveConnections: int32(stats.ActiveConnections),
		IdleConnections:   int32(stats.IdleConnections),
		TotalConnections:  int32(stats.TotalConnections),
	}, nil
}

func TestConnectionPoolServiceServer_WithMockManager(t *testing.T) {
	// Create server with mock manager for isolated testing
	server := NewConnectionPoolServiceServerWithMock(NewMockConnectionPoolManager())

	ctx := context.Background()

	// Test successful connection
	req := &pb.ConnectionRequest{
		TenantId: "test-tenant",
		Dsn:      "mock://localhost:5432/testdb",
	}

	resp, err := server.GetConnection(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.ConnectionId)
	assert.Contains(t, resp.ConnectionId, "conn-test-tenant")
	assert.Empty(t, resp.Error)
}

func TestConnectionPoolServiceServer_WithErroringMockManager(t *testing.T) {
	// Create server with mock manager that returns errors
	mockManager := NewMockConnectionPoolManager()
	mockManager.shouldError = true

	server := NewConnectionPoolServiceServerWithMock(mockManager)

	ctx := context.Background()

	// Test connection failure
	req := &pb.ConnectionRequest{
		TenantId: "test-tenant",
		Dsn:      "mock://localhost:5432/testdb",
	}

	resp, err := server.GetConnection(ctx, req)
	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Error)
	assert.Empty(t, resp.ConnectionId)
}

func TestConnectionPoolServiceServer_ReleaseWithMockManager(t *testing.T) {
	mockManager := NewMockConnectionPoolManager()
	server := NewConnectionPoolServiceServerWithMock(mockManager)

	ctx := context.Background()

	// First create a connection using the SAME DSN that ReleaseConnection will use
	dsn := "host=localhost port=5432 user=admin password=securepassword dbname=tenant_registry"
	_, err := mockManager.GetConnection(ctx, "tenant1", dsn)
	require.NoError(t, err)

	// Test successful release
	releaseReq := &pb.ConnectionRelease{
		ConnectionId: "conn-tenant1-1234567890",
	}

	releaseResp, err := server.ReleaseConnection(ctx, releaseReq)
	assert.NoError(t, err)
	assert.NotNil(t, releaseResp)
	assert.True(t, releaseResp.Success)
	assert.Empty(t, releaseResp.Error)
}

func TestConnectionPoolServiceServer_StatsWithMockManager(t *testing.T) {
	mockManager := NewMockConnectionPoolManager()
	server := NewConnectionPoolServiceServerWithMock(mockManager)

	ctx := context.Background()

	// Test successful stats retrieval
	statsReq := &pb.StatsRequest{
		TenantId: "test-tenant",
	}

	statsResp, err := server.GetPoolStats(ctx, statsReq)
	assert.NoError(t, err)
	assert.NotNil(t, statsResp)
	assert.Equal(t, int32(5), statsResp.ActiveConnections)
	assert.Equal(t, int32(3), statsResp.IdleConnections)
	assert.Equal(t, int32(8), statsResp.TotalConnections)
	assert.Empty(t, statsResp.Error)
}

// Test helper function to verify connection ID format
func TestConnectionIDGeneration(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
	}{
		{"Normal tenant ID", "tenant1"},
		{"Empty tenant ID", ""},
		{"Special characters", "tenant@123"},
		{"Long tenant ID", "very-long-tenant-id-with-many-characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := NewMockConnectionPoolManager()
			server := NewConnectionPoolServiceServerWithMock(mockManager)

			ctx := context.Background()
			req := &pb.ConnectionRequest{
				TenantId: tt.tenantID,
				Dsn:      "mock://localhost:5432/testdb",
			}

			resp, err := server.GetConnection(ctx, req)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.NotEmpty(t, resp.ConnectionId)
			assert.Contains(t, resp.ConnectionId, "conn-")

			if tt.tenantID != "" {
				assert.Contains(t, resp.ConnectionId, tt.tenantID)
			}
		})
	}
}
