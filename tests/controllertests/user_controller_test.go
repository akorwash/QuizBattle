package controllertests

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/stretchr/testify/assert"
)

func TestLogin(t *testing.T) {
	user, err := seedtestUser()
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}

	samples := []struct {
		inputJSON  string
		statusCode int
	}{
		{
			inputJSON:  `{"identifier": "test@test.com", "password": "TestPass#2010"}`,
			statusCode: 200,
		},
		{
			inputJSON:  `{"identifier": "01585285285", "password": "TestPass#2010"}`,
			statusCode: 200,
		},
		{
			inputJSON:  `{"identifier": "testuser", "password": "TestPass#2010"}`,
			statusCode: 200,
		},
		{
			inputJSON:  `{"identifier": "01585285285", "password": ""}`,
			statusCode: 400,
		},
		{
			inputJSON:  `{"identifier": "", "password": "password"}`,
			statusCode: 400,
		},
	}

	for _, v := range samples {
		req, err := http.NewRequest("POST", "/user/login", bytes.NewBufferString(v.inputJSON))
		if err != nil {
			t.Errorf("this is the error: %v", err)
		}
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(userController.Login)
		handler.ServeHTTP(rr, req)

		if !assert.Equal(t, rr.Code, v.statusCode) {
			fmt.Println(rr.Code, v)
		}
	}

	err = deletetestUser(user)
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
}

func TestCreateUser(t *testing.T) {
	user, err := seedtestUser()
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}

	fakeUser := entites.User{Username: "selemi", Email: "xts@email.com", Password: "Mido#R2010", MobileNumber: "01597532225"}
	samples := []struct {
		inputJSON  string
		statusCode int
	}{
		{
			inputJSON:  `{"email" : ` + "\"" + user.Email + "\"" + `,"password" : ` + "\"" + fakeUser.Password + "\"" + `, "mobilenumber": ` + "\"" + fakeUser.MobileNumber + "\"" + `,"username" : ` + "\"" + fakeUser.Username + "\"" + `}`,
			statusCode: 400,
		},
		{
			inputJSON:  `{"email" : ` + "\"" + fakeUser.Email + "\"" + `,"password" : ` + "\"" + fakeUser.Password + "\"" + `, "mobilenumber": ` + "\"" + user.MobileNumber + "\"" + `,"username" : ` + "\"" + fakeUser.Username + "\"" + `}`,
			statusCode: 400,
		},
		{
			inputJSON:  `{"email" : ` + "\"" + fakeUser.Email + "\"" + `,"password" : ` + "\"" + fakeUser.Password + "\"" + `, "mobilenumber": ` + "\"" + fakeUser.MobileNumber + "\"" + `,"username" : ` + "\"" + user.Username + "\"" + `}`,
			statusCode: 400,
		},
		{
			inputJSON:  `{"email" : ` + "\"" + fakeUser.Email + "\"" + `,"password" : ` + "\"" + fakeUser.Password + "\"" + `, "mobilenumber": ` + "\"" + fakeUser.MobileNumber + "\"" + `,"username" : ` + "\"" + fakeUser.Username + "\"" + `}`,
			statusCode: 201,
		},
	}

	for _, v := range samples {
		req, err := http.NewRequest("POST", "/user/createuser", bytes.NewBufferString(v.inputJSON))
		if err != nil {
			t.Errorf("this is the error: %v", err)
		}
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(userController.CreateUser)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, rr.Code, v.statusCode)
	}

	err = deletetestUser(user)
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}

	err = deletetestUser(&fakeUser)
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
}
