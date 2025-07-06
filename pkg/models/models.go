package models

/*
Bixor Engine Database Models

This package contains all database models organized by domain:

- user.go      - User and Balance models
- market.go    - Market and MarketData models  
- trading.go   - Order and Trade models with enums
- utils.go     - Shared utility functions

To add new models:
1. Create a new file for your domain (e.g., wallet.go, admin.go)
2. Define your models with appropriate GORM tags
3. Add TableName() methods if needed
4. Include the models in database.AutoMigrate()

Example:
```go
// pkg/models/wallet.go
type Withdrawal struct {
    ID     uint   `gorm:"primaryKey"`
    UserID uint   `gorm:"not null;index"`
    Amount decimal.Decimal `gorm:"type:decimal(20,8)"`
    // ... other fields
}

func (Withdrawal) TableName() string { return "withdrawals" }
```
*/ 