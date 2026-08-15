package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/akorwash/QuizBattle/domain/economy"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
	"github.com/akorwash/QuizBattle/domain/question"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

const (
	walletCollection      = "Wallets"
	cardCollection        = "Cards"
	ledgerCollection      = "EconomyLedger"
	listingCollection     = "MarketListings"
	tradeCollection       = "TradeOffers"
	rewardQuotaCollection = "RewardQuotas"
)

type MongoEconomyRepository struct {
	database     *mongo.Database
	wallets      *mongo.Collection
	cards        *mongo.Collection
	ledger       *mongo.Collection
	listings     *mongo.Collection
	trades       *mongo.Collection
	rewardQuotas *mongo.Collection
}

func NewMongoEconomyRepository(database *mongo.Database) *MongoEconomyRepository {
	return &MongoEconomyRepository{
		database: database,
		wallets:  database.Collection(walletCollection), cards: database.Collection(cardCollection),
		ledger: database.Collection(ledgerCollection), listings: database.Collection(listingCollection),
		trades: database.Collection(tradeCollection), rewardQuotas: database.Collection(rewardQuotaCollection),
	}
}

func (repository *MongoEconomyRepository) EnsureStarter(ctx context.Context, userID int64, questions []question.Question) error {
	if userID <= 0 || len(questions) != economy.StarterCards {
		return economy.ErrInvalidEconomyState
	}
	cardIDs := make([]int64, len(questions))
	for index := range cardIDs {
		id, err := NewID()
		if err != nil {
			return err
		}
		cardIDs[index] = id
	}
	ledgerID, err := NewID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return repository.withTransaction(ctx, func(tx context.Context) error {
		var wallet economy.Wallet
		findError := repository.wallets.FindOne(tx, bson.M{"userId": userID}).Decode(&wallet)
		if findError != nil && !errors.Is(findError, mongo.ErrNoDocuments) {
			return fmt.Errorf("find starter wallet: %w", findError)
		}
		walletCreated := errors.Is(findError, mongo.ErrNoDocuments)
		if walletCreated {
			wallet = economy.Wallet{UserID: userID, Balance: economy.StarterBalance, Version: 1, CreatedAt: now, UpdatedAt: now}
			if _, err := repository.wallets.InsertOne(tx, wallet); err != nil {
				return fmt.Errorf("create starter wallet: %w", err)
			}
			entry := economy.LedgerEntry{
				ID: ledgerID, UserID: userID, CoinDelta: economy.StarterBalance,
				BalanceAfter: economy.StarterBalance, Reason: "starter_grant",
				ReferenceType: "account", ReferenceID: fmt.Sprint(userID),
				IdempotencyKey: fmt.Sprintf("starter:%d", userID), EntryPart: "coins", CreatedAt: now,
			}
			if _, err := repository.ledger.InsertOne(tx, entry); err != nil {
				return fmt.Errorf("record starter grant: %w", err)
			}
		}

		for index, item := range questions {
			card := economy.Card{
				ID: cardIDs[index], OwnerID: userID, QuestionID: item.ID, Edition: 1,
				Rarity: economy.RarityForDifficulty(string(item.Difficulty)), Power: 1,
				Status: economy.CardAvailable, Version: 1, CreatedAt: now, UpdatedAt: now,
			}
			_, err := repository.cards.UpdateOne(
				tx,
				bson.M{"ownerId": userID, "questionId": item.ID, "edition": 1},
				bson.M{"$setOnInsert": card},
				options.UpdateOne().SetUpsert(true),
			)
			if err != nil {
				return fmt.Errorf("create starter card %s: %w", item.ID, err)
			}
		}
		return nil
	})
}

func (repository *MongoEconomyRepository) GetWallet(ctx context.Context, userID int64) (*economy.Wallet, error) {
	var wallet economy.Wallet
	if err := repository.wallets.FindOne(ctx, bson.M{"userId": userID}).Decode(&wallet); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get wallet: %w", err)
	}
	return &wallet, nil
}

func (repository *MongoEconomyRepository) ListCards(ctx context.Context, userID int64) ([]economy.Card, error) {
	cursor, err := repository.cards.Find(
		ctx,
		bson.M{"ownerId": userID},
		options.Find().SetSort(bson.D{{Key: "rarity", Value: -1}, {Key: "createdAt", Value: 1}}).SetLimit(500),
	)
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	defer cursor.Close(ctx)
	result := make([]economy.Card, 0)
	if err := cursor.All(ctx, &result); err != nil {
		return nil, fmt.Errorf("decode cards: %w", err)
	}
	return result, nil
}

