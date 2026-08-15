package service

import (
	"fmt"
	"strings"
	"sync"

	"github.com/akorwash/QuizBattle/datastore/entites"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
)

const (
	maximumActiveBattlesPerOwner = 3
	maximumPlayersPerBattle      = 8
)

type GameEventPublisher interface {
	PublishGameEvent(event resources.GameEvent)
}

type GameAccessCoordinator interface {
	AllowBattleUser(gameID, userID int64)
	DisconnectBattleUser(gameID, userID int64)
}

type GameService struct {
	gameRepo        repository.IGameRepository
	userRepo        repository.IUserRepository
	events          GameEventPublisher
	createMu        sync.Mutex
	membershipLocks [64]sync.Mutex
}

func NewGameService(gameRepo repository.IGameRepository, userRepo repository.IUserRepository, events GameEventPublisher) *GameService {
	return &GameService{gameRepo: gameRepo, userRepo: userRepo, events: events}
}

func (service *GameService) CreateNewGame(userID int64, model resources.CreateGameModel) (*resources.Game, error) {
	opponentType := strings.ToLower(strings.TrimSpace(model.OpponentType))
	if opponentType == "" {
		if strings.EqualFold(strings.TrimSpace(model.Mode), string(matchdomain.ModeBot)) {
			opponentType = "bot"
		} else {
			opponentType = "human"
		}
	}
	if opponentType != "human" && opponentType != "bot" {
		return nil, fmt.Errorf("%w: opponentType must be human or bot", ErrInvalidInput)
	}
	if opponentType == "bot" {
		rawMode := strings.ToLower(strings.TrimSpace(model.Mode))
		if model.IsPublic {
			return nil, fmt.Errorf("%w: bot battles must be private", ErrInvalidInput)
		}
		if rawMode != "" && rawMode != "duel" && rawMode != "1v1" && rawMode != string(matchdomain.ModeBot) {
			return nil, fmt.Errorf("%w: bot battles are duel-only", ErrInvalidInput)
		}
		model.Mode = string(matchdomain.ModeBot)
		model.MaxPlayers = matchdomain.MaxPlayers(matchdomain.ModeBot)
	} else {
		if strings.EqualFold(strings.TrimSpace(model.Mode), string(matchdomain.ModeBot)) {
			return nil, fmt.Errorf("%w: bot mode requires opponentType bot", ErrInvalidInput)
		}
		if !model.IsPublic {
			return nil, fmt.Errorf("%w: private human battles require an invitation flow that is not implemented yet", ErrInvalidInput)
		}
	}
	mode, _, maximumPlayers, _, err := normalizeGameMode(model.Mode, model.MaxPlayers)
	if err != nil {
		return nil, err
	}
	strategy := ""
	if mode == matchdomain.ModeBot {
		parsed, strategyErr := matchdomain.NormalizeBotStrategy(model.BotStrategy)
		if strategyErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, strategyErr)
		}
		strategy = string(parsed)
	} else if strings.TrimSpace(model.BotStrategy) != "" {
		return nil, fmt.Errorf("%w: botStrategy is only valid for bot battles", ErrInvalidInput)
	}
	if _, err := service.validateUser(userID); err != nil {
		return nil, err
	}
	// A single replica is the supported topology until the realtime/state
	// layer is distributed. Serialize creation so concurrent requests cannot
	// bypass the per-owner quota inside that supported topology.
	service.createMu.Lock()
	defer service.createMu.Unlock()
	activeCount, err := service.gameRepo.CountActiveGame(userID)
	if err != nil {
		return nil, err
	}
	if activeCount >= maximumActiveBattlesPerOwner {
		return nil, ErrActiveGameLimit
	}
	game := &entites.Game{
		IsPublic:    model.IsPublic,
		IsActive:    true,
		UserID:      userID,
		Mode:        string(mode),
		MaxPlayers:  maximumPlayers,
		JoinedUsers: []int64{userID},
		State:       "lobby",
	}
	if mode == matchdomain.ModeBot {
		game.Bot = &entites.BotSeat{ActorID: matchdomain.BotActorID, Name: "حارس المعرفة", Strategy: strategy}
	}
	if err := service.gameRepo.Add(game); err != nil {
		return nil, err
	}
	service.publish("created", game.ID)
	result, err := service.toResource(game)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (service *GameService) JoinGame(userID, gameID int64) (*resources.Game, error) {
	if _, err := service.validateUser(userID); err != nil {
		return nil, err
	}
	unlock := service.lockBattle(gameID)
	defer unlock()
	game, err := service.gameRepo.GetGameByID(gameID)
	if err != nil {
		return nil, err
	}
	if !game.IsActive {
		return nil, ErrGameClosed
	}
	mode, _, maximumPlayers, _, err := gameModeDetails(game)
	if err != nil {
		return nil, err
	}
	// A bot arena is closed to membership even if an old/corrupt document lost
	// one half of the mode/seat pair. Mongo enforces the same invariant.
	if mode == matchdomain.ModeBot || game.Bot != nil {
		return nil, ErrForbidden
	}
	if game.State != "" && game.State != "lobby" {
		return nil, ErrBattleFull
	}
	if containsUser(game.JoinedUsers, userID) {
		return service.toResource(game)
	}
	if !game.IsPublic {
		return nil, ErrForbidden
	}
	if len(game.JoinedUsers) >= maximumPlayers {
		return nil, ErrBattleFull
	}
	if err := service.gameRepo.JoinGame(gameID, userID); err != nil {
		return nil, err
	}
	if coordinator, ok := service.events.(GameAccessCoordinator); ok {
		coordinator.AllowBattleUser(gameID, userID)
	}
	game.JoinedUsers = append(game.JoinedUsers, userID)
	service.publish("joined", gameID)
	result, err := service.toResource(game)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (service *GameService) ExitGame(userID, gameID int64) (*resources.Game, error) {
	if _, err := service.validateUser(userID); err != nil {
		return nil, err
	}
	unlock := service.lockBattle(gameID)
	defer unlock()
	game, err := service.gameRepo.GetGameByID(gameID)
	if err != nil {
		return nil, err
	}
	if !game.IsActive {
		return nil, ErrGameClosed
	}
	if !containsUser(game.JoinedUsers, userID) {
		return nil, ErrForbidden
	}
	if game.State != "" && game.State != "lobby" && game.State != "completed" && game.State != "forfeited" {
		return nil, ErrMatchInProgress
	}
	eventType := "left"
	if game.UserID == userID {
		if err := service.gameRepo.CloseGame(gameID); err != nil {
			return nil, err
		}
		game.IsActive = false
		eventType = "closed"
	} else {
		if err := service.gameRepo.LeaveGame(gameID, userID); err != nil {
			return nil, err
		}
		if coordinator, ok := service.events.(GameAccessCoordinator); ok {
			coordinator.DisconnectBattleUser(gameID, userID)
		}
		game.JoinedUsers = withoutUser(game.JoinedUsers, userID)
	}
	service.publish(eventType, gameID)
	result, err := service.toResource(game)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (service *GameService) GetBattle(userID, gameID int64) (*resources.Game, error) {
	game, err := service.gameRepo.GetGameByID(gameID)
	if err != nil {
		return nil, err
	}
	if !game.IsActive {
		return nil, ErrGameClosed
	}
	if !containsUser(game.JoinedUsers, userID) {
		return nil, ErrForbidden
	}
	return service.toResource(game)
}

func (service *GameService) CanAccessBattle(userID, gameID int64) error {
	game, err := service.gameRepo.GetGameByID(gameID)
	if err != nil {
		return err
	}
	if !game.IsActive || !containsUser(game.JoinedUsers, userID) {
		return ErrForbidden
	}
	return nil
}

func (service *GameService) GetPublicBattles() ([]resources.Game, error) {
	games, err := service.gameRepo.GetPublicBattle()
	if err != nil {
		return nil, err
	}
	return service.mapGames(games)
}

func (service *GameService) GetMyBattles(userID int64) ([]resources.Game, error) {
	if _, err := service.validateUser(userID); err != nil {
		return nil, err
	}
	games, err := service.gameRepo.GetMyBattle(userID)
	if err != nil {
		return nil, err
	}
	return service.mapGames(games)
}

func (service *GameService) mapGames(games []entites.Game) ([]resources.Game, error) {
	ids := make([]int64, 0, len(games)*(maximumPlayersPerBattle+1))
	seen := make(map[int64]struct{}, cap(ids))
	for index := range games {
		_, _, maximumPlayers, _, modeErr := gameModeDetails(&games[index])
		if modeErr != nil || len(games[index].JoinedUsers) > maximumPlayers {
			continue
		}
		appendUserID := func(id int64) {
			if id <= 0 {
				return
			}
			if _, exists := seen[id]; exists {
				return
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
		appendUserID(games[index].UserID)
		for _, id := range games[index].JoinedUsers {
			appendUserID(id)
		}
	}
	users, err := service.userRepo.GetUsersByIDs(ids)
	if err != nil {
		return nil, fmt.Errorf("load battle users: %w", err)
	}
	result := make([]resources.Game, 0, len(games))
	for index := range games {
		_, _, maximumPlayers, _, modeErr := gameModeDetails(&games[index])
		if modeErr != nil || len(games[index].JoinedUsers) > maximumPlayers {
			continue
		}
		game, ok := resourceFromUsers(&games[index], users)
		if !ok {
			continue
		}
		result = append(result, game)
	}
	return result, nil
}

func (service *GameService) toResource(game *entites.Game) (*resources.Game, error) {
	_, _, maximumPlayers, _, err := gameModeDetails(game)
	if err != nil {
		return nil, err
	}
	if len(game.JoinedUsers) > maximumPlayers {
		return nil, fmt.Errorf("battle membership exceeds supported limit")
	}
	ids := make([]int64, 0, len(game.JoinedUsers)+1)
	ids = append(ids, game.UserID)
	ids = append(ids, game.JoinedUsers...)
	users, err := service.userRepo.GetUsersByIDs(ids)
	if err != nil {
		return nil, fmt.Errorf("load battle users: %w", err)
	}
	result, ok := resourceFromUsers(game, users)
	if !ok {
		return nil, fmt.Errorf("load battle owner: %w", repository.ErrNotFound)
	}
	return &result, nil
}

func resourceFromUsers(game *entites.Game, users map[int64]entites.User) (resources.Game, bool) {
	owner, found := users[game.UserID]
	if !found {
		return resources.Game{}, false
	}
	mode, minimumPlayers, maximumPlayers, teamSize, err := gameModeDetails(game)
	if err != nil {
		return resources.Game{}, false
	}
	if !validGameBotConfiguration(game, mode) {
		return resources.Game{}, false
	}
	result := &resources.Game{
		ID:           game.ID,
		IsPublic:     game.IsPublic,
		IsActive:     game.IsActive,
		Owner:        resources.UserModel{ID: owner.ID, FullName: owner.Fullname},
		Mode:         string(mode),
		OpponentType: "human",
		MinPlayers:   minimumPlayers,
		MaxPlayers:   maximumPlayers,
		TeamSize:     teamSize,
		Timeline:     append([]string(nil), game.TimeLine...),
		State:        game.State,
		MatchID:      game.MatchID,
	}
	for index, joinedUserID := range game.JoinedUsers {
		user, found := users[joinedUserID]
		if !found {
			continue
		}
		result.JoinedUsers = append(result.JoinedUsers, resources.UserModel{
			ID: user.ID, FullName: user.Fullname, Team: teamForPlayer(mode, index),
		})
	}
	if game.Bot != nil {
		result.OpponentType = "bot"
		result.BotStrategy = game.Bot.Strategy
		result.JoinedUsers = append(result.JoinedUsers, resources.UserModel{
			ID: game.Bot.ActorID, FullName: game.Bot.Name, IsBot: true, BotStrategy: game.Bot.Strategy,
		})
	}
	return *result, true
}

// validGameBotConfiguration keeps the system participant out of the user
// roster and rejects half-migrated/spoofed bot arenas before projection or
// match preparation. Legacy human duels have neither a mode nor a bot seat and
// remain valid.
func validGameBotConfiguration(game *entites.Game, mode matchdomain.Mode) bool {
	if game == nil {
		return false
	}
	if mode != matchdomain.ModeBot {
		return game.Bot == nil
	}
	if game.Bot == nil || game.UserID <= 0 || game.IsPublic || game.MaxPlayers != matchdomain.MaxPlayers(matchdomain.ModeBot) ||
		game.Bot.ActorID != matchdomain.BotActorID || strings.TrimSpace(game.Bot.Name) == "" ||
		len(game.JoinedUsers) != 1 || game.JoinedUsers[0] != game.UserID {
		return false
	}
	strategy, err := matchdomain.NormalizeBotStrategy(game.Bot.Strategy)
	return err == nil && string(strategy) == game.Bot.Strategy
}

func normalizeGameMode(rawMode string, requestedMaximum int) (matchdomain.Mode, int, int, int, error) {
	if strings.TrimSpace(rawMode) == "" {
		rawMode = string(matchdomain.ModeDuel)
	}
	mode, err := matchdomain.NormalizeMode(rawMode)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	minimumPlayers := matchdomain.MinPlayers(mode)
	maximumPlayers := matchdomain.MaxPlayers(mode)
	teamSize := matchdomain.TeamSize(mode)
	if mode == matchdomain.ModeOpen {
		if requestedMaximum == 0 {
			requestedMaximum = maximumPlayers
		}
		if requestedMaximum < minimumPlayers || requestedMaximum > maximumPlayersPerBattle {
			return "", 0, 0, 0, fmt.Errorf("%w: open battle capacity must be between %d and %d", ErrInvalidInput, minimumPlayers, maximumPlayersPerBattle)
		}
		maximumPlayers = requestedMaximum
	} else if requestedMaximum != 0 && requestedMaximum != maximumPlayers {
		return "", 0, 0, 0, fmt.Errorf("%w: %s battles require exactly %d players", ErrInvalidInput, mode, maximumPlayers)
	}
	return mode, minimumPlayers, maximumPlayers, teamSize, nil
}

func gameModeDetails(game *entites.Game) (matchdomain.Mode, int, int, int, error) {
	if game == nil {
		return "", 0, 0, 0, fmt.Errorf("%w: nil battle", ErrInvalidInput)
	}
	requestedMaximum := game.MaxPlayers
	if strings.TrimSpace(game.Mode) == "" {
		requestedMaximum = 0
	}
	return normalizeGameMode(game.Mode, requestedMaximum)
}

func teamForPlayer(mode matchdomain.Mode, rosterIndex int) int {
	if mode != matchdomain.ModeTeam2v2 && mode != matchdomain.ModeTeam4v4 {
		return 0
	}
	return rosterIndex%2 + 1
}

func (service *GameService) validateUser(userID int64) (*entites.User, error) {
	if userID <= 0 {
		return nil, ErrForbidden
	}
	return service.userRepo.GetUserByID(userID)
}

func (service *GameService) publish(eventType string, gameID int64) {
	if service.events != nil {
		service.events.PublishGameEvent(resources.GameEvent{Type: eventType, GameID: gameID})
	}
}

func containsUser(users []int64, userID int64) bool {
	for _, candidate := range users {
		if candidate == userID {
			return true
		}
	}
	return false
}

func withoutUser(users []int64, userID int64) []int64 {
	result := make([]int64, 0, len(users))
	for _, candidate := range users {
		if candidate != userID {
			result = append(result, candidate)
		}
	}
	return result
}

func (service *GameService) lockBattle(gameID int64) func() {
	index := gameID % int64(len(service.membershipLocks))
	if index < 0 {
		index += int64(len(service.membershipLocks))
	}
	service.membershipLocks[index].Lock()
	return service.membershipLocks[index].Unlock
}
