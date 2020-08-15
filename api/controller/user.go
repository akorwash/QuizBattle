package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service"
	"github.com/akorwash/QuizBattle/service/login"
)

//UserController user controller
type UserController struct{}

//CreateUser handle create user http request
func (controller *UserController) CreateUser(createAccountService service.ICreateAccountServices) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var _user resources.CreateAccountModel
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&_user); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}
		defer r.Body.Close()

		response, err := createAccountService.CrateUser(_user)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		responseHandler.RespondWithJSON(w, http.StatusCreated, *response)
	}
}

//UpdateUser handle create user http request
func (controller *UserController) UpdateUser(updateAccountSvc service.IUpdateAccountServices) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var _user resources.UpdateAccountModel
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&_user); err != nil {
			fmt.Println("Invalid request payload")
			responseHandler.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}
		defer r.Body.Close()

		response, err := updateAccountSvc.UpdateUser(_user)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}

		responseHandler.RespondWithJSON(w, http.StatusOK, *response)
	}
}

//Login  handle user login http request
func (controller *UserController) Login(loginsvc *login.LoginService) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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

		loginModel := login.LoginFactory(loginsvc, _userLogin.Identifier, _userLogin.Password)
		loginres, user, err := login.Login(loginModel)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
		}
		switch loginres {
		case true:
			token, err := login.CreateToken(*user)
			if err != nil {
				responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
				return
			}

			response := resources.UserAccount{FullName: user.Fullname, Username: user.Username, MobileNumber: user.MobileNumber, Email: user.Email, Token: token}
			responseHandler.RespondWithJSON(w, http.StatusOK, response)
			return
		case false:
			responseHandler.RespondWithError(w, http.StatusBadRequest, "Password Invalid")
			return
		}
	}
}