func (repository *MongoEconomyRepository) GetCardsByIDs(ctx context.Context, ids []int64) (map[int64]economy.Card, error) {
	result := make(map[int64]economy.Card, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	cursor, err := repository.cards.Find(ctx, bson.M{"id": bson.M{"$in": ids}})
	if err != nil {
		return nil, fmt.Errorf("find cards: %w", err)
	}
	defer cursor.Close(ctx)
	var cards []economy.Card
	if err := cursor.All(ctx, &cards); err != nil {
		return nil, fmt.Errorf("decode cards: %w", err)
	}
	for _, card := range cards {
		result[card.ID] = card
	}
	return result, nil
}

func (repository *MongoEconomyRepository) LockCardsForMatch(ctx context.Context, userID int64, cardIDs []int64, matchID int64) error {
	if userID <= 0 || matchID <= 0 || len(cardIDs) != 5 || duplicateInt64(cardIDs) {
		return economy.ErrInvalidEconomyState
	}
	lockRef := fmt.Sprintf("match:%d", matchID)
	return repository.withTransaction(ctx, func(tx context.Context) error {
		result, err := repository.cards.UpdateMany(
			tx,
			bson.M{"id": bson.M{"$in": cardIDs}, "ownerId": userID, "status": economy.CardAvailable, "lockRef": bson.M{"$in": []any{"", nil}}},
			bson.M{"$set": bson.M{"status": economy.CardMatchLocked, "lockRef": lockRef, "updatedAt": time.Now().UTC()}, "$inc": bson.M{"version": 1}},
		)
		if err != nil {
			return fmt.Errorf("lock match cards: %w", err)
		}
		if result.ModifiedCount != int64(len(cardIDs)) {
			return economy.ErrCardUnavailable
		}
		return nil
	})
}

func (repository *MongoEconomyRepository) UnlockMatchCards(ctx context.Context, matchID int64) error {
	if matchID <= 0 {
		return economy.ErrInvalidEconomyState
	}
	_, err := repository.cards.UpdateMany(
		ctx,
		bson.M{"status": economy.CardMatchLocked, "lockRef": fmt.Sprintf("match:%d", matchID)},
		bson.M{"$set": bson.M{"status": economy.CardAvailable, "lockRef": "", "updatedAt": time.Now().UTC()}, "$inc": bson.M{"version": 1}},
	)
	if err != nil {
		return fmt.Errorf("unlock match cards: %w", err)
	}
	return nil
}

func (repository *MongoEconomyRepository) SettleMatchRewards(ctx context.Context, matchID int64) error {
	if matchID <= 0 {
		return economy.ErrInvalidEconomyState
	}
	now := time.Now().UTC()
	operation := func(tx context.Context) error {
		matches := repository.database.Collection(matchCollection)
		var aggregate matchdomain.Aggregate
		if err := matches.FindOne(tx, bson.M{"id": matchID}).Decode(&aggregate); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return ErrNotFound
			}
			return fmt.Errorf("find match settlement: %w", err)
		}
		if aggregate.RewardsSettled {
			return nil
		}
		candidates, err := aggregate.RewardCandidates()
		if err != nil {
			return fmt.Errorf("plan match rewards: %w", err)
		}
		humanPlayers := make(map[int64]matchdomain.Player, len(candidates))
		expectedLockedCards := 0
		for _, player := range aggregate.Players {
			if player.IsBot() {
				continue
			}
			if len(player.Deck) != 0 && len(player.Deck) != matchdomain.DeckSize {
				return economy.ErrInvalidEconomyState
			}
			humanPlayers[player.UserID] = player
			expectedLockedCards += len(player.Deck)
		}
		if len(humanPlayers) != len(candidates) || aggregate.Status == matchdomain.StatusCompleted && expectedLockedCards != matchdomain.DeckSize*len(humanPlayers) {
			return economy.ErrInvalidEconomyState
		}

		var activeQuestions []question.Question
		receipts := make([]matchdomain.RewardReceipt, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate.UserID <= 0 || candidate.Coins < 0 {
				return economy.ErrInvalidEconomyState
			}
			receipt := matchdomain.RewardReceipt{
				UserID: candidate.UserID, PolicyVersion: aggregate.RewardPolicyVersion,
				Source: candidate.Source, BotStrategy: candidate.BotStrategy,
				Outcome: candidate.Outcome, Status: matchdomain.RewardStatusGranted,
				CoinsGranted: candidate.Coins, SettledAt: now,
			}
			grantCard := candidate.GrantsCard
			quotaLimit := 0
			quotaReason := ""
			if aggregate.RewardPolicyVersion == matchdomain.RewardPolicyV1 {
				switch {
				case candidate.Source == matchdomain.RewardSourceBot && grantCard:
					quotaLimit = matchdomain.BotDailyRewardLimit
					quotaReason = matchdomain.RewardReasonBotDailyCap
				case candidate.Source == matchdomain.RewardSourcePVP && (receipt.CoinsGranted > 0 || grantCard):
					quotaLimit = matchdomain.PVPDailyRewardLimit
					quotaReason = matchdomain.RewardReasonPVPDailyCap
				}
			}
			if quotaLimit > 0 {
				allowed, reserveErr := repository.reserveDailyReward(
					tx, candidate.UserID, candidate.Source, aggregate.CompletedAt, now, quotaLimit,
				)
				if reserveErr != nil {
					return reserveErr
				}
				if !allowed {
					receipt.Status = matchdomain.RewardStatusCapped
					receipt.Reason = quotaReason
					receipt.CoinsGranted = 0
					grantCard = false
				}
			}
			if receipt.CoinsGranted == 0 && !grantCard && receipt.Status == matchdomain.RewardStatusGranted &&
				(candidate.Outcome == matchdomain.RewardOutcomeLoss || candidate.Outcome == matchdomain.RewardOutcomeDraw || candidate.Outcome == matchdomain.RewardOutcomeForfeit) {
				receipt.Status = matchdomain.RewardStatusIneligible
				receipt.Reason = string(candidate.Outcome)
			}

			var wallet economy.Wallet
			if receipt.CoinsGranted > 0 || grantCard {
				if err := repository.wallets.FindOne(tx, bson.M{"userId": candidate.UserID}).Decode(&wallet); err != nil {
					return fmt.Errorf("find reward wallet: %w", err)
				}
			}
			if receipt.CoinsGranted > 0 {
				oldVersion := wallet.Version
				wallet.Balance += receipt.CoinsGranted
				wallet.Version++
				wallet.UpdatedAt = now
				result, replaceErr := repository.wallets.ReplaceOne(tx, bson.M{"userId": candidate.UserID, "version": oldVersion}, wallet)
				if replaceErr != nil {
					return fmt.Errorf("credit match reward: %w", replaceErr)
				}
				if err := matched(result); err != nil {
					return err
				}
				entryPart := fmt.Sprintf("player:%d", candidate.UserID)
				idempotencyKey := fmt.Sprintf("match:%d:reward", matchID)
				if aggregate.RewardPolicyVersion == matchdomain.RewardPolicyV1 {
					entryPart += ":coins"
					idempotencyKey += ":" + matchdomain.RewardPolicyV1
				}
				if err := repository.insertRewardLedger(tx, economy.LedgerEntry{
					UserID: candidate.UserID, CoinDelta: receipt.CoinsGranted, BalanceAfter: wallet.Balance,
					Reason: "match_reward", ReferenceType: "match", ReferenceID: fmt.Sprint(matchID),
					IdempotencyKey: idempotencyKey, EntryPart: entryPart, CreatedAt: now,
				}); err != nil {
					return err
				}
			}
			if grantCard {
				if len(activeQuestions) == 0 {
					activeQuestions, err = repository.activeRewardQuestions(tx)
					if err != nil {
						return err
					}
				}
				card, summary, mintErr := repository.mintRewardCard(tx, &aggregate, candidate, activeQuestions, now)
				if mintErr != nil {
					return mintErr
				}
				receipt.Card = summary
				if err := repository.insertRewardLedger(tx, economy.LedgerEntry{
					UserID: candidate.UserID, BalanceAfter: wallet.Balance, CardID: card.ID,
					NewOwner: candidate.UserID, Reason: "match_card_reward",
					ReferenceType: "match", ReferenceID: fmt.Sprint(matchID),
					IdempotencyKey: fmt.Sprintf("match:%d:reward:%s", matchID, matchdomain.RewardPolicyV1),
					EntryPart:      fmt.Sprintf("player:%d:card", candidate.UserID), CreatedAt: now,
				}); err != nil {
					return err
				}
			}
			if aggregate.RewardPolicyVersion == matchdomain.RewardPolicyV1 {
				receipts = append(receipts, receipt)
			}
		}

		lockRef := fmt.Sprintf("match:%d", matchID)
		cardUpdate := bson.M{
			"$set": bson.M{"status": economy.CardAvailable, "lockRef": "", "updatedAt": now},
			"$inc": bson.M{"version": 1},
		}
		if aggregate.Status == matchdomain.StatusCompleted {
			cardUpdate["$inc"].(bson.M)["plays"] = 1
		}
		releasedCards, err := repository.cards.UpdateMany(
			tx,
			bson.M{"status": economy.CardMatchLocked, "lockRef": lockRef},
			cardUpdate,
		)
		if err != nil {
			return fmt.Errorf("release settled match cards: %w", err)
		}
		if err := matchedAndModified(releasedCards, int64(expectedLockedCards)); err != nil {
			return fmt.Errorf("release settled match cards: %w", economy.ErrInvalidEconomyState)
		}
		if aggregate.Status == matchdomain.StatusCompleted && !aggregate.IsTie {
			winnerIDs := humanWinnerIDs(candidates)
			if len(winnerIDs) == 0 {
				// A bot may be the sole winner and has no collectible mastery.
				if aggregate.EffectiveMode() != matchdomain.ModeBot {
					return economy.ErrInvalidEconomyState
				}
			}
			winnerSet := make(map[int64]struct{}, len(winnerIDs))
			for _, winnerID := range winnerIDs {
				if winnerID <= 0 {
					return economy.ErrInvalidEconomyState
				}
				winnerSet[winnerID] = struct{}{}
			}
			winnerCardIDs := make([]int64, 0, matchdomain.DeckSize*len(winnerSet))
			for _, player := range aggregate.Players {
				if _, winner := winnerSet[player.UserID]; !winner {
					continue
				}
				for _, card := range player.Deck {
					winnerCardIDs = append(winnerCardIDs, card.ID)
				}
			}
			if len(winnerCardIDs) != matchdomain.DeckSize*len(winnerSet) {
				return economy.ErrInvalidEconomyState
			}
			if len(winnerCardIDs) > 0 {
				winningCards, updateErr := repository.cards.UpdateMany(
					tx,
					bson.M{"id": bson.M{"$in": winnerCardIDs}, "ownerId": bson.M{"$in": winnerIDs}, "status": economy.CardAvailable, "lockRef": ""},
					bson.M{"$inc": bson.M{"wins": 1, "version": 1}, "$set": bson.M{"updatedAt": now}},
				)
				if updateErr != nil {
					return fmt.Errorf("record winning card mastery: %w", updateErr)
				}
				if err := matchedAndModified(winningCards, int64(len(winnerCardIDs))); err != nil {
					return fmt.Errorf("record winning card mastery: %w", economy.ErrInvalidEconomyState)
				}
			}
		}
		settlement := bson.M{"rewardsSettled": true}
		if aggregate.RewardPolicyVersion == matchdomain.RewardPolicyV1 {
			settlement["rewardReceipts"] = receipts
		}
		result, err := matches.UpdateOne(tx, bson.M{"id": matchID, "rewardsSettled": false}, bson.M{"$set": settlement})
		if err != nil {
			return fmt.Errorf("mark rewards settled: %w", err)
		}
		if result.MatchedCount != 1 {
			return ErrConflict
		}
		return nil
	}
	const settlementAttempts = 4
	for attempt := 0; attempt < settlementAttempts; attempt++ {
		err := repository.withTransaction(ctx, operation)
		if err == nil {
			return nil
		}
		// Two first rewards for the same user/day can race while creating the
		// deterministic quota document. Retrying the complete transaction turns
		// that duplicate-key/write-version race into a normal quota increment.
		if !mongo.IsDuplicateKeyError(err) && !errors.Is(err, ErrConflict) {
			return err
		}
	}
	return ErrConflict
}

