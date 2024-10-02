package types

import (
	"gorm.io/gorm"
)

type Post struct {
	gorm.Model
	ID       int
	Title    string
	ImageURL string
	Body     string
	UserID   int
}

type User struct {
	gorm.Model
	Email    string `gorm:"unique"`
	Password string
	Posts    []Post
}
