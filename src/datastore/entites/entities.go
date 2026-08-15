package entites

import (
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Answer answer entity, eacg question will have 4 answers one of them will be is correct
type Answer struct {
	ID        int    `bson:"id" json:"id"`
	Text      string `bson:"text" json:"text"`
	IsCorrect bool   `bson:"iscorrect" json:"-"`
}

// Card card entity, tradable object between players also main object when the battle start.
type Card struct {
	ID    int     `bson:"id" json:"id"`
	Power float32 `bson:"power" json:"power"`
	Owner int64   `bson:"owner" json:"ownerId"`

	Likes int `bson:"likes" json:"likes"`
	Hits  int `bson:"hits" json:"hits"`

	Question Question `bson:"question" json:"question"`
}

// Question question entity
type Question struct {
	ID      int      `bson:"id" json:"id"`
	Header  string   `bson:"header" json:"header"`
	Answers []Answer `bson:"answers" json:"answers"`
}

// User user entity contains personal information
type User struct {
	ID             int64  `bson:"id" json:"id,string"`
	Username       string `bson:"username" json:"username"`
	Fullname       string `bson:"fullname" json:"fullName"`
	YearOfBirth    int    `bson:"yearofbirth" json:"yearOfBirth,omitempty"`
	MonthOfBirth   int    `bson:"monthofbirth" json:"monthOfBirth,omitempty"`
	DayOfBirth     int    `bson:"dayofbirth" json:"dayOfBirth,omitempty"`
	HashedPassword string `bson:"hashedpassword" json:"-"`
	Email          string `bson:"email" json:"email"`
	MobileNumber   string `bson:"mobilenumber" json:"mobileNumber"`
}

// ValidatePassword get name of Bot
func (userAccount *User) ValidatePassword(_pass string) bool {
	return comparePasswords(userAccount.HashedPassword, []byte(_pass))
}

// compare password with hashed one
func comparePasswords(hashedPwd string, plainPwd []byte) bool {
	// Since we'll be getting the hashed password from the DB it
	// will be a string so we'll need to convert it to a byte slice
	byteHash := []byte(hashedPwd)
	err := bcrypt.CompareHashAndPassword(byteHash, plainPwd)
	if err != nil {
		return false
	}

	return true
}

// HashAndSalt hash string and salt it
func HashPassword(password string) (string, error) {
	if len(password) == 0 || len(password) > 72 {
		return "", fmt.Errorf("password must contain between 1 and 72 bytes")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// Bot is class represent Player User account with login data
type Bot struct {
	ID      int    `bson:"id" json:"id"`
	BotName string `bson:"botname" json:"name"`
	Level   int    `bson:"level" json:"level"`
}

// Game class represnt game history
type Game struct {
	ID          int64     `bson:"id" json:"id,string"`
	IsPublic    bool      `bson:"ispublic" json:"isPublic"`
	IsActive    bool      `bson:"isactive" json:"isActive"`
	UserID      int64     `bson:"userid" json:"ownerId"`
	Mode        string    `bson:"mode,omitempty" json:"mode"`
	MaxPlayers  int       `bson:"maxplayers,omitempty" json:"maxPlayers"`
	TimeLine    []string  `bson:"timeline" json:"timeline,omitempty"`
	JoinedUsers []int64   `bson:"joinedusers" json:"joinedUserIds"`
	CreatedAt   time.Time `bson:"createdat" json:"createdAt"`
	State       string    `bson:"state" json:"state"`
	MatchID     int64     `bson:"matchid,omitempty" json:"matchId,omitempty,string"`
}