type rewardQuota struct {
	ID        string    `bson:"_id"`
	UserID    int64     `bson:"userId"`
	Day       string    `bson:"day"`
	Used      int       `bson:"used"`
	Version   int64     `bson:"version"`
	UpdatedAt time.Time `bson:"updatedAt"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

func humanWinnerIDs(candidates []matchdomain.RewardCandidate) []int64 {
	result := make([]int64, 0)
	for _, candidate := range candidates {
		if candidate.Outcome == matchdomain.RewardOutcomeChampion || candidate.Outcome == matchdomain.RewardOutcomeTeamWinner {
			result = append(result, candidate.UserID)
		}
	}
	return result
}

func (repository *MongoEconomyRepository) reserveDailyReward(
	ctx context.Context,
	userID int64,
	source matchdomain.RewardSource,
	completedAt, now time.Time,
	limit int,
) (bool, error) {
	if userID <= 0 || completedAt.IsZero() || limit <= 0 ||
		(source != matchdomain.RewardSourceBot && source != matchdomain.RewardSourcePVP) {
		return false, economy.ErrInvalidEconomyState
	}
	completedAt = completedAt.UTC()
	day := completedAt.Format("2006-01-02")
	key := fmt.Sprintf("%s:%s:%d:%s", matchdomain.RewardPolicyV1, source, userID, day)
	var quota rewardQuota
	err := repository.rewardQuotas.FindOne(ctx, bson.M{"_id": key}).Decode(&quota)
	if errors.Is(err, mongo.ErrNoDocuments) {
		start := time.Date(completedAt.Year(), completedAt.Month(), completedAt.Day(), 0, 0, 0, 0, time.UTC)
		expiresAt := start.Add(8 * 24 * time.Hour)
		if minimumExpiry := now.Add(24 * time.Hour); expiresAt.Before(minimumExpiry) {
			expiresAt = minimumExpiry
		}
		quota = rewardQuota{ID: key, UserID: userID, Day: day, Used: 1, Version: 1, UpdatedAt: now, ExpiresAt: expiresAt}
		if _, insertErr := repository.rewardQuotas.InsertOne(ctx, quota); insertErr != nil {
			return false, fmt.Errorf("create %s reward quota: %w", source, insertErr)
		}
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("find %s reward quota: %w", source, err)
	}
	if quota.Used >= limit {
		return false, nil
	}
	result, err := repository.rewardQuotas.UpdateOne(
		ctx,
		bson.M{"_id": key, "version": quota.Version, "used": bson.M{"$lt": limit}},
		bson.M{"$inc": bson.M{"used": 1, "version": 1}, "$set": bson.M{"updatedAt": now}},
	)
	if err != nil {
		return false, fmt.Errorf("reserve %s reward quota: %w", source, err)
	}
	if result.MatchedCount != 1 {
		return false, ErrConflict
	}
	return true, nil
}

func (repository *MongoEconomyRepository) activeRewardQuestions(ctx context.Context) ([]question.Question, error) {
	cursor, err := repository.database.Collection(questionBankCollection).Find(
		ctx,
		bson.M{"status": question.StatusActive},
		options.Find().SetSort(bson.D{{Key: "id", Value: 1}}).SetLimit(5000),
	)
	if err != nil {
		return nil, fmt.Errorf("find reward questions: %w", err)
	}
	defer cursor.Close(ctx)
	var items []question.Question
	if err := cursor.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("decode reward questions: %w", err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("reward question pool is empty: %w", economy.ErrInvalidEconomyState)
	}
	return items, nil
}

func (repository *MongoEconomyRepository) mintRewardCard(
	ctx context.Context,
	aggregate *matchdomain.Aggregate,
	candidate matchdomain.RewardCandidate,
	questions []question.Question,
	now time.Time,
) (economy.Card, *matchdomain.RewardCard, error) {
	if aggregate == nil || aggregate.RewardPolicyVersion != matchdomain.RewardPolicyV1 || candidate.UserID <= 0 || len(questions) == 0 {
		return economy.Card{}, nil, economy.ErrInvalidEconomyState
	}
	desiredRarity, err := matchdomain.RewardRarity(aggregate.RewardNonce, aggregate.ID, candidate)
	if err != nil {
		return economy.Card{}, nil, err
	}
	ownedEditions, err := repository.rewardOwnedEditions(ctx, candidate.UserID)
	if err != nil {
		return economy.Card{}, nil, err
	}
	selected, err := selectRewardQuestion(questions, ownedEditions, desiredRarity, aggregate.RewardNonce, aggregate.ID, candidate.UserID)
	if err != nil {
		return economy.Card{}, nil, err
	}
	edition := 1
	if previous := ownedEditions[selected.ID]; previous > 0 {
		edition = previous + 1
	}
	cardID, err := NewID()
	if err != nil {
		return economy.Card{}, nil, err
	}
	rarity := economy.RarityForDifficulty(string(selected.Difficulty))
	card := economy.Card{
		ID: cardID, OwnerID: candidate.UserID, QuestionID: selected.ID, Edition: edition,
		Rarity: rarity, Power: 1, Status: economy.CardAvailable, Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if _, err := repository.cards.InsertOne(ctx, card); err != nil {
		return economy.Card{}, nil, fmt.Errorf("mint match reward card: %w", err)
	}
	summary := &matchdomain.RewardCard{
		ID: card.ID, QuestionID: card.QuestionID, Category: selected.Category,
		Difficulty: string(selected.Difficulty), Rarity: card.Rarity, Power: card.Power, Edition: card.Edition,
	}
	return card, summary, nil
}

func (repository *MongoEconomyRepository) rewardOwnedEditions(ctx context.Context, userID int64) (map[string]int, error) {
	cursor, err := repository.cards.Find(ctx, bson.M{"ownerId": userID}, options.Find().SetProjection(bson.M{"questionId": 1, "edition": 1}))
	if err != nil {
		return nil, fmt.Errorf("find reward owner cards: %w", err)
	}
	defer cursor.Close(ctx)
	var cards []economy.Card
	if err := cursor.All(ctx, &cards); err != nil {
		return nil, fmt.Errorf("decode reward owner cards: %w", err)
	}
	result := make(map[string]int, len(cards))
	for _, card := range cards {
		edition := card.Edition
		if edition < 1 {
			edition = 1
		}
		if edition > result[card.QuestionID] {
			result[card.QuestionID] = edition
		}
	}
	return result, nil
}

func selectRewardQuestion(
	questions []question.Question,
	ownedEditions map[string]int,
	desiredRarity string,
	nonce []byte,
	matchID, userID int64,
) (question.Question, error) {
	desiredDifficulty := map[string]question.Difficulty{
		"common": question.DifficultyEasy,
		"rare":   question.DifficultyMedium,
		"epic":   question.DifficultyHard,
	}[desiredRarity]
	if desiredDifficulty == "" {
		return question.Question{}, matchdomain.ErrInvalidRewardPolicy
	}
	groups := make([][]question.Question, 0, 3)
	uniqueDesired := make([]question.Question, 0)
	uniqueAny := make([]question.Question, 0)
	duplicateDesired := make([]question.Question, 0)
	for _, item := range questions {
		_, owned := ownedEditions[item.ID]
		if !owned {
			uniqueAny = append(uniqueAny, item)
			if item.Difficulty == desiredDifficulty {
				uniqueDesired = append(uniqueDesired, item)
			}
		} else if item.Difficulty == desiredDifficulty {
			duplicateDesired = append(duplicateDesired, item)
		}
	}
	groups = append(groups, uniqueDesired, uniqueAny, duplicateDesired, questions)
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		sort.Slice(group, func(left, right int) bool {
			leftRank, leftErr := matchdomain.RewardQuestionRank(nonce, matchID, userID, group[left].ID)
			rightRank, rightErr := matchdomain.RewardQuestionRank(nonce, matchID, userID, group[right].ID)
			if leftErr != nil || rightErr != nil || leftRank == rightRank {
				return group[left].ID < group[right].ID
			}
			return leftRank < rightRank
		})
		return group[0], nil
	}
	return question.Question{}, economy.ErrInvalidEconomyState
}

func (repository *MongoEconomyRepository) insertRewardLedger(ctx context.Context, entry economy.LedgerEntry) error {
	ledgerID, err := NewID()
	if err != nil {
		return err
	}
	entry.ID = ledgerID
	if _, err := repository.ledger.InsertOne(ctx, entry); err != nil {
		return fmt.Errorf("record match reward: %w", err)
	}
	return nil
}

func (repository *MongoEconomyRepository) withTransaction(ctx context.Context, operation func(context.Context) error) error {
	session, err := repository.database.Client().StartSession()
	if err != nil {
		return fmt.Errorf("start economy transaction: %w", err)
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(
		ctx,
		func(tx context.Context) (any, error) { return nil, operation(tx) },
		options.Transaction().SetReadConcern(readconcern.Snapshot()).SetWriteConcern(writeconcern.Majority()),
	)
	if err != nil {
		return fmt.Errorf("economy transaction: %w", err)
	}
	return nil
}

func duplicateInt64(values []int64) bool {
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return true
		}
		if _, exists := seen[value]; exists {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func matched(result *mongo.UpdateResult) error {
	if result == nil || result.MatchedCount != 1 {
		return ErrConflict
	}
	return nil
}

func matchedAndModified(result *mongo.UpdateResult, expected int64) error {
	if expected < 0 || result == nil || result.MatchedCount != expected || result.ModifiedCount != expected {
		return ErrConflict
	}
	return nil
}
