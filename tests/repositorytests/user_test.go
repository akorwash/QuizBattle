package repositorytests

import (
	"fmt"
	"testing"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/stretchr/testify/assert"
)

var userRepo repository.UserRepository

func TestGetUserByName(t *testing.T) {
	fakeUser, err := seedtestUser()
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
	ruser, rerr := userRepo.GetUserByName(fakeUser.Username)
	if rerr != nil {
		fmt.Printf("This is the error %v\n", err)
	}

	assert.Equal(t, fakeUser, ruser)

	ruser, rerr = userRepo.GetUserByName("Not Username")
	if rerr != nil {
		fmt.Printf("This is the error %v\n", err)
	}

	assert.NotEqual(t, fakeUser, ruser)

	err = deletetestUser(fakeUser)
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
}

func TestGetUserByMobile(t *testing.T) {
	fakeUser, err := seedtestUser()
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
	ruser, rerr := userRepo.GetUserByMobile(fakeUser.MobileNumber)
	if rerr != nil {
		fmt.Printf("This is the error %v\n", err)
	}

	assert.Equal(t, fakeUser, ruser)

	ruser, rerr = userRepo.GetUserByName("Not Username")
	if rerr != nil {
		fmt.Printf("This is the error %v\n", err)
	}

	assert.NotEqual(t, fakeUser, ruser)

	err = deletetestUser(fakeUser)
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
}

func TestGetUserByEmail(t *testing.T) {
	fakeUser, err := seedtestUser()
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
	ruser, rerr := userRepo.GetUserByEmail(fakeUser.Email)
	if rerr != nil {
		fmt.Printf("This is the error %v\n", err)
	}

	assert.Equal(t, fakeUser, ruser)

	ruser, rerr = userRepo.GetUserByName("Not Username")
	if rerr != nil {
		fmt.Printf("This is the error %v\n", err)
	}

	assert.NotEqual(t, fakeUser, ruser)

	err = deletetestUser(fakeUser)
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
}

func TestAddUser(t *testing.T) {
	user := entites.User{Username: "testuser", Password: "TestPass#2010", Email: "test@test.com", MobileNumber: "01585285285"}
	err := userRepo.AddUser(user)
	assert.NoError(t, err)

	err = deletetestUser(&user)
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
}
