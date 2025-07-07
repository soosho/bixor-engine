# API Documentation

This document provides information about the Bixor Exchange API documentation system.

## Overview

The Bixor Exchange API provides comprehensive documentation using OpenAPI 3.0 specification with interactive Swagger UI. The documentation covers all endpoints, authentication methods, request/response schemas, and includes examples for easy testing.

## Accessing Documentation

### Interactive Documentation (Swagger UI)
- **URL**: `http://localhost:8080/docs/`
- **Description**: Interactive API documentation with "Try it out" functionality
- **Features**:
  - Browse all endpoints organized by categories
  - View request/response schemas
  - Test endpoints directly from the browser
  - Authentication support for protected endpoints
  - Example values and responses

### OpenAPI Specification
- **JSON Format**: `http://localhost:8080/api/v1/openapi.json`
- **YAML Format**: `http://localhost:8080/api/v1/openapi.yaml`
- **Info Endpoint**: `http://localhost:8080/api/v1/docs`

## API Categories

### 🏥 Health
- Health check endpoints
- System status monitoring

### 🔐 Authentication
- User registration and login
- JWT token management
- Password management

### 🔢 Two-Factor Authentication
- TOTP setup and verification
- Backup codes management
- 2FA enable/disable

### 🔑 API Keys
- Create and manage API keys
- Permission-based access control
- Key revocation

### 📊 Markets
- Market information
- Order book data
- Trade history
- Market statistics
- Candlestick (OHLCV) data

### 💹 Trading
- Order placement and management
- Order history
- Real-time order status

### 👤 User
- Account information
- Balance queries
- Trading history

### 🔌 WebSocket
- Real-time data streaming
- Market data subscriptions
- User-specific notifications

### ⚙️ Admin
- Administrative functions
- System monitoring
- User management (admin only)

## Authentication Methods

### JWT Bearer Token
For web applications and user sessions:
```http
Authorization: Bearer <your_jwt_token>
```

### API Key Authentication
For programmatic access and trading bots:
```http
X-API-Key: <your_api_key>
X-API-Secret: <your_api_secret>
```

## Rate Limiting

All endpoints are rate limited to ensure fair usage:

- **Public endpoints**: 1000 requests/minute
- **Trading endpoints**: 10 requests/second per user  
- **General authenticated endpoints**: 100 requests/minute per IP

Rate limit information is returned in response headers:
- `X-RateLimit-Limit`: Request limit per window
- `X-RateLimit-Remaining`: Remaining requests in current window
- `X-RateLimit-Reset`: Unix timestamp when the window resets

## WebSocket API

Real-time data is available via WebSocket at `/api/v1/ws`

### Connection
```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/ws');
```

### Subscription Channels
- `orderbook.<market_id>` - Order book updates
- `trades.<market_id>` - Trade updates
- `user_orders` - User order updates (requires auth)
- `user_balances` - User balance updates (requires auth)

### Example Subscription
```json
{
  "type": "subscribe",
  "channel": "orderbook.BTC-USDT"
}
```

## Error Handling

The API uses conventional HTTP response codes:

- `200` - Success
- `201` - Created
- `400` - Bad Request / Validation Error
- `401` - Unauthorized
- `403` - Forbidden
- `404` - Not Found
- `409` - Conflict
- `429` - Too Many Requests (Rate Limited)
- `500` - Internal Server Error

Error responses include structured error information:
```json
{
  "error": "Error message",
  "details": "Additional error details"
}
```

For validation errors:
```json
{
  "error": "Validation failed",
  "details": [
    {
      "field": "email",
      "message": "invalid email format"
    }
  ]
}
```

## Getting Started

1. **Start the server**:
   ```bash
   go run cmd/server/main.go
   ```

2. **Access documentation**:
   - Open your browser to `http://localhost:8080/docs/`

3. **Register a user**:
   - Use the `/api/v1/auth/register` endpoint
   - Provide email, username, and password

4. **Login and get tokens**:
   - Use the `/api/v1/auth/login` endpoint
   - Save the JWT tokens for authenticated requests

5. **Test endpoints**:
   - Use the "Authorize" button in Swagger UI
   - Enter your JWT token to test protected endpoints

## Examples

### Register a New User
```bash
curl -X POST "http://localhost:8080/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "trader@example.com",
    "username": "trader123",
    "password": "SecurePass123!",
    "first_name": "John",
    "last_name": "Doe"
  }'
```

### Login
```bash
curl -X POST "http://localhost:8080/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "trader@example.com",
    "password": "SecurePass123!"
  }'
```

### Get Markets
```bash
curl -X GET "http://localhost:8080/api/v1/markets"
```

### Create an Order (requires authentication)
```bash
curl -X POST "http://localhost:8080/api/v1/orders" \
  -H "Authorization: Bearer <your_jwt_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "market_id": "BTC-USDT",
    "side": 1,
    "type": "limit",
    "price": "50000.00",
    "size": "0.1"
  }'
```

## Development

### Updating Documentation

The OpenAPI specification is located at `docs/swagger.yaml`. To update the documentation:

1. Edit the `docs/swagger.yaml` file
2. Restart the server to reload the documentation
3. Check the updated documentation at `http://localhost:8080/docs/`

### Adding New Endpoints

When adding new endpoints:

1. Add the endpoint definition to `docs/swagger.yaml`
2. Include appropriate tags, parameters, and response schemas
3. Add security requirements if the endpoint requires authentication
4. Update relevant schemas if new data structures are introduced

### Validation

The OpenAPI specification includes comprehensive validation rules:
- Request parameter validation
- Request body validation  
- Response schema validation
- Security requirement validation

## Support

For API questions or issues:
- Check the interactive documentation at `/docs/`
- Review the OpenAPI specification
- Contact the development team

## License

This API documentation is part of the Bixor Exchange project and is licensed under the MIT License. 