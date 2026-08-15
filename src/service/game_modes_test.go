package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/akorwash/QuizBattle/datastore/entites"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
	"github.com/akorwash/QuizBattle/resources"
)

func TestCreateGameNormalizesSupportedArenaModes(t *testing.T) {
	tests := []struct {
		name           string
		input          resources.CreateGameModel
		mode           string
		minimumPlayers int
		maximumPlayers int
		teamSize       int
		ownerTeam      int
	}{
		{
			name: "legacy request defaults to duel", input: resources.CreateGameModel{IsPublic: true},
			mode: "duel", minimumPlayers: 2, maximumPlayers: 2, teamSize: 1,
		},
		{
			name: "duel", input: resources.CreateGameModel{IsPublic: true, Mode: "duel"},
			mode: "duel", minimumPlayers: 2, maximumPlayers: 2, teamSize: 1,
		},
		{
			name: "two versus two", input: resources.CreateGameModel{IsPublic: true, Mode: "team_2v2"},
			mode: "team_2v2", minimumPlayers: 4, maximumPlayers: 4, teamSize: 2, ownerTeam: 1,
		},
		{
			name: "four versus four", input: resources.CreateGameModel{IsPublic: true, Mode: "team_4v4"},
			mode: "team_4v4", minimumPlayers: 8, maximumPlayers: 8, teamSize: 4, ownerTeam: 1,
		},
		{
			name: "open defaults to eight", input: resources.CreateGameModel{IsPublic: true, Mode: "open"},
			mode: "open", minimumPlayers: 2, maximumPlayers: 8,
		},
		{
			name: "open custom capacity", input: resources.CreateGameModel{IsPublic: true, Mode: "open", MaxPlayers: 5},
			mode: "open", minimumPlayers: 2, maximumPlayers: 5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users := gameModesTestUsers(1)
			games := &fakeGameRepository{games: make(map[int64]entites.Game), nextID: 100}
			created, err := NewGameService(games, users, nil).CreateNewGame(1, test.input)
			if err != nil {
				t.Fatal(err)
			}

			if created.Mode != test.mode || created.MinPlayers != test.minimumPlayers ||
				created.MaxPlayers != test.maximumPlayers || created.TeamSize != test.teamSize {
				t.Fatalf("unexpected arena contract: %+v", created)
			}
			if len(created.JoinedUsers) != 1 || created.JoinedUsers[0].ID != 1 || created.JoinedUsers[0].Team != test.ownerTeam {
				t.Fatalf("owner was not projected into the expected team: %+v", created.JoinedUsers)
			}

			stored := games.games[created.ID]
			if stored.Mode != test.mode || stored.MaxPlayers != test.maximumPlayers ||
				stored.UserID != 1 || len(stored.JoinedUsers) != 1 || stored.JoinedUsers[0] != 1 {
				t.Fatalf("normalized mode was not persisted: %+v", stored)
			}
		})
	}
}

