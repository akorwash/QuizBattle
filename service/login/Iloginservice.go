package login

import (
	"os"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/handler"
	"github.com/dgrijalva/jwt-go"
)

//ILoginServices interface for login service
type ILoginServices interface {
	Login() (bool, *entites.User, error)
	GetUser(_id string) (*entites.User, error)
}

//Login here user can login
func Login(loginservice ILoginServices) (bool, *entites.User, error) {
	return loginservice.Login()
}

//LoginFactory identity the user login using email or username or mobile number
//There are 3 factory Username, Email, Mobile Login Factory
//Each factory will represent differnent way to login using differnent implmention
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

//CreateToken generate jwt token for the user
func CreateToken(user entites.User) (string, error) {
	var err error
	//Creating Access Token
	atClaims := jwt.MapClaims{}
	atClaims["authorized"] = true
	atClaims["username"] = user.Username
	atClaims["mobileNumber"] = user.MobileNumber
	atClaims["Email"] = user.Email
	atClaims["exp"] = time.Now().Add(time.Hour * 336).Unix()
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	token, err := at.SignedString([]byte(os.Getenv("ACCESS_SECRET")))
	if err != nil {
		return "", err
	}
	return token, nil
}
