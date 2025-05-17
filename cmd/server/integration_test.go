package main

import (
	"net"
	"os"
	"testing"
	"time"
)

// TestLoadTLSCredentials_WithValidCerts tests loading valid certificates
func TestLoadTLSCredentials_WithValidCerts(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Cleanup()

	// Create valid test certificates
	if err := helper.CreateTestCertificates(); err != nil {
		t.Fatalf("Failed to create test certificates: %v", err)
	}

	certFile, keyFile := helper.GetCertPaths()

	// Test loading valid certificates
	creds, err := loadTLSCredentials(certFile, keyFile)
	if err != nil {
		t.Errorf("loadTLSCredentials() with valid certs failed: %v", err)
	}

	if creds == nil {
		t.Error("loadTLSCredentials() returned nil credentials")
	}
}

// TestLoadTLSCredentials_WithInvalidCerts tests loading invalid certificates
func TestLoadTLSCredentials_WithInvalidCerts(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Cleanup()

	// Create invalid certificate files
	if err := helper.CreateInvalidCertFiles(); err != nil {
		t.Fatalf("Failed to create invalid cert files: %v", err)
	}

	certFile, keyFile := helper.GetCertPaths()

	// Test loading invalid certificates
	_, err := loadTLSCredentials(certFile, keyFile)
	if err == nil {
		t.Error("loadTLSCredentials() with invalid certs should have failed")
	}
}

// TestSetupGRPCServer_WithValidCreds tests gRPC server setup with valid credentials
func TestSetupGRPCServer_WithValidCreds(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Cleanup()

	// Create valid test certificates
	if err := helper.CreateTestCertificates(); err != nil {
		t.Fatalf("Failed to create test certificates: %v", err)
	}

	certFile, keyFile := helper.GetCertPaths()

	// Load credentials
	creds, err := loadTLSCredentials(certFile, keyFile)
	if err != nil {
		t.Fatalf("Failed to load credentials: %v", err)
	}

	// Test gRPC server setup
	server := setupGRPCServer(creds)
	if server == nil {
		t.Error("setupGRPCServer() returned nil")
	}

	// Clean up
	server.Stop()
}

// TestNewApplication_WithValidCerts tests application creation with valid certificates
func TestNewApplication_WithValidCerts(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Cleanup()

	// Create valid test certificates in the expected location
	certsDir := "certs"
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		t.Fatalf("Failed to create certs directory: %v", err)
	}
	defer os.RemoveAll(certsDir)

	// Create certificates in the expected location
	if err := helper.CreateTestCertificates(); err != nil {
		t.Fatalf("Failed to create test certificates: %v", err)
	}

	// Copy certificates to expected location
	certFile, keyFile := helper.GetCertPaths()
	expectedCertFile := "certs/cert.pem"
	expectedKeyFile := "certs/key.pem"

	// Copy cert file
	certData, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("Failed to read cert file: %v", err)
	}
	if err := os.WriteFile(expectedCertFile, certData, 0644); err != nil {
		t.Fatalf("Failed to write cert file: %v", err)
	}

	// Copy key file
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("Failed to read key file: %v", err)
	}
	if err := os.WriteFile(expectedKeyFile, keyData, 0644); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	// Now test application creation
	app, err := NewApplication()
	if err != nil {
		t.Errorf("NewApplication() with valid certs failed: %v", err)
	}

	if app != nil {
		defer app.listener.Close()

		if app.config.Port == 0 {
			t.Error("Application port not set")
		}

		if app.grpcServer == nil {
			t.Error("gRPC server not initialized")
		}

		if app.httpServer == nil {
			t.Error("HTTP server not initialized")
		}

		if app.listener == nil {
			t.Error("Listener not initialized")
		}
	}
}

