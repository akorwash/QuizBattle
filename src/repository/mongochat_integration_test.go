package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	chatdomain "github.com/akorwash/QuizBattle/domain/chat"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoChatRetentionOrderingAndIndexesIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoChatRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clock := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	repository.now = func() time.Time {
		clock = clock.Add(time.Second)
		return clock
	}
	for index := 0; index < 130; index++ {
		message, err := chatdomain.NewMessage(42, "player_1", "لاعب أول", fmt.Sprintf("رسالة %03d", index))
		if err != nil {
			t.Fatal(err)
		}
		if err := repository.Save(ctx, message); err != nil {
			t.Fatalf("save message %d: %v", index, err)
		}
	}

	count, err := database.Collection(chatMessageCollection).CountDocuments(ctx, bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	if count != chatStoredLimit {
		t.Fatalf("retention kept %d messages; want %d", count, chatStoredLimit)
	}
	messages, err := repository.ListRecent(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 50 || messages[0].Text != "رسالة 080" || messages[49].Text != "رسالة 129" {
		t.Fatalf("wrong recent window: first=%q last=%q count=%d", messages[0].Text, messages[len(messages)-1].Text, len(messages))
	}
	for index := 1; index < len(messages); index++ {
		if messages[index].CreatedAt.Before(messages[index-1].CreatedAt) {
			t.Fatalf("history is not chronological at %d", index)
		}
	}

	cursor, err := database.Collection(chatMessageCollection).Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.Close(ctx)
	var indexes []bson.M
	if err := cursor.All(ctx, &indexes); err != nil {
		t.Fatal(err)
	}
	foundTTL := false
	foundRecent := false
	for _, index := range indexes {
		name, _ := index["name"].(string)
		if name == "ttl_chat_message_created" {
			foundTTL = true
			expiry, ok := index["expireAfterSeconds"]
			if !ok || fmt.Sprint(expiry) != "604800" {
				t.Fatalf("wrong chat TTL: %#v", index)
			}
		}
		if name == "ix_chat_message_created_id" {
			foundRecent = true
		}
	}
	if !foundTTL || !foundRecent {
		t.Fatalf("chat indexes missing: ttl=%v recent=%v indexes=%#v", foundTTL, foundRecent, indexes)
	}
}