func TestCreateBotArenaIsPrivateSystemOwnedAndCannotBeJoined(t *testing.T) {
	users := gameModesTestUsers(2)
	games := &fakeGameRepository{games: make(map[int64]entites.Game), nextID: 120}
	gameService := NewGameService(games, users, nil)

	created, err := gameService.CreateNewGame(1, resources.CreateGameModel{
		Mode: "bot", OpponentType: "bot", BotStrategy: "smart", MaxPlayers: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.IsPublic || created.Mode != "bot" || created.OpponentType != "bot" || created.BotStrategy != "smart" {
		t.Fatalf("unexpected bot arena: %+v", created)
	}
	if len(created.JoinedUsers) != 2 || created.JoinedUsers[0].ID != 1 || !created.JoinedUsers[1].IsBot ||
		created.JoinedUsers[1].ID != matchdomain.BotActorID {
		t.Fatalf("unexpected bot roster: %+v", created.JoinedUsers)
	}
	stored := games.games[created.ID]
	if stored.Bot == nil || stored.Bot.ActorID != matchdomain.BotActorID || stored.Bot.Strategy != "smart" || len(stored.JoinedUsers) != 1 {
		t.Fatalf("bot was persisted as a user instead of a system seat: %+v", stored)
	}
	if _, err := gameService.JoinGame(2, created.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("bot arena accepted a human join: %v", err)
	}
}

func TestCreateBotArenaRejectsSpoofedOrIncompatibleOptions(t *testing.T) {
	tests := []resources.CreateGameModel{
		{IsPublic: true, Mode: "bot", OpponentType: "bot", BotStrategy: "random"},
		{IsPublic: true, Mode: "bot", OpponentType: "human", BotStrategy: "smart"},
		{Mode: "team_2v2", OpponentType: "bot", BotStrategy: "smart"},
		{Mode: "bot", OpponentType: "bot", BotStrategy: "impossible"},
		{IsPublic: true, Mode: "duel", OpponentType: "human", BotStrategy: "smart"},
	}
	for _, input := range tests {
		games := &fakeGameRepository{games: make(map[int64]entites.Game), nextID: 130}
		if _, err := NewGameService(games, gameModesTestUsers(1), nil).CreateNewGame(1, input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("input %+v returned %v", input, err)
		}
		if len(games.games) != 0 {
			t.Fatalf("invalid bot arena was stored: %+v", games.games)
		}
	}
}

func TestBotArenaProjectionAndMembershipRejectCorruptConfigurations(t *testing.T) {
	users := gameModesTestUsers(2)
	invalid := []entites.Game{
		{
			ID: 201, UserID: 1, IsPublic: true, IsActive: true, Mode: "bot", MaxPlayers: 2,
			JoinedUsers: []int64{1}, State: "lobby",
			Bot: &entites.BotSeat{ActorID: matchdomain.BotActorID, Name: "Bot", Strategy: "smart"},
		},
		{
			ID: 202, UserID: 1, IsPublic: false, IsActive: true, Mode: "bot", MaxPlayers: 2,
			JoinedUsers: []int64{1}, State: "lobby",
		},
		{
			ID: 203, UserID: 1, IsPublic: true, IsActive: true, Mode: "duel", MaxPlayers: 2,
			JoinedUsers: []int64{1}, State: "lobby",
			Bot: &entites.BotSeat{ActorID: matchdomain.BotActorID, Name: "Bot", Strategy: "random"},
		},
	}
	games := make(map[int64]entites.Game, len(invalid))
	for index := range invalid {
		game := invalid[index]
		games[game.ID] = game
		if _, ok := resourceFromUsers(&game, users.users); ok {
			t.Fatalf("corrupt bot arena %d was projected", game.ID)
		}
	}
	service := NewGameService(&fakeGameRepository{games: games}, users, nil)
	for _, game := range invalid {
		if _, err := service.JoinGame(2, game.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("corrupt bot arena %d accepted membership: %v", game.ID, err)
		}
	}
}

func TestCreateGameRejectsInvalidArenaModesAndCapacities(t *testing.T) {
	tests := []struct {
		name  string
		input resources.CreateGameModel
	}{
		{"unknown mode", resources.CreateGameModel{IsPublic: true, Mode: "battle_royale"}},
		{"duel capacity", resources.CreateGameModel{IsPublic: true, Mode: "duel", MaxPlayers: 3}},
		{"two versus two capacity", resources.CreateGameModel{IsPublic: true, Mode: "team_2v2", MaxPlayers: 2}},
		{"four versus four capacity", resources.CreateGameModel{IsPublic: true, Mode: "team_4v4", MaxPlayers: 7}},
		{"open below minimum", resources.CreateGameModel{IsPublic: true, Mode: "open", MaxPlayers: 1}},
		{"open above maximum", resources.CreateGameModel{IsPublic: true, Mode: "open", MaxPlayers: 9}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			games := &fakeGameRepository{games: make(map[int64]entites.Game), nextID: 100}
			_, err := NewGameService(games, gameModesTestUsers(1), nil).CreateNewGame(1, test.input)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected invalid input, got %v", err)
			}
			if len(games.games) != 0 {
				t.Fatalf("invalid arena was persisted: %+v", games.games)
			}
		})
	}
}

