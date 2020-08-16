package service

import (
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/websockets"
)

//GameService busniess of how to create account
type GameService struct {
	gameRepo repository.IGameRepository
}

//NewGameService busniess of how to create account
func NewGameService(_gameRepo repository.IGameRepository) *GameService {
	return &GameService{gameRepo: _gameRepo}
}

//CreateNewGame to do
func (svc GameService) CreateNewGame(model resources.CreateGameModel) (*resources.Game, error) {
	return nil, nil
}

//JoinGame to do
func (svc GameService) JoinGame(gameID int) (*resources.Game, error) {
	if gameID == 0 {
		websockets.AddNew(gameID)
		return nil, nil
	}
	return nil, nil
}
