
# Ulangi struktur ini untuk setiap layanan (api-gateway, tenant-management-service, connection-pool-manager)

mkdir -p cmd/server
mkdir -p internal/api
mkdir -p internal/grpc
mkdir -p internal/model
mkdir -p internal/service
mkdir -p internal/store
mkdir -p pkg/grpc
mkdir -p pkg/cache
mkdir -p pkg/health
mkdir -p pkg/resilience
mkdir -p configs
mkdir -p scripts/migrations

# Buat file utama
touch cmd/server/main.go
touch configs/config.yaml
touch go.mod

# Di setiap direktori, buat file .gitkeep agar folder tetap ada di Git
find . -type d -empty -not -path "./.git*" -exec touch {}/.gitkeep \;
