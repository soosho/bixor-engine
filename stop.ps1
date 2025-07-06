# Bixor Engine - Stop Script
Write-Host "🛑 Stopping Bixor Engine..." -ForegroundColor Yellow

# Stop and remove containers
Write-Host "🐳 Stopping Docker containers..." -ForegroundColor Blue
docker-compose down

# Check if containers are stopped
$running_containers = docker ps --filter "name=bixor-" --quiet
if ($running_containers) {
    Write-Host "⚠️  Some containers are still running:" -ForegroundColor Yellow
    docker ps --filter "name=bixor-" --format "table {{.Names}}\t{{.Status}}"
    Write-Host "Force stopping..." -ForegroundColor Red
    docker stop $running_containers
    docker rm $running_containers
} else {
    Write-Host "✅ All containers stopped successfully" -ForegroundColor Green
}

Write-Host "🎯 Bixor Engine stopped" -ForegroundColor Green 