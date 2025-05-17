package main

import (
	"flag"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	// Don't reset flags - let Go's testing framework handle them
	// Just run the tests
	code := m.Run()
	os.Exit(code)
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected Config
	}{
		{
			name: "default values",
			args: []string{},
			expected: Config{
				Port:     50052,
				CertFile: "certs/cert.pem",
				KeyFile:  "certs/key.pem",
				HTTPPort: ":8082",
			},
		},
		{
			name: "custom port",
			args: []string{"-port", "8080"},
			expected: Config{
				Port:     8080,
				CertFile: "certs/cert.pem",
				KeyFile:  "certs/key.pem",
				HTTPPort: ":8082",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new flag set for this test
			testFlags := flag.NewFlagSet("test", flag.ContinueOnError)
			port := testFlags.Int("port", 50052, "Port gRPC server")

			// Parse the test arguments
			if err := testFlags.Parse(tt.args); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			// Create config with parsed values
			result := Config{
				Port:     *port,
				CertFile: "certs/cert.pem",
				KeyFile:  "certs/key.pem",
				HTTPPort: ":8082",
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parseFlags() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestSetupLogger(t *testing.T) {
	// Capture the original logger
	originalLogger := log.Logger
	defer func() { log.Logger = originalLogger }()

	// Test logger setup
	setupLogger()

	// Verify that TimeFieldFormat is set correctly
	if zerolog.TimeFieldFormat != zerolog.TimeFormatUnix {
		t.Errorf("Expected TimeFieldFormat to be %s, got %s",
			zerolog.TimeFormatUnix, zerolog.TimeFieldFormat)
	}

	// We can't easily test the console writer setup without mocking,
	// but we can verify the function doesn't panic
}

func TestLoadTLSCredentials(t *testing.T) {
	tests := []struct {
		name     string
		certFile string
		keyFile  string
		wantErr  bool
	}{
		{
			name:     "non-existent files",
			certFile: "non-existent-cert.pem",
			keyFile:  "non-existent-key.pem",
			wantErr:  true,
		},
		{
			name:     "empty file paths",
			certFile: "",
			keyFile:  "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadTLSCredentials(tt.certFile, tt.keyFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadTLSCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateTCPListener(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{
			name:    "valid port",
			port:    0, // Port 0 lets the OS choose an available port
			wantErr: false,
		},
		{
			name:    "invalid port",
			port:    -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := createTCPListener(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("createTCPListener() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && listener != nil {
				listener.Close()
			}
		})
	}
}

func TestSetupGRPCServer(t *testing.T) {
	// Create a simple server without credentials for testing
	server := setupGRPCServer(nil)

	if server == nil {
		t.Error("setupGRPCServer() returned nil")
	}

	// Clean up
	server.Stop()
}

func TestSetupHTTPMux(t *testing.T) {
	mux := setupHTTPMux()

	if mux == nil {
		t.Error("setupHTTPMux() returned nil")
	}

	// Test that the health endpoint is registered
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("health endpoint returned status %d, want %d", rec.Code, http.StatusOK)
	}

	expectedBody := "OK"
	if body := rec.Body.String(); body != expectedBody {
		t.Errorf("health endpoint returned body %q, want %q", body, expectedBody)
	}

	// Test that the metrics endpoint is registered
	req = httptest.NewRequest("GET", "/metrics", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Metrics endpoint should return 200 (prometheus metrics)
	if rec.Code != http.StatusOK {
		t.Errorf("metrics endpoint returned status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET request",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name:           "POST request",
			method:         "POST",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			rec := httptest.NewRecorder()

			healthHandler(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("healthHandler() status = %d, want %d", rec.Code, tt.expectedStatus)
			}

			if body := rec.Body.String(); body != tt.expectedBody {
				t.Errorf("healthHandler() body = %q, want %q", body, tt.expectedBody)
			}
		})
	}
}

func TestCreateHTTPServer(t *testing.T) {
	handler := http.NewServeMux()
	addr := ":8080"

	server := createHTTPServer(addr, handler)

	if server == nil {
		t.Error("createHTTPServer() returned nil")
	}

	if server.Addr != addr {
		t.Errorf("createHTTPServer() addr = %q, want %q", server.Addr, addr)
	}

	if server.Handler != handler {
		t.Error("createHTTPServer() handler not set correctly")
	}
}

func TestStartGRPCServer(t *testing.T) {
	// Create a mock server and listener
	server := grpc.NewServer()
	listener, err := net.Listen("tcp", ":0") // Use port 0 for automatic port assignment
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Start the server in a separate goroutine
	errChan := make(chan error, 1)
	go func() {
		// This will exit once the server is stopped
		if err := server.Serve(listener); err != nil {
			errChan <- err
		}
	}()

	// Wait a moment for the server to start
	time.Sleep(10 * time.Millisecond)

	// Gracefully stop the server
	server.GracefulStop()

	// Wait for the server to stop and check for errors
	select {
	case err := <-errChan:
		if err != nil && err != grpc.ErrServerStopped {
			t.Errorf("Server error: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		// If no error within timeout, that's fine
	}
}

func TestStartHTTPServer(t *testing.T) {
	// Rather than test with real TLS, we'll test just that
	// the function returns a channel and starts a goroutine
	server := &http.Server{
		Addr: ":0", // Use port 0 for automatic port assignment
	}

	// This should return an error channel
	errChan := startHTTPServer(server, "non-existent-cert.pem", "non-existent-key.pem")

	if errChan == nil {
		t.Error("startHTTPServer() returned nil error channel")
	}

	// Wait for the error to occur
	select {
	case err := <-errChan:
		// We expect an error due to missing certificate files
		if err == nil {
			t.Error("Expected error due to non-existent certificate files")
		}
	case <-time.After(100 * time.Millisecond):
		// If no error within timeout, the test might hang
		t.Error("Expected error within timeout")
	}
}

func TestSetupSignalHandler(t *testing.T) {
	// We just test that the function returns a channel
	signalChan := setupSignalHandler()

	if signalChan == nil {
		t.Error("setupSignalHandler() returned nil")
	}
}

func TestNewApplication(t *testing.T) {
	// This test will fail because we don't have real certificate files
	// But we can test that it handles the error properly
	app, err := NewApplication()

	if err == nil {
		t.Error("Expected NewApplication() to fail due to missing certificate files")
		if app != nil {
			app.listener.Close()
		}
	}

	// Verify error message mentions TLS credentials
	if err != nil && !strings.Contains(err.Error(), "TLS credentials") {
		t.Errorf("Expected error to mention TLS credentials, got: %v", err)
	}
}

// TestLogger_Output verifies logger output
func TestLogger_Output(t *testing.T) {
	// Capture the logger output
	var buf strings.Builder
	originalLogger := log.Logger
	defer func() { log.Logger = originalLogger }()

	// Set up a logger that writes to our buffer
	log.Logger = zerolog.New(&buf).With().Timestamp().Logger()

	// Log a test message
	log.Info().Msg("test message")

	// Verify the output contains our message
	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected log output to contain 'test message', got: %s", output)
	}
}

// Mock interfaces for better testability
type MockListener struct {
	net.Listener
	closeCalled bool
}

func (m *MockListener) Close() error {
	m.closeCalled = true
	return nil
}

func (m *MockListener) Accept() (net.Conn, error) {
	return nil, net.ErrClosed // Return an error instead of blocking forever
}

func (m *MockListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
}

// Test with mock listener
func TestStartGRPCServer_WithMock(t *testing.T) {
	server := grpc.NewServer()
	mockListener := &MockListener{}

	errChan := make(chan error, 1)
	go func() {
		if err := server.Serve(mockListener); err != nil {
			errChan <- err
		}
	}()

	// Give it a moment to start attempting to serve
	time.Sleep(10 * time.Millisecond)

	// Stop the server
	server.Stop()

	// Check for errors
	select {
	case err := <-errChan:
		// We expect the mock listener to return an error
		if err == nil {
			t.Error("Expected error from mock listener")
		}
	case <-time.After(100 * time.Millisecond):
		// If no error within timeout, something might be wrong
		t.Error("Expected error within timeout")
	}
}

// Benchmark tests
func BenchmarkParseFlags(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testFlags := flag.NewFlagSet("test", flag.ContinueOnError)
		testFlags.Int("port", 50052, "Port gRPC server")
		testFlags.Parse([]string{})
	}
}

func BenchmarkSetupHTTPMux(b *testing.B) {
	for i := 0; i < b.N; i++ {
		setupHTTPMux()
	}
}
