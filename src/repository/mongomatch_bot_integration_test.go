package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoMatchCreatesBotDuelWithoutAUserSeatIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	const (
		gameID  int64 = 71_001
		matchID int64 = 72_001
		ownerID int64 = 73_001
	)
	now := time.Now().UTC()
	mustInsert(t, ctx, database.Collection("Game"), entites.Game{
		ID: gameID, UserID: ownerID, IsPublic: false, IsActive: true,
		Mode: string(matchdomain.ModeBot), MaxPlayers: 2, JoinedUsers: []int64{ownerID}, State: "lobby", CreatedAt: now,
		Bot: &entites.BotSeat{ActorID: matchdomain.BotActorID, Name: "حارس المعرفة", Strategy: string(matchdomain.BotSmart)},
	})
	aggregate, err := matchdomain.NewBotDuel(
		matchID, gameID, ownerID, matchdomain.BotSmart, bytes.Repeat([]byte{0x71}, matchdomain.BotSeedSize), now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.CommitBotDeck(repositoryBotDeck(matchdomain.BotActorID, 74_000), "bot-deck-create-001", now); err != nil {
		t.Fatal(err)
	}
	if err := NewMongoMatchRepository(database).CreateForGame(ctx, aggregate); err != nil {
		t.Fatal(err)
	}

	var stored matchdomain.Aggregate
	if err := database.Collection(matchCollection).FindOne(ctx, bson.M{"id": matchID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.Players) != 2 || stored.Players[0].UserID != ownerID || !stored.Players[1].IsBot() ||
		stored.Players[1].Bot == nil || stored.Players[1].Bot.Strategy != matchdomain.BotSmart {
		t.Fatalf("unexpected stored bot match: %+v", stored)
	}
	game, err := NewMongoGameRepository(database).GetGameByID(gameID)
	if err != nil {
		t.Fatal(err)
	}
	if game.MatchID != matchID || game.State != string(matchdomain.StatusCollectingDecks) || len(game.JoinedUsers) != 1 {
		t.Fatalf("bot game was not attached safely: %+v", game)
	}
	public, err := NewMongoGameRepository(database).GetPublicBattle()
	if err != nil {
		t.Fatal(err)
	}
	if len(public) != 0 {
		t.Fatalf("private bot battle leaked into public lobby: %+v", public)
	}
	mine, err := NewMongoGameRepository(database).GetMyBattle(ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 1 || mine[0].ID != gameID {
		t.Fatalf("owner could not reopen bot battle: %+v", mine)
	}
}

func TestMongoGameBotArenaCannotBeJoinedAtRepositoryBoundaryIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	const (
		gameID  int64 = 75_001
		ownerID int64 = 75_002
	)
	mustInsert(t, ctx, database.Collection("Game"), entites.Game{
		ID: gameID, UserID: ownerID, IsPublic: false, IsActive: true,
		Mode: string(matchdomain.ModeBot), MaxPlayers: 2, JoinedUsers: []int64{ownerID}, State: "lobby",
		Bot: &entites.BotSeat{ActorID: matchdomain.BotActorID, Name: "Bot", Strategy: string(matchdomain.BotRandom)},
	})

	if err := NewMongoGameRepository(database).JoinGame(gameID, ownerID+1); !errors.Is(err, ErrConflict) {
		t.Fatalf("bot arena join returned %v", err)
	}
	game, err := NewMongoGameRepository(database).GetGameByID(gameID)
	if err != nil {
		t.Fatal(err)
	}
	if len(game.JoinedUsers) != 1 || game.JoinedUsers[0] != ownerID {
		t.Fatalf("bot arena roster changed: %+v", game.JoinedUsers)
	}
}

func TestMongoMatchBotCreationBindsArenaOwnerModeAndPrivacyIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	tests := []struct {
		name   string
		mutate func(*entites.Game)
	}{
		{"owner", func(game *entites.Game) { game.UserID++ }},
		{"mode", func(game *entites.Game) { game.Mode = string(matchdomain.ModeDuel) }},
		{"privacy", func(game *entites.Game) { game.IsPublic = true }},
		{"strategy", func(game *entites.Game) { game.Bot.Strategy = string(matchdomain.BotRandom) }},
	}
	for index, test := range tests {
		gameID := int64(76_000 + index)
		matchID := int64(77_000 + index)
		ownerID := int64(78_000 + index)
		now := time.Now().UTC()
		game := entites.Game{
			ID: gameID, UserID: ownerID, IsPublic: false, IsActive: true,
			Mode: string(matchdomain.ModeBot), MaxPlayers: 2, JoinedUsers: []int64{ownerID}, State: "lobby",
			Bot: &entites.BotSeat{ActorID: matchdomain.BotActorID, Name: "Bot", Strategy: string(matchdomain.BotSmart)},
		}
		test.mutate(&game)
		mustInsert(t, ctx, database.Collection("Game"), game)
		aggregate, err := matchdomain.NewBotDuel(
			matchID, gameID, ownerID, matchdomain.BotSmart, bytes.Repeat([]byte{byte(0x80 + index)}, matchdomain.BotSeedSize), now,
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := aggregate.CommitBotDeck(repositoryBotDeck(matchdomain.BotActorID, 79_000+int64(index*100)), fmt.Sprintf("bot-deck-filter-%03d", index), now); err != nil {
			t.Fatal(err)
		}
		if err := NewMongoMatchRepository(database).CreateForGame(ctx, aggregate); !errors.Is(err, ErrConflict) {
			t.Fatalf("%s mismatch returned %v", test.name, err)
		}
	}
	count, err := database.Collection(matchCollection).CountDocuments(ctx, bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("mismatched arenas created %d matches", count)
	}
}

func TestMongoGameRejectsMalformedBotEntityBeforeInsert(t *testing.T) {
	game := &entites.Game{
		UserID: 1, IsPublic: true, IsActive: true, Mode: string(matchdomain.ModeBot), MaxPlayers: 2,
		JoinedUsers: []int64{1}, State: "lobby",
		Bot: &entites.BotSeat{ActorID: matchdomain.BotActorID, Name: "Bot", Strategy: string(matchdomain.BotSmart)},
	}
	if err := (&MongoGameRepository{}).Add(game); !errors.Is(err, matchdomain.ErrInvalidMatch) {
		t.Fatalf("malformed bot entity returned %v", err)
	}
}

func repositoryBotDeck(ownerID, idBase int64) []matchdomain.CardSnapshot {
	result := make([]matchdomain.CardSnapshot, matchdomain.DeckSize)
	for index := range result {
		result[index] = matchdomain.CardSnapshot{
			ID: idBase + int64(index+1), OwnerID: ownerID, Rarity: "common", Power: 1,
			Question: matchdomain.QuestionSnapshot{
				ID: fmt.Sprintf("repository-bot-%d-%02d", idBase, index), Prompt: "Question?",
				Options: []string{"A", "B", "C", "D"}, CorrectOption: 0, Difficulty: "easy",
			},
		}
	}
	return result
}
