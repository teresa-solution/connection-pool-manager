package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/teresa-solution/connection-pool-manager/pkg/pool"
	pb "github.com/teresa-solution/connection-pool-manager/proto"
	"google.golang.org/grpc"
)

type ConnectionPoolServiceServer struct {
	pb.UnimplementedConnectionPoolServiceServer
	poolManager *pool.ConnectionPoolManager
}

func NewConnectionPoolServiceServer() *ConnectionPoolServiceServer {
	return &ConnectionPoolServiceServer{
		poolManager: pool.NewConnectionPoolManager(),
	}
}

func (s *ConnectionPoolServiceServer) GetConnection(ctx context.Context, req *pb.ConnectionRequest) (*pb.ConnectionResponse, error) {
	_, err := s.poolManager.GetConnection(ctx, req.TenantId, req.Dsn)
	if err != nil {
		return &pb.ConnectionResponse{Error: err.Error()}, err
	}
	// Simulate connection ID (in practice, this could be a unique identifier)
	connID := fmt.Sprintf("conn-%s-%d", req.TenantId, time.Now().UnixNano())
	return &pb.ConnectionResponse{ConnectionId: connID}, nil
}

func (s *ConnectionPoolServiceServer) ReleaseConnection(ctx context.Context, req *pb.ConnectionRelease) (*pb.ReleaseResponse, error) {
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

func (s *ConnectionPoolServiceServer) GetPoolStats(ctx context.Context, req *pb.StatsRequest) (*pb.StatsResponse, error) {
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

// Register the gRPC server
func RegisterServer(s *grpc.Server, srv *ConnectionPoolServiceServer) {
	pb.RegisterConnectionPoolServiceServer(s, srv)
}
