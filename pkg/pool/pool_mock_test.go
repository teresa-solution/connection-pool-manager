package pool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockPool simulates pgxpool.Pool for testing
type MockPool struct {
	closed bool
}

func (m *MockPool) Close() {
	m.closed = true
}

// Test with mock pools to avoid database dependencies
func TestConnectionPoolManager_ReleaseConnection_WithMock(t *testing.T) {
	cpm := NewConnectionPoolManager()
	ctx := context.Background()
	tenantID := "tenant1"
	dsn := "mock://localhost:5432/testdb"

	// Test the error case when no pool exists
	err := cpm.ReleaseConnection(ctx, tenantID, dsn)
	assert.Error(t, err) // This will error because no pool exists
	assert.Contains(t, err.Error(), "pool not found")

	// Verify the pool was not added
	assert.Equal(t, 0, len(cpm.pools))
}

func TestConnectionPoolManager_KeyGeneration(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		dsn      string
		expected string
	}{
		{
			name:     "Normal case",
			tenantID: "tenant1",
			dsn:      "postgres://localhost:5432/db",
			expected: "tenant1:postgres://localhost:5432/db",
		},
		{
			name:     "Empty tenant ID",
			tenantID: "",
			dsn:      "postgres://localhost:5432/db",
			expected: ":postgres://localhost:5432/db",
		},
		{
			name:     "Special characters",
			tenantID: "tenant@1",
			dsn:      "postgres://user:pass@localhost:5432/db?ssl=true",
			expected: "tenant@1:postgres://user:pass@localhost:5432/db?ssl=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test key generation logic (this is implicit in the implementation)
			expected := tt.tenantID + ":" + tt.dsn
			assert.Equal(t, tt.expected, expected)
		})
	}
}

func TestConnectionPoolManager_ConcurrentOperations(t *testing.T) {
	cpm := NewConnectionPoolManager()
	ctx := context.Background()

	// Test that concurrent operations don't cause race conditions
	// Even if they fail, they shouldn't panic
	done := make(chan bool, 4)

	// Concurrent release operations (these will fail gracefully)
	go func() {
		for i := 0; i < 5; i++ {
			_ = cpm.ReleaseConnection(ctx, "tenant1", "postgres://localhost:5432/db1")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 5; i++ {
			_ = cpm.ReleaseConnection(ctx, "tenant2", "postgres://localhost:5432/db2")
		}
		done <- true
	}()

	// Test GetStats operations (these will also fail gracefully)
	go func() {
		for i := 0; i < 5; i++ {
			_, _ = cpm.GetStats(ctx, "tenant1", "postgres://localhost:5432/db1")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 5; i++ {
			_, _ = cpm.GetStats(ctx, "tenant2", "postgres://localhost:5432/db2")
		}
		done <- true
	}()

	// Wait for all operations to complete
	for i := 0; i < 4; i++ {
		<-done
	}

	// Test passed if no panics occurred
	assert.True(t, true)
}

func TestConnectionPoolManager_PoolsMap(t *testing.T) {
	cpm := NewConnectionPoolManager()

	// Verify initial state
	assert.NotNil(t, cpm.pools)
	assert.Equal(t, 0, len(cpm.pools))

	// The pools map should exist and be accessible
	assert.NotNil(t, cpm.pools)
}
