package connectionpool

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// MockConnectionPool is a mock implementation of a connection pool
type MockConnectionPool struct {
	mock.Mock
	connections map[string]*sql.DB
	mu          sync.Mutex
}

func NewMockConnectionPool() *MockConnectionPool {
	return &MockConnectionPool{
		connections: make(map[string]*sql.DB),
	}
}

func (m *MockConnectionPool) GetConnection(tenantID, dsn string) (string, *sql.DB, error) {
	args := m.Called(tenantID, dsn)

	if args.Error(2) != nil {
		return "", nil, args.Error(2)
	}

	connID := args.String(0)
	conn := args.Get(1).(*sql.DB)

	m.mu.Lock()
	m.connections[connID] = conn
	m.mu.Unlock()

	return connID, conn, nil
}

func (m *MockConnectionPool) ReleaseConnection(connectionID string) (bool, error) {
	args := m.Called(connectionID)

	if args.Error(1) != nil {
		return false, args.Error(1)
	}

	m.mu.Lock()
	delete(m.connections, connectionID)
	m.mu.Unlock()

	return args.Bool(0), nil
}

func (m *MockConnectionPool) GetStats(tenantID string) (int32, int32, int32, error) {
	args := m.Called(tenantID)
	return args.Get(0).(int32), args.Get(1).(int32), args.Get(2).(int32), args.Error(3)
}

// ConnectionPoolServer is our implementation of the ConnectionPoolServiceServer interface
type ConnectionPoolServer struct {
	UnimplementedConnectionPoolServiceServer
	pool IConnectionPool
}

type IConnectionPool interface {
	GetConnection(tenantID, dsn string) (string, *sql.DB, error)
	ReleaseConnection(connectionID string) (bool, error)
	GetStats(tenantID string) (activeConns, idleConns, totalConns int32, err error)
}

func NewConnectionPoolServer(pool IConnectionPool) *ConnectionPoolServer {
	return &ConnectionPoolServer{
		pool: pool,
	}
}

func (s *ConnectionPoolServer) GetConnection(ctx context.Context, req *ConnectionRequest) (*ConnectionResponse, error) {
	if req.TenantId == "" {
		return &ConnectionResponse{Error: "tenant_id is required"}, nil
	}

	if req.Dsn == "" {
		return &ConnectionResponse{Error: "dsn is required"}, nil
	}

	connID, _, err := s.pool.GetConnection(req.TenantId, req.Dsn)
	if err != nil {
		return &ConnectionResponse{Error: err.Error()}, nil
	}

	return &ConnectionResponse{ConnectionId: connID}, nil
}

func (s *ConnectionPoolServer) ReleaseConnection(ctx context.Context, req *ConnectionRelease) (*ReleaseResponse, error) {
	if req.ConnectionId == "" {
		return &ReleaseResponse{Success: false, Error: "connection_id is required"}, nil
	}

	success, err := s.pool.ReleaseConnection(req.ConnectionId)
	if err != nil {
		return &ReleaseResponse{Success: false, Error: err.Error()}, nil
	}

	return &ReleaseResponse{Success: success}, nil
}

func (s *ConnectionPoolServer) GetPoolStats(ctx context.Context, req *StatsRequest) (*StatsResponse, error) {
	if req.TenantId == "" {
		return &StatsResponse{Error: "tenant_id is required"}, nil
	}

	active, idle, total, err := s.pool.GetStats(req.TenantId)
	if err != nil {
		return &StatsResponse{Error: err.Error()}, nil
	}

	return &StatsResponse{
		ActiveConnections: active,
		IdleConnections:   idle,
		TotalConnections:  total,
	}, nil
}

// setupServer creates a test gRPC server with bufconn listener for unit testing
func setupServer(t *testing.T, pool IConnectionPool) (ConnectionPoolServiceClient, func()) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()

	poolServer := NewConnectionPoolServer(pool)
	RegisterConnectionPoolServiceServer(srv, poolServer)

	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("Failed to serve: %v", err)
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithInsecure(),
	)
	require.NoError(t, err)

	client := NewConnectionPoolServiceClient(conn)

	cleanup := func() {
		conn.Close()
		srv.Stop()
	}

	return client, cleanup
}

