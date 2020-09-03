package service

import (
	"fmt"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/websockets"
)

//GameService busniess of how to create account
type GameService struct {
	gameRepo repository.IGameRepository
	userRepo repository.IUserRepository
}

//NewGameService busniess of how to create account
func NewGameService(_gameRepo repository.IGameRepository, _userrepo repository.IUserRepository) *GameService {
	return &GameService{gameRepo: _gameRepo, userRepo: _userrepo}
}

//CreateNewGame to do
func (svc GameService) CreateNewGame(model resources.CreateGameModel) (*resources.Game, error) {
	user, err := svc.userRepo.GetUserByID(int64(model.UserID))
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, fmt.Errorf("User not found")
	}

	countActiveGame, err := svc.gameRepo.CountActiveGame(model.UserID)
	if err != nil {
		return nil, err
	}

	if countActiveGame >= 3 {
		return nil, fmt.Errorf("Can't create another game you have 3 games active")
	}

	gamesCount, err := svc.gameRepo.Count()
	if err != nil {
		return nil, err
	}

	var game = resources.Game{ID: gamesCount + 1, IsActive: true, IsPublic: true}
	err = svc.gameRepo.Add(entites.Game{ID: game.ID, IsActive: true, UserID: model.UserID, IsPublic: true, JoinedUsers: []uint64{model.UserID}})
	if err != nil {
		return nil, err
	}

	if websockets.Games == nil {
		websockets.Games = make(map[int64]resources.Game)
	}

	if websockets.GameConnections == nil {
		websockets.GameConnections = make(map[int64]websockets.Hub)
	}

	joinedUser := resources.UserModel{ID: user.ID, Fullname: user.Fullname}
	game.User = joinedUser
	game.JoinedUser = append(game.JoinedUser, joinedUser)
	websockets.Games[game.ID] = game

	hub := websockets.NewHub()
	go hub.Run()
	websockets.GameConnections[game.ID] = *hub

	return &game, nil
}

//JoinGame to do
func (svc GameService) JoinGame(userID uint64, gameID int64, modAny bool) (*resources.Game, error) {
	//check where the owner user exist in our system
	user, err := svc.validateUser(userID)
	if err != nil {
		return nil, err
	}

	//here we check about join mod, maybe the player need to join or create
	if gameID == 0 {
		return svc.CreateNewGame(resources.CreateGameModel{IsPublic: true, UserID: userID})
	}

	//check where the game already exist in our system
	game, err := svc.gameRepo.GetGameByID(gameID)
	if err != nil {
		return nil, err
	}

	if !game.IsActive {
		return nil, fmt.Errorf("This game is closed")
	}

	//validate the owner of the game already exist
	ownderuser, err := svc.validateUser(game.UserID)
	if err != nil {
		return nil, err
	}
	owneruser := resources.UserModel{ID: ownderuser.ID, Fullname: ownderuser.Fullname}
	//insure that player not joineed the game twice
	alreadyExist, seed := svc.checkExistInJoinedPlayer(userID, game)
	if alreadyExist && !modAny {
		return nil, fmt.Errorf("User already joined this game")
	}

	//wirte to database joined players and update the document
	if seed {
		game.JoinedUsers = append(game.JoinedUsers, userID)
		err = svc.gameRepo.JoinedGame(gameID, game.JoinedUsers)
		if err != nil {
			return nil, err
		}
	}

	if websockets.Games == nil {
		websockets.Games = make(map[int64]resources.Game)
	}

	if _gamesocket, ok := websockets.Games[game.ID]; ok {
		svc.updateSocketGame(_gamesocket, user)
	} else {
		gameSocket := resources.Game{ID: game.ID, IsActive: true, IsPublic: game.IsPublic, User: owneruser, TimeLine: game.TimeLine}
		for _, _juserID := range game.JoinedUsers {
			_juser, err := svc.userRepo.GetUserByID(int64(_juserID))
			if err != nil || user == nil {
				continue
			}

			jUser := resources.UserModel{ID: _juser.ID, Fullname: _juser.Username}
			gameSocket.JoinedUser = append(gameSocket.JoinedUser, jUser)
		}
		websockets.Games[game.ID] = gameSocket
	}

	responseGameModel := websockets.Games[game.ID]
	return &responseGameModel, nil
}

