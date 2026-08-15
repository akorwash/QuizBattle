package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	avatardomain "github.com/akorwash/QuizBattle/domain/avatar"
	"github.com/akorwash/QuizBattle/repository"
)

type avatarRepositoryStub struct {
	saved       *avatardomain.Image
	stored      *avatardomain.Image
	saveErr     error
	getErr      error
	deleteErr   error
	deletedUser int64
}

func (stub *avatarRepositoryStub) Save(_ context.Context, avatar *avatardomain.Image) error {
	if stub.saveErr != nil {
		return stub.saveErr
	}
	copy := *avatar
	copy.Data = append([]byte(nil), avatar.Data...)
	stub.saved = &copy
	return nil
}

func (stub *avatarRepositoryStub) GetByUserID(_ context.Context, _ int64) (*avatardomain.Image, error) {
	if stub.getErr != nil {
		return nil, stub.getErr
	}
	if stub.stored == nil {
		return nil, repository.ErrNotFound
	}
	copy := *stub.stored
	copy.Data = append([]byte(nil), stub.stored.Data...)
	return &copy, nil
}

func (stub *avatarRepositoryStub) DeleteByUserID(_ context.Context, userID int64) error {
	stub.deletedUser = userID
	return stub.deleteErr
}

func TestAvatarServiceSavesCanonicalReplacement(t *testing.T) {
	repositoryStub := &avatarRepositoryStub{}
	avatarService := NewAvatarService(repositoryStub)
	now := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	avatarService.now = func() time.Time { return now }
	result, err := avatarService.SaveOrReplace(context.Background(), 42, avatarPNG(t, color.NRGBA{R: 10, G: 90, B: 160, A: 255}))
	if err != nil {
		t.Fatal(err)
	}
	if repositoryStub.saved == nil || repositoryStub.saved.UserID != 42 || repositoryStub.saved.ContentType != avatardomain.ContentType || repositoryStub.saved.Width != avatardomain.OutputSize {
		t.Fatalf("canonical avatar was not persisted: %+v", repositoryStub.saved)
	}
	if result.UserID != 42 || result.URL != "/api/v1/user/avatar/42" || result.ETag != `"`+repositoryStub.saved.ETag+`"` || result.UpdatedAt != now {
		t.Fatalf("unexpected upload metadata: %+v", result)
	}
}

func TestAvatarServiceReturnsIsolatedContentAndDeletesCurrentUser(t *testing.T) {
	processed, err := avatardomain.Process(42, avatarPNG(t, color.NRGBA{R: 180, G: 40, B: 20, A: 255}), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	repositoryStub := &avatarRepositoryStub{stored: processed}
	avatarService := NewAvatarService(repositoryStub)
	content, err := avatarService.Get(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if content.UserID != 42 || content.ContentType != avatardomain.ContentType || content.ETag != processed.ETag || !bytes.Equal(content.Data, processed.Data) {
		t.Fatalf("unexpected avatar content: %+v", content)
	}
	content.Data[0] ^= 0xff
	if bytes.Equal(content.Data, repositoryStub.stored.Data) {
		t.Fatal("service exposed the repository byte slice")
	}
	if err := avatarService.Delete(context.Background(), 42); err != nil || repositoryStub.deletedUser != 42 {
		t.Fatalf("delete did not use authenticated user: id=%d err=%v", repositoryStub.deletedUser, err)
	}
}

func TestAvatarServiceRejectsInvalidInputBeforePersistence(t *testing.T) {
	repositoryStub := &avatarRepositoryStub{}
	avatarService := NewAvatarService(repositoryStub)
	if _, err := avatarService.SaveOrReplace(context.Background(), 42, []byte("script")); !errors.Is(err, avatardomain.ErrUnsupportedFormat) {
		t.Fatalf("invalid image returned %v", err)
	}
	if repositoryStub.saved != nil {
		t.Fatal("invalid upload reached repository")
	}
	if _, err := avatarService.Get(context.Background(), 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid get returned %v", err)
	}
	if err := avatarService.Delete(context.Background(), 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid delete returned %v", err)
	}
}

func TestAvatarServicePropagatesRepositoryFailures(t *testing.T) {
	want := errors.New("database unavailable")
	repositoryStub := &avatarRepositoryStub{saveErr: want, getErr: want, deleteErr: want}
	avatarService := NewAvatarService(repositoryStub)
	if _, err := avatarService.SaveOrReplace(context.Background(), 42, avatarPNG(t, color.NRGBA{A: 255})); !errors.Is(err, want) {
		t.Fatalf("save failure was hidden: %v", err)
	}
	if _, err := avatarService.Get(context.Background(), 42); !errors.Is(err, want) {
		t.Fatalf("get failure was hidden: %v", err)
	}
	if err := avatarService.Delete(context.Background(), 42); !errors.Is(err, want) {
		t.Fatalf("delete failure was hidden: %v", err)
	}
}

func avatarPNG(t *testing.T, fill color.NRGBA) []byte {
	t.Helper()
	source := image.NewNRGBA(image.Rect(0, 0, 8, 6))
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
