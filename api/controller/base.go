package controller

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/akorwash/QuizBattle/handler"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
)

var responseHandler handler.WebResponseHandler

func isForm(r *http.Request) bool {
	if err := r.ParseForm(); err != nil {
		return false
	}

	return true
}

//GetHandler to do
func GetHandler() handler.WebResponseHandler {
	return responseHandler
}

//TokenAuthMiddleware auth by token middleware
func TokenAuthMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := TokenValid(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

//TokenValid validate jwt token
func TokenValid(r *http.Request) error {
	token, err := VerifyToken(r)
	if err != nil {
		return err
	}
	if _, ok := token.Claims.(jwt.Claims); !ok && !token.Valid {
		return err
	}
	return nil
}

//VerifyToken to do
func VerifyToken(r *http.Request) (*jwt.Token, error) {
	tokenString := ExtractToken(r)
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		//Make sure that the token method conform to "SigningMethodHMAC"
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("ACCESS_SECRET")), nil
	})
	if err != nil {
		return nil, err
	}
	return token, nil
}

//ExtractTokenMetadata to do
func ExtractTokenMetadata(r *http.Request) (*AccessDetails, error) {
	token, err := VerifyToken(r)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if ok && token.Valid {
		username, ok := claims["username"].(string)
		if !ok {
			return nil, err
		}

		email, ok := claims["Email"].(string)
		if !ok {
			return nil, err
		}

		fullname, ok := claims["fullname"].(string)
		if !ok {
			return nil, err
		}

		mobilenumber, ok := claims["mobileNumber"].(string)
		if !ok {
			return nil, err
		}

		_userID, err := strconv.ParseUint(fmt.Sprintf("%.f", claims["user_id"]), 10, 64)
		if err != nil {
			return nil, err
		}

		return &AccessDetails{
			Username:     username,
			Email:        email,
			MobileNumber: mobilenumber,
			UserID:       _userID,
			Fullname:     fullname,
		}, nil
	}
	return nil, err
}

//ExtractToken to do
func ExtractToken(r *http.Request) string {
	vars := mux.Vars(r)
	pathtoken := vars["token"]
	if pathtoken != "" {
		return pathtoken
	}

	bearToken := r.Header.Get("Authorization")
	//normally Authorization the_token_xxx
	strArr := strings.Split(bearToken, " ")
	if len(strArr) == 2 {
		return strArr[1]
	}
	return ""
}

//AccessDetails to do
type AccessDetails struct {
	Username     string
	MobileNumber string
	UserID       uint64
	Fullname     string
	Email        string
}
