package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
)

type fakeUserRepository struct {
	users    map[int64]entites.User
	batchErr error
}

func (repo *fakeUserRepository) GetUserByName(name string) (*entites.User, error) {
	for _, user := range repo.users {
		if user.Username == name {
			copy := user
			return &copy, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (repo *fakeUserRepository) GetUserByMobile(mobile string) (*entites.User, error) {
	for _, user := range repo.users {
		if user.MobileNumber == mobile {
			copy := user
			return &copy, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (repo *fakeUserRepository) GetUserByEmail(email string) (*entites.User, error) {
	for _, user := range repo.users {
		if user.Email == email {
			copy := user
			return &copy, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (repo *fakeUserRepository) GetUserByID(id int64) (*entites.User, error) {
	user, ok := repo.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	copy := user
	return &copy, nil
}
func (repo *fakeUserRepository) GetUsersByIDs(ids []int64) (map[int64]entites.User, error) {
	if repo.batchErr != nil {
		return nil, repo.batchErr
	}
	result := make(map[int64]entites.User, len(ids))
	for _, id := range ids {
		if user, ok := repo.users[id]; ok {
			result[id] = user
		}
	}
	return result, nil
}
func (repo *fakeUserRepository) AddUser(user *entites.User) error {
	user.ID = 99
	repo.users[user.ID] = *user
	return nil
}
func (repo *fakeUserRepository) UpdateUser(user entites.User) error {
	repo.users[user.ID] = user
	return nil
}

type fakeGameRepository struct {
	games       map[int64]entites.Game
	nextID      int64
	joinCalled  bool
	leaveCalled bool
	closeCalled bool
}

func (repo *fakeGameRepository) CountActiveGame(userID int64) (int64, error) {
	var count int64
	for _, game := range repo.games {
		terminal := game.State == "completed" || game.State == "forfeited"
		if game.UserID == userID && game.IsActive && !terminal {
			count++
		}
	}
	return count, nil
}
func (repo *fakeGameRepository) Add(game *entites.Game) error {
	repo.nextID++
	game.ID = repo.nextID
	repo.games[game.ID] = *game
	return nil
}
func (repo *fakeGameRepository) GetGameByID(id int64) (*entites.Game, error) {
	game, ok := repo.games[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	game.JoinedUsers = append([]int64(nil), game.JoinedUsers...)
	return &game, nil
}
func (repo *fakeGameRepository) GetPublicBattle() ([]entites.Game, error) {
	var result []entites.Game
	for _, game := range repo.games {
		if game.IsPublic && game.IsActive {
			result = append(result, game)
		}
	}
	return result, nil
}
func (repo *fakeGameRepository) GetMyBattle(userID int64) ([]entites.Game, error) {
	var result []entites.Game
	for _, game := range repo.games {
		if containsUser(game.JoinedUsers, userID) {
			result = append(result, game)
		}
	}
	return result, nil
}
func (repo *fakeGameRepository) JoinGame(gameID, userID int64) error {
	repo.joinCalled = true
	game := repo.games[gameID]
	game.JoinedUsers = append(game.JoinedUsers, userID)
	repo.games[gameID] = game
	return nil
}
func (repo *fakeGameRepository) LeaveGame(gameID, userID int64) error {
	repo.leaveCalled = true
	game := repo.games[gameID]
	game.JoinedUsers = withoutUser(game.JoinedUsers, userID)
	repo.games[gameID] = game
	return nil
}
func (repo *fakeGameRepository) CloseGame(gameID int64) error {
	repo.closeCalled = true
	game := repo.games[gameID]
	game.IsActive = false
	repo.games[gameID] = game
	return nil
}

type eventRecorder struct{ events []resources.GameEvent }

func (recorder *eventRecorder) PublishGameEvent(event resources.GameEvent) {
	recorder.events = append(recorder.events, event)
}

func TestCreateGameUsesAuthenticatedOwner(t *testing.T) {
	users := &fakeUserRepository{users: map[int64]entites.User{7: {ID: 7, Username: "owner", Fullname: "Owner"}}}
	games := &fakeGameRepository{games: make(map[int64]entites.Game), nextID: 100}
	events := &eventRecorder{}
	service := NewGameService(games, users, events)
	game, err := service.CreateNewGame(7, resources.CreateGameModel{IsPublic: true})
	if err != nil {
		t.Fatal(err)
	}
	stored := games.games[game.ID]
	if stored.UserID != 7 || !stored.IsPublic || game.Owner.ID != 7 {
		t.Fatalf("wrong authenticated ownership/privacy: %#v %#v", stored, game)
	}
	if len(events.events) != 1 || events.events[0].Type != "created" {
		t.Fatalf("missing server event: %#v", events.events)
	}
}

func TestCreateGameQuotaIgnoresTerminalBattleHistory(t *testing.T) {
	users := &fakeUserRepository{users: map[int64]entites.User{7: {ID: 7, Username: "owner", Fullname: "Owner"}}}
	games := &fakeGameRepository{games: map[int64]entites.Game{
		1: {ID: 1, UserID: 7, IsActive: true, State: "completed", JoinedUsers: []int64{7}},
		2: {ID: 2, UserID: 7, IsActive: true, State: "forfeited", JoinedUsers: []int64{7}},
		3: {ID: 3, UserID: 7, IsActive: true, State: "lobby", JoinedUsers: []int64{7}},
		4: {ID: 4, UserID: 7, IsActive: true, State: "active", JoinedUsers: []int64{7}},
	}, nextID: 100}

	created, err := NewGameService(games, users, nil).CreateNewGame(7, resources.CreateGameModel{IsPublic: true})
	if err != nil {
		t.Fatalf("terminal battle history consumed the active quota: %v", err)
	}
	if created.State != "lobby" || !created.IsActive {
		t.Fatalf("unexpected created battle: %+v", created)
	}
}

func TestCreateGameRejectsFourthConcurrentBattle(t *testing.T) {
	users := &fakeUserRepository{users: map[int64]entites.User{7: {ID: 7, Username: "owner", Fullname: "Owner"}}}
	games := &fakeGameRepository{games: map[int64]entites.Game{
		1: {ID: 1, UserID: 7, IsActive: true, State: "lobby", JoinedUsers: []int64{7}},
		2: {ID: 2, UserID: 7, IsActive: true, State: "collecting_decks", JoinedUsers: []int64{7}},
		3: {ID: 3, UserID: 7, IsActive: true, State: "active", JoinedUsers: []int64{7}},
	}, nextID: 100}

	_, err := NewGameService(games, users, nil).CreateNewGame(7, resources.CreateGameModel{IsPublic: true})
	if !errors.Is(err, ErrActiveGameLimit) {
		t.Fatalf("expected concurrent battle quota, got %v", err)
	}
	if len(games.games) != maximumActiveBattlesPerOwner {
		t.Fatal("battle was persisted after reaching the concurrent quota")
	}
}

func TestCreateGameRejectsPrivateModeUntilInvitationsExist(t *testing.T) {
	users := &fakeUserRepository{users: map[int64]entites.User{7: {ID: 7, Username: "owner", Fullname: "Owner"}}}
	games := &fakeGameRepository{games: make(map[int64]entites.Game), nextID: 100}
	_, err := NewGameService(games, users, nil).CreateNewGame(7, resources.CreateGameModel{IsPublic: false})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected private mode to be unavailable, got %v", err)
	}
	if len(games.games) != 0 {
		t.Fatal("private battle was persisted without an invitation model")
	}
}

func TestPrivateGameRejectsUninvitedJoin(t *testing.T) {
	users := &fakeUserRepository{users: map[int64]entites.User{
		1: {ID: 1, Username: "owner", Fullname: "Owner"},
		2: {ID: 2, Username: "other", Fullname: "Other"},
	}}
	games := &fakeGameRepository{games: map[int64]entites.Game{10: {ID: 10, UserID: 1, IsActive: true, IsPublic: false, JoinedUsers: []int64{1}}}}
	service := NewGameService(games, users, nil)
	if _, err := service.JoinGame(2, 10); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if games.joinCalled {
		t.Fatal("repository join was called for a private battle")
	}
}

func TestOwnerExitClosesBattleAndMemberExitOnlyLeaves(t *testing.T) {
	users := &fakeUserRepository{users: map[int64]entites.User{
		1: {ID: 1, Username: "owner", Fullname: "Owner"},
		2: {ID: 2, Username: "member", Fullname: "Member"},
	}}
	games := &fakeGameRepository{games: map[int64]entites.Game{10: {ID: 10, UserID: 1, IsActive: true, IsPublic: true, JoinedUsers: []int64{1, 2}}}}
	service := NewGameService(games, users, nil)
	if _, err := service.ExitGame(2, 10); err != nil {
		t.Fatal(err)
	}
	if !games.leaveCalled || games.closeCalled {
		t.Fatal("member exit did not use leave semantics")
	}
	if _, err := service.ExitGame(1, 10); err != nil {
		t.Fatal(err)
	}
	if !games.closeCalled {
		t.Fatal("owner exit did not close the battle")
	}
}

func TestOwnerExitPublishesClosureEvenWhenResponseProjectionFails(t *testing.T) {
	users := &fakeUserRepository{
		users:    map[int64]entites.User{1: {ID: 1, Username: "owner", Fullname: "Owner"}},
		batchErr: errors.New("user lookup unavailable"),
	}
	games := &fakeGameRepository{games: map[int64]entites.Game{10: {ID: 10, UserID: 1, IsActive: true, IsPublic: true, JoinedUsers: []int64{1}}}}
	events := &eventRecorder{}
	_, err := NewGameService(games, users, events).ExitGame(1, 10)
	if err == nil {
		t.Fatal("response projection failure was not returned")
	}
	if games.games[10].IsActive || len(events.events) != 1 || events.events[0].Type != "closed" {
		t.Fatalf("closed battle was left live in realtime state: game=%#v events=%#v", games.games[10], events.events)
	}
}

func TestJoinRejectsFullBattle(t *testing.T) {
	users := &fakeUserRepository{users: map[int64]entites.User{99: {ID: 99, Username: "late", Fullname: "Late Player"}}}
	joined := make([]int64, maximumPlayersPerBattle)
	for index := range joined {
		joined[index] = int64(index + 1)
		users.users[int64(index+1)] = entites.User{ID: int64(index + 1), Username: fmt.Sprintf("player%d", index+1)}
	}
	games := &fakeGameRepository{games: map[int64]entites.Game{10: {ID: 10, UserID: 1, IsActive: true, IsPublic: true, JoinedUsers: joined}}}
	_, err := NewGameService(games, users, nil).JoinGame(99, 10)
	if !errors.Is(err, ErrBattleFull) || games.joinCalled {
		t.Fatalf("full battle was not rejected safely: err=%v joined=%v", err, games.joinCalled)
	}
}

type fakeQuestionRepository struct{ question entites.Question }

func (repo fakeQuestionRepository) GetQuestionByID(int) (*entites.Question, error) {
	copy := repo.question
	return &copy, nil
}

func TestQuestionServiceRedactsCorrectAnswer(t *testing.T) {
	service := NewQuestionServices(fakeQuestionRepository{question: entites.Question{ID: 1, Header: "Q", Answers: []entites.Answer{{ID: 1, Text: "A", IsCorrect: true}}}})
	question, err := service.GetQuestionByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if question.Answers[0].ID != 1 || question.Answers[0].Text != "A" {
		t.Fatalf("unexpected safe question: %#v", question)
	}
}

func TestAccountServiceReturnsOnlyAccountModel(t *testing.T) {
	repo := &fakeUserRepository{users: map[int64]entites.User{1: {ID: 1, Username: "one", Fullname: "One", HashedPassword: "secret"}}}
	account, err := NewAccountService(repo).GetAccount(1)
	if err != nil {
		t.Fatal(err)
	}
	if account.UserID != 1 || account.Username != "one" {
		t.Fatalf("unexpected account: %#v", account)
	}
}
