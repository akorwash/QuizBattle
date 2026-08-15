package controller

import (
	"net/http"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service"
	"github.com/akorwash/QuizBattle/service/createaccount"
	"github.com/akorwash/QuizBattle/service/login"
)

type UserController struct {
	authenticator *Authenticator
	connections   interface {
		DisconnectSession(userID int64, tokenID string, expiresAt time.Time)
	}
}

func NewUserController(authenticator *Authenticator, connections interface {
	DisconnectSession(userID int64, tokenID string, expiresAt time.Time)
}) *UserController {
	return &UserController{authenticator: authenticator, connections: connections}
}

func (controller *UserController) CreateUser(createAccountService *createaccount.CreateAccountServices) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input resources.CreateAccountModel
		if err := decodeJSON(w, r, &input); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		account, err := createAccountService.CreateUser(input)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		if err := controller.issueAccountSession(w, account); err != nil {
			respondServiceError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		responseHandler.RespondWithJSON(w, http.StatusCreated, account)
	}
}

func (controller *UserController) Login(loginService *login.LoginService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input resources.UserLogin
		if err := decodeJSON(w, r, &input); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		user, err := loginService.Authenticate(input.Identifier, input.Password)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		if err := controller.authenticator.IssueSession(w, *user); err != nil {
			respondServiceError(w, err)
			return
		}
		account := service.AccountFromUser(user)
		w.Header().Set("Cache-Control", "no-store")
		responseHandler.RespondWithJSON(w, http.StatusOK, account)
	}
}

func (controller *UserController) UpdateUser(updateAccountService service.IUpdateAccountServices) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		var input resources.UpdateAccountModel
		if err := decodeJSON(w, r, &input); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		account, err := updateAccountService.UpdateUser(identity.UserID, input)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		if err := controller.authenticator.Revoke(identity); err != nil {
			respondServiceError(w, err)
			return
		}
		if err := controller.issueAccountSession(w, account); err != nil {
			respondServiceError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		responseHandler.RespondWithJSON(w, http.StatusOK, account)
		if controller.connections != nil {
			controller.connections.DisconnectSession(identity.UserID, identity.TokenID, controller.authenticator.revocationExpiry(identity))
		}
	}
}

func (controller *UserController) Session(accountService *service.AccountService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		account, err := accountService.GetAccount(identity.UserID)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		responseHandler.RespondWithJSON(w, http.StatusOK, account)
	}
}

func (controller *UserController) Logout(w http.ResponseWriter, r *http.Request) {
	if rawToken, err := controller.authenticator.extractToken(r); err == nil {
		if identity, verifyErr := controller.authenticator.verifySignedToken(rawToken); verifyErr == nil {
			if revokeErr := controller.authenticator.Revoke(identity); revokeErr != nil {
				responseHandler.RespondWithError(w, http.StatusServiceUnavailable, "could not end session safely; retry")
				return
			}
			if controller.connections != nil {
				controller.connections.DisconnectSession(identity.UserID, identity.TokenID, controller.authenticator.revocationExpiry(identity))
			}
		}
	}
	controller.authenticator.ClearSession(w)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (controller *UserController) UserProfilePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./api/view/user.html")
}

func (controller *UserController) issueAccountSession(w http.ResponseWriter, account *resources.UserAccount) error {
	return controller.authenticator.IssueSession(w, entites.User{
		ID:           account.UserID,
		Username:     account.Username,
		Fullname:     account.FullName,
		Email:        account.Email,
		MobileNumber: account.MobileNumber,
	})
}
