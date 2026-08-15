package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	avatardomain "github.com/akorwash/QuizBattle/domain/avatar"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoAvatarSaveReplaceGetDeleteAndIndexIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoAvatarRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	first, err := avatardomain.Process(42, repositoryAvatarPNG(t, color.NRGBA{R: 20, G: 80, B: 160, A: 255}), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(ctx, first); err != nil {
		t.Fatalf("save avatar: %v", err)
	}
	stored, err := repository.GetByUserID(ctx, 42)
	if err != nil {
		t.Fatalf("get avatar: %v", err)
	}
	if stored.UserID != first.UserID || stored.ETag != first.ETag || !bytes.Equal(stored.Data, first.Data) {
		t.Fatalf("stored avatar changed: %+v", stored)
	}

	second, err := avatardomain.Process(42, repositoryAvatarPNG(t, color.NRGBA{R: 180, G: 60, B: 30, A: 255}), first.UpdatedAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(ctx, second); err != nil {
		t.Fatalf("replace avatar: %v", err)
	}
	count, err := database.Collection(userAvatarCollection).CountDocuments(ctx, bson.M{"userId": int64(42)})
	if err != nil || count != 1 {
		t.Fatalf("replacement created duplicates: count=%d err=%v", count, err)
	}
	stored, err = repository.GetByUserID(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ETag != second.ETag || stored.ETag == first.ETag || !bytes.Equal(stored.Data, second.Data) || !stored.UpdatedAt.Equal(second.UpdatedAt) {
		t.Fatalf("replacement was not atomic: %+v", stored)
	}

	if err := repository.DeleteByUserID(ctx, 42); err != nil {
		t.Fatalf("delete avatar: %v", err)
	}
	if err := repository.DeleteByUserID(ctx, 42); err != nil {
		t.Fatalf("repeated delete should be idempotent: %v", err)
	}
	if _, err := repository.GetByUserID(ctx, 42); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted avatar returned %v", err)
	}

	cursor, err := database.Collection(userAvatarCollection).Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.Close(ctx)
	var indexes []bson.M
	if err := cursor.All(ctx, &indexes); err != nil {
		t.Fatal(err)
	}
	foundUniqueUser := false
	for _, index := range indexes {
		name, _ := index["name"].(string)
		if name == "uq_user_avatar_user" {
			foundUniqueUser = fmt.Sprint(index["unique"]) == "true"
		}
	}
	if !foundUniqueUser {
		t.Fatalf("unique avatar owner index missing: %#v", indexes)
	}
}

func TestMongoAvatarRejectsMalformedCanonicalRecordIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoAvatarRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	invalid := &avatardomain.Image{UserID: 42, ContentType: "image/png", Data: []byte("untrusted")}
	if err := repository.Save(ctx, invalid); !errors.Is(err, avatardomain.ErrInvalidImage) {
		t.Fatalf("invalid canonical record returned %v", err)
	}
	count, err := database.Collection(userAvatarCollection).CountDocuments(ctx, bson.M{})
	if err != nil || count != 0 {
		t.Fatalf("invalid record reached MongoDB: count=%d err=%v", count, err)
	}
}

func repositoryAvatarPNG(t *testing.T, fill color.NRGBA) []byte {
	t.Helper()
	source := image.NewNRGBA(image.Rect(0, 0, 12, 9))
	for y := 0; y < source.Bounds().Dy(); y++ {
		for x := 0; x < source.Bounds().Dx(); x++ {
			source.SetNRGBA(x, y, fill)
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, source); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
