package controllertests

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service/createaccount"
	"github.com/akorwash/QuizBattle/service/login"
	"github.com/akorwash/QuizBattle/service/updateaccount"
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
		handler := http.HandlerFunc(userController.Login(login.New(repository.NewMongoUserRepository())))
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

	fakeUser := resources.CreateAccountModel{FullName: "selemi Test", Username: "selemiTestFunc", Email: "xts@email.com", Password: "Mido#R2010", MobileNumber: "01597532225"}
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
		handler := http.HandlerFunc(userController.CreateUser(createaccount.NEW(repository.NewMongoUserRepository())))
		handler.ServeHTTP(rr, req)

		assert.Equal(t, rr.Code, v.statusCode)
	}

	defer func() {
		err = deletetestUser(user)
		if err != nil {
			fmt.Printf("This is the error %v\n", err)
		}

		user, err = repository.NewMongoUserRepository().GetUserByName(fakeUser.Username)
		if err != nil {
			fmt.Printf("This is the error %v\n", err)
		}
		err = deletetestUser(user)
		if err != nil {
			fmt.Printf("This is the error %v\n", err)
		}
	}()

}

func TestUpdateUser(t *testing.T) {
	user, err := seedtestUser()
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}

	samples := []struct {
		inputJSON  string
		statusCode int
	}{
		{
			inputJSON:  `{"ID":10000, "Fullname": "Fullname", "YearOfBirth": 1996, "MonthOfBirth": 7, "DayOfBirth": 5}`,
			statusCode: 400,
		},
		{
			inputJSON:  `{"ID":` + strconv.Itoa(int(user.ID)) + `, "Fullname": "Fullname", "YearOfBirth": 0, "MonthOfBirth": 7, "DayOfBirth": 5}`,
			statusCode: 400,
		},
		{
			inputJSON:  `{"ID":` + strconv.Itoa(int(user.ID)) + `, "Fullname": "Fullname", "YearOfBirth": 1996, "MonthOfBirth": 0, "DayOfBirth": 5}`,
			statusCode: 400,
		},
		{
			inputJSON:  `{"ID":` + strconv.Itoa(int(user.ID)) + `, "Fullname": "Fullname", "YearOfBirth": 1996, "MonthOfBirth": 7, "DayOfBirth": 0}`,
			statusCode: 400,
		},
		{
			inputJSON:  `{"ID":` + strconv.Itoa(int(user.ID)) + `, "Fullname": "Fullname", "YearOfBirth": 1996, "MonthOfBirth": 7, "DayOfBirth": 5}`,
			statusCode: 200,
		},
	}

	for _, v := range samples {
		req, err := http.NewRequest("POST", "/user/updateuser", bytes.NewBufferString(v.inputJSON))
		if err != nil {
			t.Errorf("this is the error: %v", err)
		}
		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(userController.UpdateUser(updateaccount.NEW(repository.NewMongoUserRepository())))
		handler.ServeHTTP(rr, req)

		assert.Equal(t, rr.Code, v.statusCode)
		fmt.Println(v.inputJSON)
	}

	defer func() {
		user, err = repository.NewMongoUserRepository().GetUserByID(user.ID)
		if err != nil {
			fmt.Printf("This is the error %v\n", err)
		}
		err = deletetestUser(user)
		if err != nil {
			fmt.Printf("This is the error %v\n", err)
		}
	}()

}
