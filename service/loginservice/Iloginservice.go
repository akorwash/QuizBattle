package loginservice

import (
	"os"
	"time"

	"github.com/akorwash/QuizBattle/actor"
	"github.com/dgrijalva/jwt-go"

	"github.com/akorwash/QuizBattle/handler"
)

//ILoginServices to do
type ILoginServices interface {
	Login() bool
	GetUser(_id string) *actor.User
}

//Login to do
func Login(loginservice ILoginServices) bool {
	return loginservice.Login()
}

//LoginFactory to do
func LoginFactory(_id string, _pass string) ILoginServices {
	var loginModel ILoginServices

	switch {
	case handler.IsEmailValid(_id):
		loginModel = NewEmailLogin(_id, _pass)
		break
	case handler.IsMobileNumberValid(_id):
		loginModel = NewMobileLogin(_id, _pass)
		break
	default:
		loginModel = NewUsernameLogin(_id, _pass)
		break
	}
	return loginModel
}

//CreateToken to do
func CreateToken(user *actor.User) (string, error) {
	var err error
	//Creating Access Token
	os.Setenv("ACCESS_SECRET", "KMN3KIJnj32iN3KNh6952ub34NGF3J2H4HU32B4Cr43d3FVG") //this should be in an env file
	atClaims := jwt.MapClaims{}
	atClaims["authorized"] = true
	atClaims["username"] = user.GetUserName()
	atClaims["mobileNumber"] = user.GetMobileNumber()
	atClaims["Email"] = user.GetEmail()
	atClaims["exp"] = time.Now().Add(time.Hour * 336).Unix()
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	token, err := at.SignedString([]byte(os.Getenv("ACCESS_SECRET")))
	if err != nil {
		return "", err
	}
	return token, nil
}
