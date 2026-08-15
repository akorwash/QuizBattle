package service

import (
	"context"
	"fmt"
	"time"

	avatardomain "github.com/akorwash/QuizBattle/domain/avatar"
	"github.com/akorwash/QuizBattle/resources"
)

// AvatarRepository is the persistence boundary for canonical user images.
// Implementations must replace by UserID atomically so every user owns at
// most one avatar document.
type AvatarRepository interface {
	Save(ctx context.Context, avatar *avatardomain.Image) error
	GetByUserID(ctx context.Context, userID int64) (*avatardomain.Image, error)
	DeleteByUserID(ctx context.Context, userID int64) error
}

// AvatarServices is the narrow contract consumed by the HTTP controller.
type AvatarServices interface {
	SaveOrReplace(ctx context.Context, userID int64, upload []byte) (*resources.UserAvatar, error)
	Get(ctx context.Context, userID int64) (*resources.UserAvatarContent, error)
	Delete(ctx context.Context, userID int64) error
}

type AvatarService struct {
	repository AvatarRepository
	now        func() time.Time
}

func NewAvatarService(repository AvatarRepository) *AvatarService {
	return &AvatarService{repository: repository, now: time.Now}
}

func (service *AvatarService) SaveOrReplace(ctx context.Context, userID int64, upload []byte) (*resources.UserAvatar, error) {
	if service == nil || service.repository == nil || userID <= 0 {
		return nil, fmt.Errorf("%w: invalid avatar request", ErrInvalidInput)
	}
	processed, err := avatardomain.Process(userID, upload, service.now().UTC())
	if err != nil {
		return nil, err
	}
	if err := service.repository.Save(ctx, processed); err != nil {
		return nil, err
	}
	result := avatarMetadata(processed)
	return &result, nil
}

func (service *AvatarService) Get(ctx context.Context, userID int64) (*resources.UserAvatarContent, error) {
	if service == nil || service.repository == nil || userID <= 0 {
		return nil, fmt.Errorf("%w: invalid user ID", ErrInvalidInput)
	}
	stored, err := service.repository.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := stored.ValidateStored(); err != nil {
		return nil, fmt.Errorf("stored avatar failed validation: %w", err)
	}
	return &resources.UserAvatarContent{
		UserID:      stored.UserID,
		Data:        append([]byte(nil), stored.Data...),
		ETag:        stored.ETag,
		ContentType: stored.ContentType,
		UpdatedAt:   stored.UpdatedAt,
	}, nil
}

func (service *AvatarService) Delete(ctx context.Context, userID int64) error {
	if service == nil || service.repository == nil || userID <= 0 {
		return fmt.Errorf("%w: invalid user ID", ErrInvalidInput)
	}
	return service.repository.DeleteByUserID(ctx, userID)
}

func avatarMetadata(image *avatardomain.Image) resources.UserAvatar {
	return resources.UserAvatar{
		UserID:      image.UserID,
		URL:         fmt.Sprintf("/api/v1/user/avatar/%d", image.UserID),
		ETag:        quotedETag(image.ETag),
		ContentType: image.ContentType,
		Width:       image.Width,
		Height:      image.Height,
		ByteSize:    image.ByteSize,
		UpdatedAt:   image.UpdatedAt,
	}
}

func quotedETag(value string) string {
	return `"` + value + `"`
}
