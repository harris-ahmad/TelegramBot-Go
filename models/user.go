// models/user.go
package models

import (
	"github.com/jinzhu/gorm"
)

type User struct {
	gorm.Model
	UserId   string `gorm:"unique_index"`
	Username string
}