//ExitGame to do
func (svc GameService) ExitGame(userID uint64, gameID int64) (*resources.Game, error) {
	//check where the owner user exist in our system
	_, err := svc.validateUser(userID)
	if err != nil {
		return nil, err
	}

	//check where the game already exist in our system
	game, err := svc.gameRepo.GetGameByID(gameID)
	if err != nil {
		return nil, err
	}

	//insure that player not joineed the game twice
	alreadyExist := svc.checkExistInGame(userID, game)
	if !alreadyExist {
		return nil, fmt.Errorf("User not joined this game yet")
	}

	//wirte to database joined players and update the document
	jusers := svc.getJoinedUsers(userID, game)
	if len(jusers) > 1 {
		err = svc.gameRepo.JoinedGame(gameID, jusers)
		if err != nil {
			return nil, err
		}
	} else {
		err = svc.gameRepo.CloseGame(gameID)
		if err != nil {
			return nil, err
		}

		game, err = svc.gameRepo.GetGameByID(gameID)
		if err != nil {
			return nil, err
		}
	}

	if websockets.Games == nil {
		websockets.Games = make(map[int64]resources.Game)
	}

	if _gamesocket, ok := websockets.Games[game.ID]; ok {
		_gamesocket.IsActive = game.IsActive
		_gamesocket.JoinedUser = svc.getJoinedUsersFromSocketGane(userID, _gamesocket)
		websockets.Games[_gamesocket.ID] = _gamesocket
	} else {
		return nil, fmt.Errorf("Game not intiated by system")
	}

	responseGameModel := websockets.Games[game.ID]
	return &responseGameModel, nil
}

func (svc GameService) validateUser(userID uint64) (*entites.User, error) {
	user, err := svc.userRepo.GetUserByID(int64(userID))
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, fmt.Errorf("User not found")
	}
	return user, nil
}

func (svc GameService) checkExistInJoinedPlayer(userID uint64, game *entites.Game) (notExist bool, seed bool) {
	seed = true
	notExist = false
	for _, uID := range game.JoinedUsers {
		if uID == userID {
			seed = false
			if _gamesocket, ok := websockets.Games[game.ID]; ok {
				for _, joinedUser := range _gamesocket.JoinedUser {
					if joinedUser.ID == int64(userID) {
						notExist = true
					}
				}
			}
		}
	}
	return notExist, seed
}

func (svc GameService) checkExistInGame(userID uint64, game *entites.Game) (notExist bool) {
	notExist = false
	for _, uID := range game.JoinedUsers {
		if uID == userID {
			notExist = true
			break
		}
	}
	return notExist
}

func (svc GameService) getJoinedUsers(userID uint64, game *entites.Game) (users []uint64) {
	for _, uID := range game.JoinedUsers {
		if uID != userID {
			users = append(users, uID)
		}
	}
	return users
}

func (svc GameService) getJoinedUsersFromSocketGane(userID uint64, _gamesocket resources.Game) (users []resources.UserModel) {
	for _, uID := range _gamesocket.JoinedUser {
		if uID.ID != int64(userID) {
			users = append(users, uID)
		}
	}
	return users
}

func (svc GameService) updateSocketGame(_gamesocket resources.Game, user *entites.User) {
	userplayer := resources.UserModel{ID: user.ID, Fullname: user.Username}
	_gamesocket.JoinedUser = append(_gamesocket.JoinedUser, userplayer)
	websockets.Games[_gamesocket.ID] = _gamesocket
}

//GetPublicBattles get public battles
func (svc GameService) GetPublicBattles() ([]resources.Game, error) {
	var response []resources.Game
	games, err := svc.gameRepo.GetPublicBattle()
	if err != nil {
		return nil, fmt.Errorf("Error we can't get data now")
	}

	for _, game := range games {
		ownderuser, err := svc.validateUser(game.UserID)
		if err != nil {
			return nil, err
		}

		owneruser := resources.UserModel{ID: ownderuser.ID, Fullname: ownderuser.Fullname}
		_game := resources.Game{ID: game.ID, IsActive: game.IsActive, IsPublic: game.IsPublic, User: owneruser}

		for _, _juserID := range game.JoinedUsers {
			_juser, err := svc.userRepo.GetUserByID(int64(_juserID))
			if err != nil {
				continue
			}

			jUser := resources.UserModel{ID: _juser.ID, Fullname: _juser.Fullname}
			_game.JoinedUser = append(_game.JoinedUser, jUser)
		}
		response = append(response, _game)
	}
	return response, nil
}

//GetMyBattles get my battles
func (svc GameService) GetMyBattles(userID uint64) ([]resources.Game, error) {
	var response []resources.Game
	games, err := svc.gameRepo.GetMyBattle(userID)
	if err != nil {
		return nil, fmt.Errorf("Error we can't get data now")
	}

	for _, game := range games {
		ownderuser, err := svc.validateUser(game.UserID)
		if err != nil {
			return nil, err
		}

		owneruser := resources.UserModel{ID: ownderuser.ID, Fullname: ownderuser.Fullname}
		_game := resources.Game{ID: game.ID, IsActive: game.IsActive, IsPublic: game.IsPublic, User: owneruser}

		for _, _juserID := range game.JoinedUsers {
			_juser, err := svc.userRepo.GetUserByID(int64(_juserID))
			if err != nil {
				continue
			}

			jUser := resources.UserModel{ID: _juser.ID, Fullname: _juser.Fullname}
			_game.JoinedUser = append(_game.JoinedUser, jUser)
		}
		response = append(response, _game)
	}
	return response, nil
}
