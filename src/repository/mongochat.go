package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	chatdomain "github.com/akorwash/QuizBattle/domain/chat"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	chatMessageCollection = "ChatMessage"
	chatStoredLimit       = int64(100)
	chatOperationTimeout  = 5 * time.Second
)

type MongoChatRepository struct {
	collection *mongo.Collection
	now        func() time.Time
	pruneGate  chan struct{}
}

func NewMongoChatRepository(database *mongo.Database) *MongoChatRepository {
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	return &MongoChatRepository{collection: database.Collection(chatMessageCollection), now: time.Now, pruneGate: gate}
}

// Save persists a server-authored message and prunes everything older than the
// newest 100 records. The context-aware gate bounds queueing behind a slow
// datastore. Once the insert commits, retention maintenance is best effort so
// clients never see a false failure for a message that will appear on reload.
func (repository *MongoChatRepository) Save(ctx context.Context, message *chatdomain.Message) error {
	if message == nil || message.Validate() != nil {
		return fmt.Errorf("save chat message: %w", chatdomain.ErrInvalidMessage)
	}
	operation, cancel := boundedChatContext(ctx)
	defer cancel()
	select {
	case <-repository.pruneGate:
		defer func() { repository.pruneGate <- struct{}{} }()
	case <-operation.Done():
		return fmt.Errorf("save chat message: %w", operation.Err())
	}
	id, err := newID()
	if err != nil {
		return fmt.Errorf("save chat message: %w", err)
	}
	// Persistence owns both values so transport callers cannot forge chronology
	// or bypass retention with an arbitrary identifier/timestamp.
	message.ID = id
	message.CreatedAt = repository.now().UTC()
	if _, err := repository.collection.InsertOne(operation, message); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrConflict
		}
		return fmt.Errorf("save chat message: %w", err)
	}
	if err := repository.prune(operation); err != nil {
		slog.Warn("chat retention maintenance deferred", "error", err)
	}
	return nil
}

// ListRecent returns at most limit messages in chronological order even though
// Mongo reads the newest records first for an efficient bounded query.
func (repository *MongoChatRepository) ListRecent(ctx context.Context, limit int64) ([]chatdomain.Message, error) {
	if limit <= 0 || limit > chatStoredLimit {
		return nil, fmt.Errorf("list chat messages: invalid limit")
	}
	operation, cancel := boundedChatContext(ctx)
	defer cancel()
	cursor, err := repository.collection.Find(
		operation,
		bson.M{},
		options.Find().SetLimit(limit).SetSort(bson.D{{Key: "createdAt", Value: -1}, {Key: "id", Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	defer cursor.Close(operation)
	messages := make([]chatdomain.Message, 0, limit)
	if err := cursor.All(operation, &messages); err != nil {
		return nil, fmt.Errorf("decode chat messages: %w", err)
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func (repository *MongoChatRepository) prune(ctx context.Context) error {
	var cutoff struct {
		ID        int64     `bson:"id"`
		CreatedAt time.Time `bson:"createdAt"`
	}
	err := repository.collection.FindOne(
		ctx,
		bson.M{},
		options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}, {Key: "id", Value: -1}}).SetSkip(chatStoredLimit),
	).Decode(&cutoff)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find chat retention cutoff: %w", err)
	}
	filter := bson.M{"$or": bson.A{
		bson.M{"createdAt": bson.M{"$lt": cutoff.CreatedAt}},
		bson.M{"createdAt": cutoff.CreatedAt, "id": bson.M{"$lte": cutoff.ID}},
	}}
	if _, err := repository.collection.DeleteMany(ctx, filter); err != nil {
		return fmt.Errorf("prune chat messages: %w", err)
	}
	return nil
}

func boundedChatContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, chatOperationTimeout)
}