// TestGetConnection tests the GetConnection RPC
func TestGetConnection(t *testing.T) {
	mockPool := NewMockConnectionPool()
	client, cleanup := setupServer(t, mockPool)
	defer cleanup()

	testCases := []struct {
		name         string
		request      *ConnectionRequest
		mockSetup    func()
		expectedResp *ConnectionResponse
		expectedErr  error
	}{
		{
			name: "successful connection",
			request: &ConnectionRequest{
				TenantId: "tenant1",
				Dsn:      "postgres://user:password@localhost:5432/db",
			},
			mockSetup: func() {
				db, _ := sql.Open("postgres", "postgres://user:password@localhost:5432/db")
				mockPool.On("GetConnection", "tenant1", "postgres://user:password@localhost:5432/db").
					Return("conn-123", db, nil)
			},
			expectedResp: &ConnectionResponse{
				ConnectionId: "conn-123",
				Error:        "",
			},
			expectedErr: nil,
		},
		{
			name: "missing tenant id",
			request: &ConnectionRequest{
				TenantId: "",
				Dsn:      "postgres://user:password@localhost:5432/db",
			},
			mockSetup: func() {},
			expectedResp: &ConnectionResponse{
				ConnectionId: "",
				Error:        "tenant_id is required",
			},
			expectedErr: nil,
		},
		{
			name: "missing dsn",
			request: &ConnectionRequest{
				TenantId: "tenant1",
				Dsn:      "",
			},
			mockSetup: func() {},
			expectedResp: &ConnectionResponse{
				ConnectionId: "",
				Error:        "dsn is required",
			},
			expectedErr: nil,
		},
		{
			name: "connection error",
			request: &ConnectionRequest{
				TenantId: "tenant1",
				Dsn:      "postgres://user:password@localhost:5432/db",
			},
			mockSetup: func() {
				mockPool.On("GetConnection", "tenant1", "postgres://user:password@localhost:5432/db").
					Return("", nil, errors.New("connection error"))
			},
			expectedResp: &ConnectionResponse{
				ConnectionId: "",
				Error:        "connection error",
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockPool.ExpectedCalls = nil
			tc.mockSetup()

			resp, err := client.GetConnection(context.Background(), tc.request)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResp.ConnectionId, resp.ConnectionId)
				assert.Equal(t, tc.expectedResp.Error, resp.Error)
			}

			mockPool.AssertExpectations(t)
		})
	}
}

// TestReleaseConnection tests the ReleaseConnection RPC
func TestReleaseConnection(t *testing.T) {
	mockPool := NewMockConnectionPool()
	client, cleanup := setupServer(t, mockPool)
	defer cleanup()

	testCases := []struct {
		name         string
		request      *ConnectionRelease
		mockSetup    func()
		expectedResp *ReleaseResponse
		expectedErr  error
	}{
		{
			name: "successful release",
			request: &ConnectionRelease{
				ConnectionId: "conn-123",
			},
			mockSetup: func() {
				mockPool.On("ReleaseConnection", "conn-123").Return(true, nil)
			},
			expectedResp: &ReleaseResponse{
				Success: true,
				Error:   "",
			},
			expectedErr: nil,
		},
		{
			name: "missing connection id",
			request: &ConnectionRelease{
				ConnectionId: "",
			},
			mockSetup: func() {},
			expectedResp: &ReleaseResponse{
				Success: false,
				Error:   "connection_id is required",
			},
			expectedErr: nil,
		},
		{
			name: "release error",
			request: &ConnectionRelease{
				ConnectionId: "conn-123",
			},
			mockSetup: func() {
				mockPool.On("ReleaseConnection", "conn-123").
					Return(false, errors.New("release error"))
			},
			expectedResp: &ReleaseResponse{
				Success: false,
				Error:   "release error",
			},
			expectedErr: nil,
		},
		{
			name: "connection not found",
			request: &ConnectionRelease{
				ConnectionId: "non-existent",
			},
			mockSetup: func() {
				mockPool.On("ReleaseConnection", "non-existent").
					Return(false, nil)
			},
			expectedResp: &ReleaseResponse{
				Success: false,
				Error:   "",
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockPool.ExpectedCalls = nil
			tc.mockSetup()

			resp, err := client.ReleaseConnection(context.Background(), tc.request)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResp.Success, resp.Success)
				assert.Equal(t, tc.expectedResp.Error, resp.Error)
			}

			mockPool.AssertExpectations(t)
		})
	}
}

