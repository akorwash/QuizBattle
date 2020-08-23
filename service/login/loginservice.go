package login

import (
	"os"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/handler"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/service"
	"github.com/dgrijalva/jwt-go"
)

//LoginService login services
type LoginService struct {
	userRepo repository.IUserRepository
}

//New create instance for Login services
func New(repository repository.IUserRepository) *LoginService {
	return &LoginService{userRepo: repository}
}

//Login here user can login
func Login(loginservice service.ILoginServices) (bool, *entites.User, error) {
	return loginservice.Login()
}

//LoginFactory identity the user login using email or username or mobile number
//There are 3 factory Username, Email, Mobile Login Factory
//Each factory will represent differnent way to login using differnent implmention
func LoginFactory(svc *LoginService, _id string, _pass string) service.ILoginServices {
	var loginModel service.ILoginServices
	switch {
	case handler.IsEmailValid(_id):
		loginModel = NewEmailLogin(svc, _id, _pass)
		break
	case handler.IsMobileNumberValid(_id):
		loginModel = NewMobileLogin(svc, _id, _pass)
		break
	default:
		loginModel = NewUsernameLogin(svc, _id, _pass)
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
	atClaims["user_id"] = user.ID
	atClaims["username"] = user.Username
	atClaims["fullname"] = user.Fullname
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
