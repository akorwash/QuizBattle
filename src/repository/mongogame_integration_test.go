package repository

import (
	"context"
	"testing"
	"time"

	"github.com/akorwash/QuizBattle/datastore/entites"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestCountActiveGameExcludesTerminalBattleHistoryIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoGameRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	const ownerID int64 = 71
	games := []any{
		entites.Game{ID: 1, UserID: ownerID, IsActive: true, State: "lobby"},
		entites.Game{ID: 2, UserID: ownerID, IsActive: true, State: "collecting_decks"},
		entites.Game{ID: 3, UserID: ownerID, IsActive: true, State: "active"},
		entites.Game{ID: 8, UserID: ownerID, IsActive: true, State: ""},
		bson.M{"id": int64(9), "userid": ownerID, "isactive": true},
		entites.Game{ID: 4, UserID: ownerID, IsActive: true, State: "completed"},
		entites.Game{ID: 5, UserID: ownerID, IsActive: true, State: "forfeited"},
		entites.Game{ID: 6, UserID: ownerID, IsActive: false, State: "lobby"},
		entites.Game{ID: 7, UserID: ownerID + 1, IsActive: true, State: "lobby"},
	}
	if _, err := database.Collection("Game").InsertMany(ctx, games); err != nil {
		t.Fatal(err)
	}

	count, err := repository.CountActiveGame(ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("terminal or unrelated battles consumed the quota, or a legacy open battle was missed: got %d want 5", count)
	}

	if _, err := database.Collection("Game").UpdateOne(ctx, bson.M{"id": int64(2)}, bson.M{"$set": bson.M{"state": "completed"}}); err != nil {
		t.Fatal(err)
	}
	count, err = repository.CountActiveGame(ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("newly completed battle still consumed the quota: got %d want 4", count)
	}
}
