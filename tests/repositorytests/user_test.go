package repositorytests

import (
	"context"
	"fmt"
	"testing"

	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

var userRepo repository.IUserRepository

func TestGetUserByName(t *testing.T) {
	fakeUser, err := seedtestUser()
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
	ruser, rerr := userRepo.GetUserByName(fakeUser.Username)
	if rerr != nil && rerr.Error() != "User not found" {
		fmt.Printf("This is the error %v\n", err)
	}

	assert.Equal(t, fakeUser, ruser)

	ruser, rerr = userRepo.GetUserByName("Not Username")
	if rerr != nil && rerr.Error() != "User not found" {
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
	_, rerr := userRepo.GetUserByMobile(fakeUser.MobileNumber)
	if rerr != nil && rerr.Error() != "User not found" {
		fmt.Printf("This is the error %v\n", err)
	}

	assert.NoError(t, rerr)

	_, rerr = userRepo.GetUserByMobile("Not Username")
	assert.Error(t, rerr)

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
	if rerr != nil && rerr.Error() != "User not found" {
		fmt.Printf("This is the error %v\n", err)
	}

	assert.Equal(t, fakeUser, ruser)

	ruser, rerr = userRepo.GetUserByEmail("Not Username")
	if rerr != nil && rerr.Error() != "User not found" {
		fmt.Printf("This is the error %v\n", err)
	}

	assert.NotEqual(t, fakeUser, ruser)

	err = deletetestUser(fakeUser)
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
}

func TestAddUser(t *testing.T) {
	dbcontext, err := datastore.GetContext()
	iter := dbcontext.Collection("users")
	userCount, err := iter.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		println("Error while count users recored: %v\n", err)
		return
	}

	user := entites.User{ID: userCount + 1, Username: "testuser", Password: "TestPass#2010", Email: "test@test.com", MobileNumber: "01585285285"}
	err = userRepo.AddUser(user)
	assert.NoError(t, err)

	err = deletetestUser(&user)
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
}
