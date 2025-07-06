# Bixor Engine - Windows Setup Script
Write-Host "🚀 Starting Bixor Engine..." -ForegroundColor Green

# Check if Docker is running
Write-Host "🐳 Checking Docker..." -ForegroundColor Blue
try {
    docker version | Out-Null
    Write-Host "✅ Docker is running" -ForegroundColor Green
} catch {
    Write-Host "❌ Docker is not running. Please start Docker Desktop first." -ForegroundColor Red
    exit 1
}

# Start databases
Write-Host "🗄️ Starting PostgreSQL and Redis..." -ForegroundColor Blue
docker-compose up -d

# Wait for databases to be ready
Write-Host "⏳ Waiting for databases to initialize..." -ForegroundColor Yellow
Start-Sleep -Seconds 15

# Check if databases are ready
$postgres_ready = docker exec bixor-postgres pg_isready -U postgres
$redis_ready = docker exec bixor-redis redis-cli ping

if ($postgres_ready -match "accepting connections" -and $redis_ready -eq "PONG") {
    Write-Host "✅ Databases are ready!" -ForegroundColor Green
    Write-Host "📊 Database Admin Panel: http://localhost:8081" -ForegroundColor Cyan
    Write-Host "   User: postgres, Password: postgres" -ForegroundColor Cyan
} else {
    Write-Host "❌ Databases failed to start. Check logs:" -ForegroundColor Red
    Write-Host "   docker-compose logs postgres" -ForegroundColor Yellow
    Write-Host "   docker-compose logs redis" -ForegroundColor Yellow
    exit 1
}

# Set environment variables
Write-Host "⚙️ Setting environment variables..." -ForegroundColor Blue
$env:DB_HOST = "localhost"
$env:DB_PORT = "5432"
$env:DB_USER = "postgres"
$env:DB_PASSWORD = "postgres"
$env:DB_NAME = "bixor_db"
$env:REDIS_HOST = "localhost"
$env:REDIS_PORT = "6379"
$env:REDIS_PASSWORD = ""
$env:SERVER_PORT = "8080"
$env:GIN_MODE = "debug"
$env:JWT_SECRET = "your-super-secret-jwt-key-change-this-in-production"
$env:ENABLE_TRADING = "true"
$env:MAX_ORDERS_PER_USER = "1000"
$env:ORDER_TIMEOUT = "3600"

# Install Go dependencies
Write-Host "📦 Installing Go dependencies..." -ForegroundColor Blue
go mod tidy

# Start the server
Write-Host "🎯 Starting Bixor Engine server..." -ForegroundColor Green
Write-Host "   Server will be available at: http://localhost:8080" -ForegroundColor Cyan
Write-Host "   API Documentation: http://localhost:8080/api/v1" -ForegroundColor Cyan
Write-Host "   Health Check: http://localhost:8080/health" -ForegroundColor Cyan
Write-Host ""
Write-Host "Press Ctrl+C to stop the server" -ForegroundColor Yellow
Write-Host ""

go run cmd/server/main.go 