package createaccount

import (
	"context"
	"fmt"
	"log"

	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/handler"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service/login"
	"go.mongodb.org/mongo-driver/bson"
)

//ICreateAccountServices services interface to create account
type ICreateAccountServices interface {
	CrateUser(user entites.User) (*resources.UserAccount, error)
}

//CreateAccountServices busniess of how to create account
type CreateAccountServices struct {
}

var userRepo repository.UserRepository

//CrateUser apply busniess of validation and create user if passed or return error
func (services CreateAccountServices) CrateUser(_user entites.User) (*resources.UserAccount, error) {
	err := validateInutes(_user)
	if err != nil {
		return nil, err
	}

	dbcontext, err := datastore.GetContext()
	if err != nil {
		log.Fatal("Error while get database context: \n", err)
		return nil, err

	}

	iter := dbcontext.Collection("users")
	userCount, err := iter.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		println("Error while count users recored: %v\n", err)
		return nil, err

	}

	userentity := entites.User{ID: userCount + 1, Username: _user.Username, Password: _user.Password, Email: _user.Email, MobileNumber: _user.MobileNumber}
	err = userRepo.AddUser(userentity)
	if err != nil {
		return nil, err
	}

	token, err := login.CreateToken(userentity)
	if err != nil {
		return nil, err
	}
	response := resources.UserAccount{Username: userentity.Username, MobileNumber: userentity.MobileNumber, Email: userentity.Email, Token: token}

	return &response, nil
}

//validate models that comes from the body when the user hit the apis
//also validate if the user inputes exist before by another users
//return detailed error
func validateInutes(_user entites.User) error {

	var usernameValidation handler.ValidateUsername
	if !usernameValidation.Validate(_user.Username) {
		return fmt.Errorf("usernane can't start with numbers, or have a whitespace, should be >= 5 char")
	}

	var mobileValidation handler.ValidateMobile
	if !mobileValidation.Validate(_user.MobileNumber) {
		return fmt.Errorf("mobileNumber is wrong")
	}

	var emailValidation handler.ValidateEmail
	if !emailValidation.Validate(_user.Email) {
		return fmt.Errorf("email is wrong")
	}

	var passwordValidation handler.ValidatePassword
	if !passwordValidation.Validate(_user.Password) {
		return fmt.Errorf("password at least one (upper and lower) case letter, at least one (digit and special) character and should be >= 8 char")
	}

	user, errRepo := userRepo.GetUserByName(_user.Username)
	if errRepo != nil && errRepo.Error() != "User not found" {
		return errRepo
	}

	if user != nil {
		return fmt.Errorf("Username found")
	}

	user, errRepo = userRepo.GetUserByEmail(_user.Email)
	if errRepo != nil && errRepo.Error() != "User not found" {
		return errRepo
	}

	if user != nil {
		return fmt.Errorf("Email found")
	}

	user, errRepo = userRepo.GetUserByMobile(_user.MobileNumber)
	if errRepo != nil && errRepo.Error() != "User not found" {
		return errRepo
	}

	if user != nil {
		return fmt.Errorf("MobileNumber found")
	}

	return nil
}
