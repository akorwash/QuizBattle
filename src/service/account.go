package service

import (
	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
)

type AccountService struct {
	userRepo repository.IUserRepository
}

func NewAccountService(userRepo repository.IUserRepository) *AccountService {
	return &AccountService{userRepo: userRepo}
}

func (service *AccountService) GetAccount(userID int64) (*resources.UserAccount, error) {
	user, err := service.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	account := AccountFromUser(user)
	return &account, nil
}

func AccountFromUser(user *entites.User) resources.UserAccount {
	return resources.UserAccount{
		UserID:       user.ID,
		FullName:     user.Fullname,
		Username:     user.Username,
		MobileNumber: user.MobileNumber,
		Email:        user.Email,
		YearOfBirth:  user.YearOfBirth,
		MonthOfBirth: user.MonthOfBirth,
		DayOfBirth:   user.DayOfBirth,
	}
}
