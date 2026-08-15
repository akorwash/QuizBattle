package repository

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func EnsureIndexes(ctx context.Context, database *mongo.Database) error {
	userIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetName("uq_users_id").SetUnique(true)},
		{Keys: bson.D{{Key: "username", Value: 1}}, Options: options.Index().SetName("uq_users_username").SetUnique(true)},
		{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetName("uq_users_email").SetUnique(true)},
		{Keys: bson.D{{Key: "mobilenumber", Value: 1}}, Options: options.Index().SetName("uq_users_mobile").SetUnique(true)},
	}
	if _, err := database.Collection("users").Indexes().CreateMany(ctx, userIndexes); err != nil {
		return fmt.Errorf("create user indexes: %w", err)
	}
	avatarIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}},
		Options: options.Index().SetName("uq_user_avatar_user").SetUnique(true),
	}
	if _, err := database.Collection(userAvatarCollection).Indexes().CreateOne(ctx, avatarIndex); err != nil {
		return fmt.Errorf("create user avatar index: %w", err)
	}
	gameIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetName("uq_games_id").SetUnique(true)},
		{Keys: bson.D{{Key: "userid", Value: 1}, {Key: "isactive", Value: 1}, {Key: "state", Value: 1}}, Options: options.Index().SetName("ix_games_owner_active_state")},
		{Keys: bson.D{{Key: "ispublic", Value: 1}, {Key: "isactive", Value: 1}}, Options: options.Index().SetName("ix_games_public_active")},
		{Keys: bson.D{{Key: "ispublic", Value: 1}, {Key: "isactive", Value: 1}, {Key: "createdat", Value: -1}}, Options: options.Index().SetName("ix_games_public_active_created")},
		{Keys: bson.D{{Key: "joinedusers", Value: 1}}, Options: options.Index().SetName("ix_games_joined_users")},
		{Keys: bson.D{{Key: "state", Value: 1}, {Key: "createdat", Value: -1}}, Options: options.Index().SetName("ix_games_state_created")},
	}
	if _, err := database.Collection("Game").Indexes().CreateMany(ctx, gameIndexes); err != nil {
		return fmt.Errorf("create game indexes: %w", err)
	}
	questionIndex := mongo.IndexModel{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetName("uq_questions_id").SetUnique(true)}
	if _, err := database.Collection("Question").Indexes().CreateOne(ctx, questionIndex); err != nil {
		return fmt.Errorf("create question indexes: %w", err)
	}
	questionBankIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetName("uq_question_bank_id").SetUnique(true)},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "category", Value: 1}, {Key: "difficulty", Value: 1}}, Options: options.Index().SetName("ix_question_bank_pool")},
		{Keys: bson.D{{Key: "contentHash", Value: 1}}, Options: options.Index().SetName("ix_question_bank_hash")},
	}
	if _, err := database.Collection(questionBankCollection).Indexes().CreateMany(ctx, questionBankIndexes); err != nil {
		return fmt.Errorf("create question bank indexes: %w", err)
	}
	economyIndexes := map[string][]mongo.IndexModel{
		walletCollection: {
			{Keys: bson.D{{Key: "userId", Value: 1}}, Options: options.Index().SetName("uq_wallet_user").SetUnique(true)},
		},
		cardCollection: {
			{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetName("uq_cards_id").SetUnique(true)},
			{Keys: bson.D{{Key: "ownerId", Value: 1}, {Key: "status", Value: 1}}, Options: options.Index().SetName("ix_cards_owner_status")},
			{Keys: bson.D{{Key: "questionId", Value: 1}}, Options: options.Index().SetName("ix_cards_question")},
		},
		ledgerCollection: {
			{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetName("uq_ledger_id").SetUnique(true)},
			{Keys: bson.D{{Key: "idempotencyKey", Value: 1}, {Key: "entryPart", Value: 1}}, Options: options.Index().SetName("uq_ledger_command_part").SetUnique(true)},
			{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}}, Options: options.Index().SetName("ix_ledger_user_created")},
		},
		listingCollection: {
			{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetName("uq_listing_id").SetUnique(true)},
			{Keys: bson.D{{Key: "commandId", Value: 1}}, Options: options.Index().SetName("uq_listing_command").SetUnique(true)},
			{Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}, Options: options.Index().SetName("ix_listing_status_created")},
			{Keys: bson.D{{Key: "status", Value: 1}, {Key: "expiresAt", Value: 1}, {Key: "id", Value: 1}}, Options: options.Index().SetName("ix_listing_status_expires")},
			{Keys: bson.D{{Key: "cardId", Value: 1}}, Options: options.Index().SetName("uq_listing_active_card").SetUnique(true).SetPartialFilterExpression(bson.M{"status": "active"})},
		},
		tradeCollection: {
			{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetName("uq_trade_id").SetUnique(true)},
			{Keys: bson.D{{Key: "commandId", Value: 1}}, Options: options.Index().SetName("uq_trade_command").SetUnique(true)},
			{Keys: bson.D{{Key: "senderId", Value: 1}, {Key: "status", Value: 1}}, Options: options.Index().SetName("ix_trade_sender_status")},
			{Keys: bson.D{{Key: "receiverId", Value: 1}, {Key: "status", Value: 1}}, Options: options.Index().SetName("ix_trade_receiver_status")},
			{Keys: bson.D{{Key: "status", Value: 1}, {Key: "expiresAt", Value: 1}, {Key: "id", Value: 1}}, Options: options.Index().SetName("ix_trade_status_expires")},
		},
		rewardQuotaCollection: {
			{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "day", Value: 1}}, Options: options.Index().SetName("ix_reward_quota_user_day")},
			{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetName("ttl_reward_quota_expires").SetExpireAfterSeconds(0)},
		},
		matchCollection: {
			{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetName("uq_match_id").SetUnique(true)},
			{Keys: bson.D{{Key: "gameId", Value: 1}}, Options: options.Index().SetName("uq_match_game").SetUnique(true)},
			{Keys: bson.D{{Key: "players.userId", Value: 1}, {Key: "status", Value: 1}}, Options: options.Index().SetName("ix_match_player_status")},
		},
	}
	for collection, indexes := range economyIndexes {
		if _, err := database.Collection(collection).Indexes().CreateMany(ctx, indexes); err != nil {
			return fmt.Errorf("create %s indexes: %w", collection, err)
		}
	}
	sessionIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tokenid", Value: 1}}, Options: options.Index().SetName("uq_session_revocation_token").SetUnique(true)},
		{Keys: bson.D{{Key: "expiresat", Value: 1}}, Options: options.Index().SetName("ttl_session_revocation").SetExpireAfterSeconds(0)},
	}
	if _, err := database.Collection(sessionRevocationCollection).Indexes().CreateMany(ctx, sessionIndexes); err != nil {
		return fmt.Errorf("create session revocation indexes: %w", err)
	}
	chatIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetName("uq_chat_message_id").SetUnique(true)},
		{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetName("ttl_chat_message_created").SetExpireAfterSeconds(7 * 24 * 60 * 60)},
		{Keys: bson.D{{Key: "createdAt", Value: -1}, {Key: "id", Value: -1}}, Options: options.Index().SetName("ix_chat_message_created_id")},
	}
	if _, err := database.Collection(chatMessageCollection).Indexes().CreateMany(ctx, chatIndexes); err != nil {
		return fmt.Errorf("create chat message indexes: %w", err)
	}
	return nil
}