// TestGetPoolStats tests the GetPoolStats RPC
func TestGetPoolStats(t *testing.T) {
	mockPool := NewMockConnectionPool()
	client, cleanup := setupServer(t, mockPool)
	defer cleanup()

	testCases := []struct {
		name         string
		request      *StatsRequest
		mockSetup    func()
		expectedResp *StatsResponse
		expectedErr  error
	}{
		{
			name: "successful stats",
			request: &StatsRequest{
				TenantId: "tenant1",
			},
			mockSetup: func() {
				mockPool.On("GetStats", "tenant1").
					Return(int32(5), int32(10), int32(15), nil)
			},
			expectedResp: &StatsResponse{
				ActiveConnections: 5,
				IdleConnections:   10,
				TotalConnections:  15,
				Error:             "",
			},
			expectedErr: nil,
		},
		{
			name: "missing tenant id",
			request: &StatsRequest{
				TenantId: "",
			},
			mockSetup: func() {},
			expectedResp: &StatsResponse{
				ActiveConnections: 0,
				IdleConnections:   0,
				TotalConnections:  0,
				Error:             "tenant_id is required",
			},
			expectedErr: nil,
		},
		{
			name: "stats error",
			request: &StatsRequest{
				TenantId: "tenant1",
			},
			mockSetup: func() {
				mockPool.On("GetStats", "tenant1").
					Return(int32(0), int32(0), int32(0), errors.New("stats error"))
			},
			expectedResp: &StatsResponse{
				ActiveConnections: 0,
				IdleConnections:   0,
				TotalConnections:  0,
				Error:             "stats error",
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockPool.ExpectedCalls = nil
			tc.mockSetup()

			resp, err := client.GetPoolStats(context.Background(), tc.request)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResp.ActiveConnections, resp.ActiveConnections)
				assert.Equal(t, tc.expectedResp.IdleConnections, resp.IdleConnections)
				assert.Equal(t, tc.expectedResp.TotalConnections, resp.TotalConnections)
				assert.Equal(t, tc.expectedResp.Error, resp.Error)
			}

			mockPool.AssertExpectations(t)
		})
	}
}

// TestConcurrentAccess tests concurrent access to the connection pool
func TestConcurrentAccess(t *testing.T) {
	mockPool := NewMockConnectionPool()
	client, cleanup := setupServer(t, mockPool)
	defer cleanup()

	// Setup mocks
	db, _ := sql.Open("postgres", "postgres://user:password@localhost:5432/db")
	mockPool.On("GetConnection", "tenant1", "postgres://user:password@localhost:5432/db").
		Return("conn-123", db, nil).Times(5)
	mockPool.On("ReleaseConnection", "conn-123").Return(true, nil).Times(5)

	wg := sync.WaitGroup{}
	wg.Add(5)

	// Run 5 concurrent requests
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()

			// Get connection
			resp, err := client.GetConnection(context.Background(), &ConnectionRequest{
				TenantId: "tenant1",
				Dsn:      "postgres://user:password@localhost:5432/db",
			})
			require.NoError(t, err)
			require.Empty(t, resp.Error)

			// Simulate some work
			time.Sleep(10 * time.Millisecond)

			// Release connection
			releaseResp, err := client.ReleaseConnection(context.Background(), &ConnectionRelease{
				ConnectionId: resp.ConnectionId,
			})
			require.NoError(t, err)
			require.True(t, releaseResp.Success)
		}()
	}

	wg.Wait()
	mockPool.AssertExpectations(t)
}

// TestContextCancellation tests context cancellation during requests
func TestContextCancellation(t *testing.T) {
	mockPool := NewMockConnectionPool()
	client, cleanup := setupServer(t, mockPool)
	defer cleanup()

	// Create a context that will be canceled immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Try to make a request with the canceled context
	_, err := client.GetConnection(ctx, &ConnectionRequest{
		TenantId: "tenant1",
		Dsn:      "postgres://user:password@localhost:5432/db",
	})

	// We should get a context canceled error
	assert.Error(t, err)
	assert.Equal(t, codes.Canceled, status.Code(err))
}

// Additional import for the TestContextCancellation test
// import "net"
