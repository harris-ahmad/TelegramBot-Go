// models/alert.go
package models

import (
	"github.com/jinzhu/gorm"
)

type Alert struct {
	gorm.Model
	UserId         string `gorm:"index"`
	TokenSymbol    string
	PriceThreshold float64
	Condition      string
}