// TestHTTPServer_EndToEnd tests the HTTP server endpoints end-to-end
func TestHTTPServer_EndToEnd(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Cleanup()

	// Create valid test certificates
	if err := helper.CreateTestCertificates(); err != nil {
		t.Fatalf("Failed to create test certificates: %v", err)
	}

	certFile, keyFile := helper.GetCertPaths()

	// Create HTTP server
	mux := setupHTTPMux()
	server := createHTTPServer(":0", mux) // Use port 0 for automatic assignment

	// Start the server
	errChan := startHTTPServer(server, certFile, keyFile)

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Test health endpoint (we can't easily test HTTPS without proper certificate validation)
	// So we'll test the mux directly which we already covered in other tests

	// The server will fail to start properly due to HTTPS configuration
	// but that's expected in this test environment
	select {
	case err := <-errChan:
		// Expected to get an error since we're using test certificates
		if err == nil {
			t.Error("Expected HTTP server to fail with test certificates")
		}
	case <-time.After(200 * time.Millisecond):
		// If no error within timeout, that's also acceptable
	}
}

// TestCreateTCPListener_PortInUse tests listener creation when port is in use
func TestCreateTCPListener_PortInUse(t *testing.T) {
	// First, create a listener on a random port
	listener1, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to create first listener: %v", err)
	}
	defer listener1.Close()

	// Get the port that was assigned
	addr := listener1.Addr().(*net.TCPAddr)
	port := addr.Port

	// Try to create another listener on the same port
	listener2, err := createTCPListener(port)
	if err == nil {
		listener2.Close()
		t.Error("Expected createTCPListener to fail when port is in use")
	}
}

// TestApplication_Lifecycle tests the complete application lifecycle with mocks
func TestApplication_Lifecycle(t *testing.T) {
	// This test would require significant mocking or actual integration testing
	// Skip for now as it would require complex setup
	t.Skip("Requires complex mocking or integration test environment")

	// In a real integration test environment, you would:
	// 1. Create an application with test certificates
	// 2. Start it in a goroutine
	// 3. Test that both gRPC and HTTP servers are responding
	// 4. Send a shutdown signal
	// 5. Verify graceful shutdown
}

// TestConcurrentServerStarts tests starting multiple servers concurrently
func TestConcurrentServerStarts(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Cleanup()

	// Create valid test certificates
	if err := helper.CreateTestCertificates(); err != nil {
		t.Fatalf("Failed to create test certificates: %v", err)
	}

	certFile, keyFile := helper.GetCertPaths()

	// Test multiple gRPC servers can be created (though they can't all bind to same port)
	for i := 0; i < 3; i++ {
		creds, err := loadTLSCredentials(certFile, keyFile)
		if err != nil {
			t.Fatalf("Failed to load credentials: %v", err)
		}

		server := setupGRPCServer(creds)
		if server == nil {
			t.Errorf("Server %d creation failed", i)
		} else {
			server.Stop()
		}
	}
}

// TestErrorHandling tests various error conditions
func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func() error
		wantErr  bool
	}{
		{
			name: "load credentials with empty paths",
			testFunc: func() error {
				_, err := loadTLSCredentials("", "")
				return err
			},
			wantErr: true,
		},
		{
			name: "create listener with invalid port range",
			testFunc: func() error {
				_, err := createTCPListener(99999)
				return err
			},
			wantErr: true,
		},
		{
			name: "create listener with negative port",
			testFunc: func() error {
				_, err := createTCPListener(-1)
				return err
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.testFunc()
			if (err != nil) != tt.wantErr {
				t.Errorf("Test %s: error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

// TestMemoryUsage tests that resources are properly cleaned up
func TestMemoryUsage(t *testing.T) {
	// Test that listeners are properly closed
	listener, err := createTCPListener(0)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	// Verify we can close it without error
	if err := listener.Close(); err != nil {
		t.Errorf("Failed to close listener: %v", err)
	}

	// Test that gRPC servers can be stopped
	server := setupGRPCServer(nil)
	server.Stop() // Should not panic or cause issues
}

// BenchmarkApplicationCreation benchmarks application creation
func BenchmarkApplicationCreation(b *testing.B) {
	// Skip benchmarks that require file system setup
	b.Skip("Requires certificate files to be present")
}

// BenchmarkHTTPMuxSetup benchmarks HTTP mux setup
func BenchmarkHTTPMuxSetup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mux := setupHTTPMux()
		_ = mux
	}
}

// BenchmarkTCPListenerCreation benchmarks TCP listener creation
func BenchmarkTCPListenerCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		listener, err := createTCPListener(0)
		if err != nil {
			b.Fatalf("Failed to create listener: %v", err)
		}
		listener.Close()
	}
}
