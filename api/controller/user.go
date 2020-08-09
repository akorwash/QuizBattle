package controller

import (
	"encoding/json"
	"net/http"

	"github.com/akorwash/QuizBattle/handler"
	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service/loginservice"

	"github.com/akorwash/QuizBattle/actor"
	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/datastore/entites"
	gameengine "github.com/akorwash/QuizBattle/gameengine"
)

//UserController to do
type UserController struct{}

//CreateUser to do
func (controller *UserController) CreateUser(w http.ResponseWriter, r *http.Request) {
	var _user entites.User
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&_user); err != nil {
		responseHandler.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	user := actor.GetUserByName(_user.Username)
	if user != nil {
		responseHandler.RespondWithError(w, http.StatusBadRequest, "Username found")
		return
	}

	user = actor.GetUserByEmail(_user.Email)
	if user != nil {
		responseHandler.RespondWithError(w, http.StatusBadRequest, "Email found")
		return
	}

	user = actor.GetUserByMobile(_user.MobileNumber)
	if user != nil {
		responseHandler.RespondWithError(w, http.StatusBadRequest, "MobileNumber found")
		return
	}

	if (!validationModel(w, handler.ValidateUsername{}, _user.Username, "Usernane Can't start with numbers, or have a whitespace, should be >= 5 char.")) {
		return
	}
	if (!validationModel(w, handler.ValidateEmail{}, _user.Email, "Email is wrong")) {
		return
	}
	if (!validationModel(w, handler.ValidateMobile{}, _user.MobileNumber, "MobileNumber is wrong")) {
		return
	}
	if (!validationModel(w, handler.ValidatePassword{}, _user.Password, "Password at least one (upper and lower) case letter, at least one (digit and special) character and should be >= 8 char.")) {
		return
	}

	user = actor.NewUser(_user.Username, _user.Password, _user.Email, _user.MobileNumber)
	actor.UserSet = append(actor.UserSet, *user)

	gameengine.CardsSet.GetRandomCard().AssignToUser(user)
	gameengine.CardsSet.GetRandomCard().AssignToUser(user)
	gameengine.CardsSet.GetRandomCard().AssignToUser(user)
	datastore.MyDBContext.SaveUsers()
	datastore.MyDBContext.SaveCards()
	token, err := loginservice.CreateToken(user)
	if err != nil {
		responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	response := resources.UserAccount{Username: user.GetUserName(), MobileNumber: user.GetMobileNumber(), Email: user.GetEmail(), Token: token}
	responseHandler.RespondWithJSON(w, http.StatusCreated, response)
}

//Login to do
func (controller *UserController) Login(w http.ResponseWriter, r *http.Request) {
	var _userLogin resources.UserLogin
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&_userLogin); err != nil {
		responseHandler.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if len(_userLogin.Identifier) <= 0 {
		responseHandler.RespondWithError(w, http.StatusBadRequest, "Identifier Invalid request payload")
		return
	}
	if len(_userLogin.Password) <= 0 {
		responseHandler.RespondWithError(w, http.StatusBadRequest, "Password Invalid request payload")
		return
	}

	loginModel := loginservice.LoginFactory(_userLogin.Identifier, _userLogin.Password)
	switch loginservice.Login(loginModel) {
	case true:
		user := loginModel.GetUser(_userLogin.Identifier)
		token, err := loginservice.CreateToken(user)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		response := resources.UserAccount{Username: user.GetUserName(), MobileNumber: user.GetMobileNumber(), Email: user.GetEmail(), Token: token}
		responseHandler.RespondWithJSON(w, http.StatusCreated, response)
		return
	case false:
		responseHandler.RespondWithError(w, http.StatusBadRequest, "Password Invalid")
		return
	}
}