func TestLegacyGameDefaultsToDuelCapacity(t *testing.T) {
	users := gameModesTestUsers(3)
	games := &fakeGameRepository{games: map[int64]entites.Game{
		10: {
			ID: 10, UserID: 1, IsPublic: true, IsActive: true,
			// Mode and MaxPlayers are deliberately absent to emulate legacy BSON.
			JoinedUsers: []int64{1}, State: "",
		},
	}}
	gameService := NewGameService(games, users, nil)

	joined, err := gameService.JoinGame(2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if joined.Mode != "duel" || joined.MinPlayers != 2 || joined.MaxPlayers != 2 || joined.TeamSize != 1 || len(joined.JoinedUsers) != 2 {
		t.Fatalf("legacy game did not project as a duel: %+v", joined)
	}
	if _, err := gameService.JoinGame(3, 10); !errors.Is(err, ErrBattleFull) {
		t.Fatalf("legacy duel accepted a third player: %v", err)
	}
	if got := len(games.games[10].JoinedUsers); got != 2 {
		t.Fatalf("legacy duel membership changed after overflow: got %d", got)
	}
}

func TestGameResourceAssignsStableTeamsByRosterOrder(t *testing.T) {
	tests := []struct {
		name           string
		mode           string
		maximumPlayers int
		minimumPlayers int
		teamSize       int
		teams          []int
	}{
		{"two versus two", "team_2v2", 4, 4, 2, []int{1, 2, 1, 2}},
		{"four versus four", "team_4v4", 8, 8, 4, []int{1, 2, 1, 2, 1, 2, 1, 2}},
		{"open is individual", "open", 7, 2, 0, []int{0, 0, 0, 0, 0, 0, 0}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users := gameModesTestUsers(test.maximumPlayers)
			joined := make([]int64, test.maximumPlayers)
			for index := range joined {
				joined[index] = int64(index + 1)
			}
			entity := entites.Game{
				ID: 70, UserID: 1, IsPublic: true, IsActive: true,
				Mode: test.mode, MaxPlayers: test.maximumPlayers, JoinedUsers: joined, State: "lobby",
			}

			projected, ok := resourceFromUsers(&entity, users.users)
			if !ok {
				t.Fatal("valid arena was not projected")
			}
			if projected.Mode != test.mode || projected.MinPlayers != test.minimumPlayers ||
				projected.MaxPlayers != test.maximumPlayers || projected.TeamSize != test.teamSize {
				t.Fatalf("unexpected projected mode details: %+v", projected)
			}
			if len(projected.JoinedUsers) != len(test.teams) {
				t.Fatalf("unexpected projected roster: %+v", projected.JoinedUsers)
			}
			for index, expectedTeam := range test.teams {
				member := projected.JoinedUsers[index]
				if member.ID != int64(index+1) || member.Team != expectedTeam {
					t.Fatalf("roster index %d: got user=%d team=%d want user=%d team=%d", index, member.ID, member.Team, index+1, expectedTeam)
				}
			}
		})
	}
}

func TestOpenArenaJoinsUpToEightAndRejectsOverflow(t *testing.T) {
	users := gameModesTestUsers(9)
	games := &fakeGameRepository{games: map[int64]entites.Game{
		80: {
			ID: 80, UserID: 1, IsPublic: true, IsActive: true, Mode: "open", MaxPlayers: 8,
			JoinedUsers: []int64{1}, State: "lobby",
		},
	}}
	gameService := NewGameService(games, users, nil)

	for userID := int64(2); userID <= 8; userID++ {
		joined, err := gameService.JoinGame(userID, 80)
		if err != nil {
			t.Fatalf("join player %d: %v", userID, err)
		}
		if got := len(joined.JoinedUsers); got != int(userID) {
			t.Fatalf("join player %d produced roster size %d", userID, got)
		}
	}
	if _, err := gameService.JoinGame(9, 80); !errors.Is(err, ErrBattleFull) {
		t.Fatalf("ninth player was not rejected: %v", err)
	}
	if got := len(games.games[80].JoinedUsers); got != 8 {
		t.Fatalf("overflow changed persisted roster: got %d", got)
	}
}

func gameModesTestUsers(count int) *fakeUserRepository {
	users := make(map[int64]entites.User, count)
	for index := 1; index <= count; index++ {
		id := int64(index)
		users[id] = entites.User{ID: id, Username: fmt.Sprintf("mode-player-%d", index), Fullname: fmt.Sprintf("Player %d", index)}
	}
	return &fakeUserRepository{users: users}
}
