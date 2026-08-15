package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoGameJoinHonorsLegacyAndCurrentCapacitiesIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoGameRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	now := time.Now().UTC()

	tests := []struct {
		name     string
		id       int64
		mode     string
		capacity int
		legacy   bool
	}{
		{"legacy duel", 1101, "", 2, true},
		{"duel", 1102, "duel", 2, false},
		{"two versus two", 1103, "team_2v2", 4, false},
		{"four versus four", 1104, "team_4v4", 8, false},
		{"open", 1105, "open", 8, false},
	}

	for testIndex, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ownerID := int64(10_000 + testIndex*100)
			if test.legacy {
				mustInsert(t, ctx, database.Collection("Game"), bson.M{
					"id": test.id, "userid": ownerID, "ispublic": true, "isactive": true,
					"joinedusers": bson.A{ownerID}, "state": "lobby", "createdat": now,
				})
			} else {
				mustInsert(t, ctx, database.Collection("Game"), entites.Game{
					ID: test.id, UserID: ownerID, IsPublic: true, IsActive: true,
					Mode: test.mode, MaxPlayers: test.capacity, JoinedUsers: []int64{ownerID},
					State: "lobby", CreatedAt: now,
				})
			}

			for rosterIndex := 1; rosterIndex < test.capacity; rosterIndex++ {
				if err := repository.JoinGame(test.id, ownerID+int64(rosterIndex)); err != nil {
					t.Fatalf("join roster index %d: %v", rosterIndex, err)
				}
			}
			if err := repository.JoinGame(test.id, ownerID+int64(test.capacity)); !errors.Is(err, ErrConflict) {
				t.Fatalf("overflow join returned %v", err)
			}

			stored, err := repository.GetGameByID(test.id)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(stored.JoinedUsers); got != test.capacity {
				t.Fatalf("stored roster size=%d want=%d: %+v", got, test.capacity, stored.JoinedUsers)
			}
			if test.legacy && (stored.Mode != "" || stored.MaxPlayers != 0) {
				t.Fatalf("legacy fields were unexpectedly materialized by join: %+v", stored)
			}
		})
	}
}

func TestMongoGameQueriesKeepLegacyAndValidCurrentCapacitiesIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoGameRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	now := time.Now().UTC()
	const viewerID int64 = 9000

	documents := []any{
		// Missing mode/maxplayers is a valid legacy duel with an effective capacity of two.
		bson.M{
			"id": int64(1201), "userid": viewerID, "ispublic": true, "isactive": true,
			"joinedusers": bson.A{viewerID, int64(9001)}, "state": "lobby", "createdat": now.Add(-time.Minute),
		},
		entites.Game{
			ID: 1202, UserID: viewerID, IsPublic: true, IsActive: true, Mode: "duel", MaxPlayers: 2,
			JoinedUsers: []int64{viewerID, 9002}, State: "lobby", CreatedAt: now,
		},
		entites.Game{
			ID: 1203, UserID: viewerID, IsPublic: true, IsActive: true, Mode: "team_2v2", MaxPlayers: 4,
			JoinedUsers: []int64{viewerID, 9003, 9004, 9005}, State: "lobby", CreatedAt: now,
		},
		entites.Game{
			ID: 1204, UserID: viewerID, IsPublic: true, IsActive: true, Mode: "team_4v4", MaxPlayers: 8,
			JoinedUsers: []int64{viewerID, 9006, 9007, 9008, 9009, 9010, 9011, 9012}, State: "lobby", CreatedAt: now,
		},
		entites.Game{
			ID: 1205, UserID: viewerID, IsPublic: true, IsActive: true, Mode: "open", MaxPlayers: 6,
			JoinedUsers: []int64{viewerID, 9013, 9014}, State: "lobby", CreatedAt: now,
		},
		// These records must be filtered out: membership exceeds capacity, capacity exceeds eight, or battle is closed.
		entites.Game{
			ID: 1291, UserID: viewerID, IsPublic: true, IsActive: true, Mode: "team_2v2", MaxPlayers: 4,
			JoinedUsers: []int64{viewerID, 9101, 9102, 9103, 9104}, State: "lobby", CreatedAt: now,
		},
		entites.Game{
			ID: 1292, UserID: viewerID, IsPublic: true, IsActive: true, Mode: "open", MaxPlayers: 9,
			JoinedUsers: []int64{viewerID}, State: "lobby", CreatedAt: now,
		},
		entites.Game{
			ID: 1293, UserID: viewerID, IsPublic: true, IsActive: false, Mode: "duel", MaxPlayers: 2,
			JoinedUsers: []int64{viewerID}, State: "lobby", CreatedAt: now,
		},
	}
	mustInsert(t, ctx, database.Collection("Game"), documents...)
	want := map[int64]struct{}{1201: {}, 1202: {}, 1203: {}, 1204: {}, 1205: {}}

	publicGames, err := repository.GetPublicBattle()
	if err != nil {
		t.Fatal(err)
	}
	assertMongoGameIDs(t, publicGames, want)

	myGames, err := repository.GetMyBattle(viewerID)
	if err != nil {
		t.Fatal(err)
	}
	assertMongoGameIDs(t, myGames, want)
}

func TestMongoGameExitCannotMutatePreparedRosterIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoGameRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	const gameID int64 = 1301
	const ownerID int64 = 13_001
	const guestID int64 = 13_002
	mustInsert(t, ctx, database.Collection("Game"), entites.Game{
		ID: gameID, UserID: ownerID, IsPublic: true, IsActive: true,
		Mode: "team_2v2", MaxPlayers: 4,
		JoinedUsers: []int64{ownerID, guestID, 13_003, 13_004},
		State:       "collecting_decks", MatchID: 91_301, CreatedAt: time.Now().UTC(),
	})

	if err := repository.LeaveGame(gameID, guestID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("prepared guest leave returned %v", err)
	}
	if err := repository.CloseGame(gameID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("prepared owner close returned %v", err)
	}

	stored, err := repository.GetGameByID(gameID)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.IsActive || len(stored.JoinedUsers) != 4 {
		t.Fatalf("prepared game changed after rejected exit: %+v", stored)
	}
}

func assertMongoGameIDs(t *testing.T, games []entites.Game, want map[int64]struct{}) {
	t.Helper()
	got := make(map[int64]struct{}, len(games))
	for _, game := range games {
		got[game.ID] = struct{}{}
	}
	if len(got) != len(want) {
		t.Fatalf("game IDs=%v want=%v", got, want)
	}
	for id := range want {
		if _, exists := got[id]; !exists {
			t.Fatalf("game %d was filtered out: got=%v", id, got)
		}
	}
}
