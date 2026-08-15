package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/akorwash/QuizBattle/domain/economy"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
	"github.com/akorwash/QuizBattle/domain/question"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

const integrationMongoEnvironment = "QUIZBATTLE_TEST_MONGO_URI"

func TestEconomyExpiryAndIdempotencyIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoEconomyRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	now := time.Now().UTC()

	wallet := economy.Wallet{UserID: 11, Balance: 600, Locked: 80, Version: 1, CreatedAt: now, UpdatedAt: now}
	listingCard := economy.Card{ID: 201, OwnerID: 11, QuestionID: "science-0001", Edition: 1, Status: economy.CardMarketEscrow, LockRef: "listing:101", Version: 1, CreatedAt: now, UpdatedAt: now}
	tradeCard := economy.Card{ID: 301, OwnerID: 11, QuestionID: "history-0001", Edition: 1, Status: economy.CardTradeEscrow, LockRef: "trade:102", Version: 1, CreatedAt: now, UpdatedAt: now}
	listing := economy.Listing{ID: 101, CardID: listingCard.ID, SellerID: 11, Price: 100, Status: economy.ListingActive, CreatedAt: now.Add(-8 * 24 * time.Hour), UpdatedAt: now, ExpiresAt: now.Add(-time.Hour), Version: 1, CommandID: "listing-create-101"}
	trade := economy.TradeOffer{ID: 102, SenderID: 11, ReceiverID: 22, OfferedCardIDs: []int64{tradeCard.ID}, OfferedCoins: 80, Status: economy.TradePending, CreatedAt: now.Add(-25 * time.Hour), UpdatedAt: now, ExpiresAt: now.Add(-time.Hour), Version: 1, CommandID: "trade-create-102"}

	mustInsert(t, ctx, database.Collection(walletCollection), wallet)
	mustInsert(t, ctx, database.Collection(cardCollection), listingCard, tradeCard)
	mustInsert(t, ctx, database.Collection(listingCollection), listing)
	mustInsert(t, ctx, database.Collection(tradeCollection), trade)

	if count, err := repository.ExpireListings(ctx, 10); err != nil || count != 1 {
		t.Fatalf("expire listings: count=%d err=%v", count, err)
	}
	if count, err := repository.ExpireTrades(ctx, 10); err != nil || count != 1 {
		t.Fatalf("expire trades: count=%d err=%v", count, err)
	}
	if count, err := repository.ExpireListings(ctx, 10); err != nil || count != 0 {
		t.Fatalf("listing sweep was not idempotent: count=%d err=%v", count, err)
	}
	if count, err := repository.ExpireTrades(ctx, 10); err != nil || count != 0 {
		t.Fatalf("trade sweep was not idempotent: count=%d err=%v", count, err)
	}

	var storedListing economy.Listing
	if err := database.Collection(listingCollection).FindOne(ctx, bson.M{"id": listing.ID}).Decode(&storedListing); err != nil {
		t.Fatal(err)
	}
	if storedListing.Status != economy.ListingExpired || storedListing.Version != 2 {
		t.Fatalf("listing not expired exactly once: %+v", storedListing)
	}
	var storedTrade economy.TradeOffer
	if err := database.Collection(tradeCollection).FindOne(ctx, bson.M{"id": trade.ID}).Decode(&storedTrade); err != nil {
		t.Fatal(err)
	}
	if storedTrade.Status != economy.TradeExpired || storedTrade.Version != 2 {
		t.Fatalf("trade not expired exactly once: %+v", storedTrade)
	}
	for _, cardID := range []int64{listingCard.ID, tradeCard.ID} {
		var card economy.Card
		if err := database.Collection(cardCollection).FindOne(ctx, bson.M{"id": cardID}).Decode(&card); err != nil {
			t.Fatal(err)
		}
		if card.Status != economy.CardAvailable || card.LockRef != "" || card.Version != 2 {
			t.Fatalf("card %d escrow not released: %+v", cardID, card)
		}
	}
	var storedWallet economy.Wallet
	if err := database.Collection(walletCollection).FindOne(ctx, bson.M{"userId": wallet.UserID}).Decode(&storedWallet); err != nil {
		t.Fatal(err)
	}
	if storedWallet.Locked != 0 || storedWallet.Balance != wallet.Balance || storedWallet.Version != 2 {
		t.Fatalf("trade coins not released: %+v", storedWallet)
	}

	first, err := repository.CreateTrade(ctx, 11, 22, nil, nil, 10, 0, "trade-idempotency-001")
	if err != nil {
		t.Fatal(err)
	}
	replay, err := repository.CreateTrade(ctx, 11, 22, nil, nil, 10, 0, "trade-idempotency-001")
	if err != nil || replay.ID != first.ID {
		t.Fatalf("matching replay: first=%+v replay=%+v err=%v", first, replay, err)
	}
	if _, err := repository.CreateTrade(ctx, 11, 23, nil, nil, 10, 0, "trade-idempotency-001"); !errors.Is(err, ErrConflict) {
		t.Fatalf("mismatched replay should conflict: %v", err)
	}
	if err := database.Collection(walletCollection).FindOne(ctx, bson.M{"userId": wallet.UserID}).Decode(&storedWallet); err != nil {
		t.Fatal(err)
	}
	if storedWallet.Locked != 10 {
		t.Fatalf("idempotent replay locked coins more than once: %+v", storedWallet)
	}
}

func TestMatchSettlementRejectsIncompleteCardMutationIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoEconomyRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	now := time.Now().UTC()
	const matchID int64 = 501

	players := []matchdomain.Player{{UserID: 41, Score: 10}, {UserID: 42, Score: 0}}
	cardDocuments := make([]any, 0, matchdomain.TurnCount-1)
	for playerIndex := range players {
		for cardIndex := 0; cardIndex < matchdomain.DeckSize; cardIndex++ {
			cardID := int64(600 + playerIndex*matchdomain.DeckSize + cardIndex)
			players[playerIndex].Deck = append(players[playerIndex].Deck, matchdomain.CardSnapshot{ID: cardID, OwnerID: players[playerIndex].UserID})
			// Deliberately omit one locked card. Settlement must roll back all
			// rewards rather than silently finalizing a partial card mutation.
			if playerIndex == 1 && cardIndex == matchdomain.DeckSize-1 {
				continue
			}
			cardDocuments = append(cardDocuments, economy.Card{ID: cardID, OwnerID: players[playerIndex].UserID, QuestionID: fmt.Sprintf("question-%d", cardID), Edition: 1, Status: economy.CardMatchLocked, LockRef: fmt.Sprintf("match:%d", matchID), Version: 1, CreatedAt: now, UpdatedAt: now})
		}
	}
	aggregate := matchdomain.Aggregate{ID: matchID, GameID: 502, OwnerID: 41, Players: players, Status: matchdomain.StatusCompleted, WinnerID: 41, Version: 1, CreatedAt: now, CompletedAt: now}
	mustInsert(t, ctx, database.Collection(walletCollection),
		economy.Wallet{UserID: 41, Balance: 600, Version: 1, CreatedAt: now, UpdatedAt: now},
		economy.Wallet{UserID: 42, Balance: 600, Version: 1, CreatedAt: now, UpdatedAt: now},
	)
	if _, err := database.Collection(cardCollection).InsertMany(ctx, cardDocuments); err != nil {
		t.Fatal(err)
	}
	mustInsert(t, ctx, database.Collection(matchCollection), aggregate)

	if err := repository.SettleMatchRewards(ctx, matchID); !errors.Is(err, economy.ErrInvalidEconomyState) {
		t.Fatalf("incomplete card settlement should fail: %v", err)
	}
	for _, userID := range []int64{41, 42} {
		var wallet economy.Wallet
		if err := database.Collection(walletCollection).FindOne(ctx, bson.M{"userId": userID}).Decode(&wallet); err != nil {
			t.Fatal(err)
		}
		if wallet.Balance != 600 || wallet.Version != 1 {
			t.Fatalf("reward transaction did not roll back for %d: %+v", userID, wallet)
		}
	}
	if count, err := database.Collection(ledgerCollection).CountDocuments(ctx, bson.M{}); err != nil || count != 0 {
		t.Fatalf("reward ledger should roll back: count=%d err=%v", count, err)
	}
	var stored matchdomain.Aggregate
	if err := database.Collection(matchCollection).FindOne(ctx, bson.M{"id": matchID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.RewardsSettled {
		t.Fatal("failed settlement was marked complete")
	}
	if count, err := database.Collection(cardCollection).CountDocuments(ctx, bson.M{"status": economy.CardMatchLocked}); err != nil || count != int64(len(cardDocuments)) {
		t.Fatalf("card release should roll back: count=%d err=%v", count, err)
	}
}

func TestForfeitSettlementUnlocksCardsWithoutRewardsIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoEconomyRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	now := time.Now().UTC()
	const matchID int64 = 701

	players := []matchdomain.Player{{UserID: 61}, {UserID: 62}}
	cardDocuments := make([]any, 0, matchdomain.DeckSize)
	for cardIndex := 0; cardIndex < matchdomain.DeckSize; cardIndex++ {
		cardID := int64(800 + cardIndex)
		players[0].Deck = append(players[0].Deck, matchdomain.CardSnapshot{ID: cardID, OwnerID: players[0].UserID})
		cardDocuments = append(cardDocuments, economy.Card{
			ID: cardID, OwnerID: players[0].UserID, QuestionID: fmt.Sprintf("question-%d", cardID),
			Edition: 1, Status: economy.CardMatchLocked, LockRef: fmt.Sprintf("match:%d", matchID),
			Version: 1, CreatedAt: now, UpdatedAt: now,
		})
	}
	aggregate := matchdomain.Aggregate{
		ID: matchID, GameID: 702, OwnerID: 61, Players: players,
		Status: matchdomain.StatusForfeited, WinnerID: 62, Version: 1,
		CreatedAt: now, CompletedAt: now,
	}
	mustInsert(t, ctx, database.Collection(walletCollection),
		economy.Wallet{UserID: 61, Balance: 600, Version: 1, CreatedAt: now, UpdatedAt: now},
		economy.Wallet{UserID: 62, Balance: 600, Version: 1, CreatedAt: now, UpdatedAt: now},
	)
	if _, err := database.Collection(cardCollection).InsertMany(ctx, cardDocuments); err != nil {
		t.Fatal(err)
	}
	mustInsert(t, ctx, database.Collection(matchCollection), aggregate)

	if err := repository.SettleMatchRewards(ctx, matchID); err != nil {
		t.Fatalf("settle forfeit: %v", err)
	}
	if err := repository.SettleMatchRewards(ctx, matchID); err != nil {
		t.Fatalf("repeat forfeit settlement: %v", err)
	}
	for _, userID := range []int64{61, 62} {
		var wallet economy.Wallet
		if err := database.Collection(walletCollection).FindOne(ctx, bson.M{"userId": userID}).Decode(&wallet); err != nil {
			t.Fatal(err)
		}
		if wallet.Balance != 600 || wallet.Locked != 0 || wallet.Version != 1 {
			t.Fatalf("forfeit changed wallet %d: %+v", userID, wallet)
		}
	}
	if count, err := database.Collection(ledgerCollection).CountDocuments(ctx, bson.M{}); err != nil || count != 0 {
		t.Fatalf("forfeit should not create reward ledger entries: count=%d err=%v", count, err)
	}
	for cardIndex := 0; cardIndex < matchdomain.DeckSize; cardIndex++ {
		var storedCard economy.Card
		if err := database.Collection(cardCollection).FindOne(ctx, bson.M{"id": int64(800 + cardIndex)}).Decode(&storedCard); err != nil {
			t.Fatal(err)
		}
		if storedCard.Status != economy.CardAvailable || storedCard.LockRef != "" || storedCard.Plays != 0 || storedCard.Version != 2 {
			t.Fatalf("forfeit card was not cleanly released: %+v", storedCard)
		}
	}
	var storedMatch matchdomain.Aggregate
	if err := database.Collection(matchCollection).FindOne(ctx, bson.M{"id": matchID}).Decode(&storedMatch); err != nil {
		t.Fatal(err)
	}
	if !storedMatch.RewardsSettled {
		t.Fatal("forfeit settlement was not marked complete")
	}
}

func TestBotRewardSettlementMintsCardAndIsIdempotentIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoEconomyRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	now := time.Now().UTC()
	const userID int64 = 81
	insertRewardQuestionPool(t, ctx, database, now)
	mustInsert(t, ctx, database.Collection(walletCollection), economy.Wallet{UserID: userID, Balance: 600, Version: 1, CreatedAt: now, UpdatedAt: now})
	insertBotRewardMatch(t, ctx, database, 801, userID, 8_100, matchdomain.BotRandom, userID, now)

	if err := repository.SettleMatchRewards(ctx, 801); err != nil {
		t.Fatalf("settle bot win: %v", err)
	}
	if err := repository.SettleMatchRewards(ctx, 801); err != nil {
		t.Fatalf("repeat bot win settlement: %v", err)
	}

	var wallet economy.Wallet
	if err := database.Collection(walletCollection).FindOne(ctx, bson.M{"userId": userID}).Decode(&wallet); err != nil {
		t.Fatal(err)
	}
	if wallet.Balance != 600+matchdomain.BotRandomWinnerCoins || wallet.Version != 2 {
		t.Fatalf("bot reward credited incorrectly: %+v", wallet)
	}
	if count, err := database.Collection(walletCollection).CountDocuments(ctx, bson.M{"userId": matchdomain.BotActorID}); err != nil || count != 0 {
		t.Fatalf("bot unexpectedly received a wallet: count=%d err=%v", count, err)
	}
	if count, err := database.Collection(cardCollection).CountDocuments(ctx, bson.M{"ownerId": userID}); err != nil || count != matchdomain.DeckSize+1 {
		t.Fatalf("winner card count=%d err=%v", count, err)
	}
	if count, err := database.Collection(ledgerCollection).CountDocuments(ctx, bson.M{"referenceId": "801"}); err != nil || count != 2 {
		t.Fatalf("reward ledger count=%d err=%v", count, err)
	}
	if count, err := database.Collection(cardCollection).CountDocuments(ctx, bson.M{"lockRef": "match:801"}); err != nil || count != 0 {
		t.Fatalf("human cards remain locked: count=%d err=%v", count, err)
	}
	var stored matchdomain.Aggregate
	if err := database.Collection(matchCollection).FindOne(ctx, bson.M{"id": int64(801)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if !stored.RewardsSettled || len(stored.RewardReceipts) != 1 {
		t.Fatalf("reward receipt was not persisted: %+v", stored.RewardReceipts)
	}
	receipt := stored.RewardReceipts[0]
	if receipt.Status != matchdomain.RewardStatusGranted || receipt.CoinsGranted != matchdomain.BotRandomWinnerCoins || receipt.Card == nil || receipt.Card.ID <= 0 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	var quota rewardQuota
	if err := database.Collection(rewardQuotaCollection).FindOne(ctx, bson.M{"userId": userID}).Decode(&quota); err != nil {
		t.Fatal(err)
	}
	if quota.Used != 1 || !quota.ExpiresAt.After(time.Now().UTC().Add(23*time.Hour)) {
		t.Fatalf("unexpected bot quota: %+v", quota)
	}
}

func TestBotLossReleasesHumanCardsWithoutEconomicRewardIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoEconomyRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	now := time.Now().UTC()
	const userID int64 = 86
	insertRewardQuestionPool(t, ctx, database, now)
	mustInsert(t, ctx, database.Collection(walletCollection), economy.Wallet{UserID: userID, Balance: 600, Version: 1, CreatedAt: now, UpdatedAt: now})
	insertBotRewardMatch(t, ctx, database, 806, userID, 8_600, matchdomain.BotSmart, matchdomain.BotActorID, now)

	if err := repository.SettleMatchRewards(ctx, 806); err != nil {
		t.Fatalf("settle bot loss: %v", err)
	}
	var wallet economy.Wallet
	if err := database.Collection(walletCollection).FindOne(ctx, bson.M{"userId": userID}).Decode(&wallet); err != nil {
		t.Fatal(err)
	}
	if wallet.Balance != 600 || wallet.Version != 1 {
		t.Fatalf("bot loss changed wallet: %+v", wallet)
	}
	if count, err := database.Collection(cardCollection).CountDocuments(ctx, bson.M{"ownerId": userID}); err != nil || count != matchdomain.DeckSize {
		t.Fatalf("bot loss minted a card: count=%d err=%v", count, err)
	}
	if count, err := database.Collection(ledgerCollection).CountDocuments(ctx, bson.M{"referenceId": "806"}); err != nil || count != 0 {
		t.Fatalf("bot loss wrote reward ledger entries: count=%d err=%v", count, err)
	}
	if count, err := database.Collection(rewardQuotaCollection).CountDocuments(ctx, bson.M{"userId": userID}); err != nil || count != 0 {
		t.Fatalf("bot loss consumed quota: count=%d err=%v", count, err)
	}
	if count, err := database.Collection(cardCollection).CountDocuments(ctx, bson.M{"ownerId": userID, "status": economy.CardAvailable, "plays": 1, "wins": 0}); err != nil || count != matchdomain.DeckSize {
		t.Fatalf("bot loss mastery/release count=%d err=%v", count, err)
	}
	var stored matchdomain.Aggregate
	if err := database.Collection(matchCollection).FindOne(ctx, bson.M{"id": int64(806)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.RewardReceipts) != 1 || stored.RewardReceipts[0].Status != matchdomain.RewardStatusIneligible ||
		stored.RewardReceipts[0].Outcome != matchdomain.RewardOutcomeLoss || stored.RewardReceipts[0].Reason != string(matchdomain.RewardOutcomeLoss) {
		t.Fatalf("unexpected bot loss receipt: %+v", stored.RewardReceipts)
	}
}

func TestPVPRewardSettlementPreservesCoinsAndMintsWinnerCardIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoEconomyRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	now := time.Now().UTC()
	insertRewardQuestionPool(t, ctx, database, now)
	for _, userID := range []int64{111, 112} {
		mustInsert(t, ctx, database.Collection(walletCollection), economy.Wallet{UserID: userID, Balance: 600, Version: 1, CreatedAt: now, UpdatedAt: now})
	}
	insertPVPRewardMatch(t, ctx, database, 1_101, 111, 112, 11_100, now)

	if err := repository.SettleMatchRewards(ctx, 1_101); err != nil {
		t.Fatalf("settle pvp rewards: %v", err)
	}
	for userID, wantBalance := range map[int64]int64{111: 720, 112: 645} {
		var wallet economy.Wallet
		if err := database.Collection(walletCollection).FindOne(ctx, bson.M{"userId": userID}).Decode(&wallet); err != nil {
			t.Fatal(err)
		}
		if wallet.Balance != wantBalance {
			t.Fatalf("pvp wallet %d balance=%d want=%d", userID, wallet.Balance, wantBalance)
		}
	}
	if count, err := database.Collection(cardCollection).CountDocuments(ctx, bson.M{"ownerId": int64(111)}); err != nil || count != matchdomain.DeckSize+1 {
		t.Fatalf("pvp winner card count=%d err=%v", count, err)
	}
	if count, err := database.Collection(cardCollection).CountDocuments(ctx, bson.M{"ownerId": int64(112)}); err != nil || count != matchdomain.DeckSize {
		t.Fatalf("pvp loser card count=%d err=%v", count, err)
	}
	if count, err := database.Collection(ledgerCollection).CountDocuments(ctx, bson.M{"referenceId": "1101"}); err != nil || count != 3 {
		t.Fatalf("pvp ledger count=%d err=%v", count, err)
	}
	var stored matchdomain.Aggregate
	if err := database.Collection(matchCollection).FindOne(ctx, bson.M{"id": int64(1_101)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.RewardReceipts) != 2 || stored.RewardReceipts[0].Card == nil || stored.RewardReceipts[1].Card != nil {
		t.Fatalf("unexpected pvp receipts: %+v", stored.RewardReceipts)
	}
}

func TestPVPDailyRewardCapStopsCoinsAndCardMintingIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoEconomyRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	now := time.Now().UTC()
	insertRewardQuestionPool(t, ctx, database, now)
	for _, userID := range []int64{121, 122} {
		mustInsert(t, ctx, database.Collection(walletCollection), economy.Wallet{
			UserID: userID, Balance: 600, Version: 1, CreatedAt: now, UpdatedAt: now,
		})
		day := now.Format("2006-01-02")
		mustInsert(t, ctx, database.Collection(rewardQuotaCollection), rewardQuota{
			ID:     fmt.Sprintf("%s:%s:%d:%s", matchdomain.RewardPolicyV1, matchdomain.RewardSourcePVP, userID, day),
			UserID: userID, Day: day, Used: matchdomain.PVPDailyRewardLimit, Version: 1,
			UpdatedAt: now, ExpiresAt: now.Add(8 * 24 * time.Hour),
		})
	}
	insertPVPRewardMatch(t, ctx, database, 1_201, 121, 122, 12_100, now)

	if err := repository.SettleMatchRewards(ctx, 1_201); err != nil {
		t.Fatalf("settle capped pvp rewards: %v", err)
	}
	if err := repository.SettleMatchRewards(ctx, 1_201); err != nil {
		t.Fatalf("repeat capped pvp settlement: %v", err)
	}
	for _, userID := range []int64{121, 122} {
		var wallet economy.Wallet
		if err := database.Collection(walletCollection).FindOne(ctx, bson.M{"userId": userID}).Decode(&wallet); err != nil {
			t.Fatal(err)
		}
		if wallet.Balance != 600 || wallet.Version != 1 {
			t.Fatalf("capped pvp reward changed wallet %d: %+v", userID, wallet)
		}
		if count, err := database.Collection(cardCollection).CountDocuments(ctx, bson.M{"ownerId": userID}); err != nil || count != matchdomain.DeckSize {
			t.Fatalf("capped pvp reward minted a card for %d: count=%d err=%v", userID, count, err)
		}
	}
	if count, err := database.Collection(ledgerCollection).CountDocuments(ctx, bson.M{"referenceId": "1201"}); err != nil || count != 0 {
		t.Fatalf("capped pvp reward wrote ledger entries: count=%d err=%v", count, err)
	}
	var stored matchdomain.Aggregate
	if err := database.Collection(matchCollection).FindOne(ctx, bson.M{"id": int64(1_201)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if !stored.RewardsSettled || len(stored.RewardReceipts) != 2 {
		t.Fatalf("capped pvp receipts missing: %+v", stored.RewardReceipts)
	}
	for _, receipt := range stored.RewardReceipts {
		if receipt.Status != matchdomain.RewardStatusCapped || receipt.Reason != matchdomain.RewardReasonPVPDailyCap ||
			receipt.CoinsGranted != 0 || receipt.Card != nil {
			t.Fatalf("unexpected capped pvp receipt: %+v", receipt)
		}
	}
}

func TestLegacyPVPSettlementIgnoresV1DailyQuotaIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoEconomyRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	now := time.Now().UTC()
	for _, userID := range []int64{131, 132} {
		mustInsert(t, ctx, database.Collection(walletCollection), economy.Wallet{
			UserID: userID, Balance: 600, Version: 1, CreatedAt: now, UpdatedAt: now,
		})
		day := now.Format("2006-01-02")
		mustInsert(t, ctx, database.Collection(rewardQuotaCollection), rewardQuota{
			ID:     fmt.Sprintf("%s:%s:%d:%s", matchdomain.RewardPolicyV1, matchdomain.RewardSourcePVP, userID, day),
			UserID: userID, Day: day, Used: matchdomain.PVPDailyRewardLimit, Version: 1,
			UpdatedAt: now, ExpiresAt: now.Add(8 * 24 * time.Hour),
		})
	}
	insertPVPRewardMatch(t, ctx, database, 1_301, 131, 132, 13_100, now)
	if _, err := database.Collection(matchCollection).UpdateOne(
		ctx,
		bson.M{"id": int64(1_301)},
		bson.M{"$unset": bson.M{"rewardPolicyVersion": "", "rewardNonce": ""}},
	); err != nil {
		t.Fatal(err)
	}

	if err := repository.SettleMatchRewards(ctx, 1_301); err != nil {
		t.Fatalf("settle legacy pvp rewards: %v", err)
	}
	for userID, wantBalance := range map[int64]int64{131: 720, 132: 645} {
		var wallet economy.Wallet
		if err := database.Collection(walletCollection).FindOne(ctx, bson.M{"userId": userID}).Decode(&wallet); err != nil {
			t.Fatal(err)
		}
		if wallet.Balance != wantBalance {
			t.Fatalf("legacy wallet %d balance=%d want=%d", userID, wallet.Balance, wantBalance)
		}
		var quota rewardQuota
		if err := database.Collection(rewardQuotaCollection).FindOne(ctx, bson.M{"userId": userID}).Decode(&quota); err != nil {
			t.Fatal(err)
		}
		if quota.Used != matchdomain.PVPDailyRewardLimit || quota.Version != 1 {
			t.Fatalf("legacy settlement changed v1 quota for %d: %+v", userID, quota)
		}
	}
	var stored matchdomain.Aggregate
	if err := database.Collection(matchCollection).FindOne(ctx, bson.M{"id": int64(1_301)}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if !stored.RewardsSettled || len(stored.RewardReceipts) != 0 {
		t.Fatalf("legacy settlement wrote v1 receipts: %+v", stored.RewardReceipts)
	}
}

func TestBotDailyRewardCapIsAtomicIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoEconomyRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	now := time.Now().UTC()
	const userID int64 = 91
	insertRewardQuestionPool(t, ctx, database, now)
	mustInsert(t, ctx, database.Collection(walletCollection), economy.Wallet{UserID: userID, Balance: 600, Version: 1, CreatedAt: now, UpdatedAt: now})
	for index := 0; index < 4; index++ {
		matchID := int64(900 + index)
		insertBotRewardMatch(t, ctx, database, matchID, userID, 9_100+int64(index*100), matchdomain.BotRandom, userID, now)
		if err := repository.SettleMatchRewards(ctx, matchID); err != nil {
			t.Fatalf("settle bot match %d: %v", matchID, err)
		}
	}
	var wallet economy.Wallet
	if err := database.Collection(walletCollection).FindOne(ctx, bson.M{"userId": userID}).Decode(&wallet); err != nil {
		t.Fatal(err)
	}
	if wallet.Balance != 600+int64(matchdomain.BotDailyRewardLimit)*matchdomain.BotRandomWinnerCoins {
		t.Fatalf("daily cap balance = %d", wallet.Balance)
	}
	if count, err := database.Collection(cardCollection).CountDocuments(ctx, bson.M{"ownerId": userID}); err != nil || count != 4*matchdomain.DeckSize+matchdomain.BotDailyRewardLimit {
		t.Fatalf("daily cap card count=%d err=%v", count, err)
	}
	var capped matchdomain.Aggregate
	if err := database.Collection(matchCollection).FindOne(ctx, bson.M{"id": int64(903)}).Decode(&capped); err != nil {
		t.Fatal(err)
	}
	if len(capped.RewardReceipts) != 1 || capped.RewardReceipts[0].Status != matchdomain.RewardStatusCapped ||
		capped.RewardReceipts[0].CoinsGranted != 0 || capped.RewardReceipts[0].Card != nil || capped.RewardReceipts[0].Reason != matchdomain.RewardReasonBotDailyCap {
		t.Fatalf("fourth bot win was not capped: %+v", capped.RewardReceipts)
	}
}

func TestConcurrentFirstBotQuotaSettlementsBothCommitOnceIntegration(t *testing.T) {
	database := integrationEconomyDatabase(t)
	repository := NewMongoEconomyRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	now := time.Now().UTC()
	const userID int64 = 101
	insertRewardQuestionPool(t, ctx, database, now)
	mustInsert(t, ctx, database.Collection(walletCollection), economy.Wallet{UserID: userID, Balance: 600, Version: 1, CreatedAt: now, UpdatedAt: now})
	insertBotRewardMatch(t, ctx, database, 1_001, userID, 10_100, matchdomain.BotRandom, userID, now)
	insertBotRewardMatch(t, ctx, database, 1_002, userID, 10_200, matchdomain.BotRandom, userID, now)

	errorsChannel := make(chan error, 2)
	var wait sync.WaitGroup
	for _, matchID := range []int64{1_001, 1_002} {
		wait.Add(1)
		go func(id int64) {
			defer wait.Done()
			errorsChannel <- repository.SettleMatchRewards(ctx, id)
		}(matchID)
	}
	wait.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("concurrent settlement: %v", err)
		}
	}
	var quota rewardQuota
	if err := database.Collection(rewardQuotaCollection).FindOne(ctx, bson.M{"userId": userID}).Decode(&quota); err != nil {
		t.Fatal(err)
	}
	if quota.Used != 2 {
		t.Fatalf("concurrent quota used=%d", quota.Used)
	}
	var wallet economy.Wallet
	if err := database.Collection(walletCollection).FindOne(ctx, bson.M{"userId": userID}).Decode(&wallet); err != nil {
		t.Fatal(err)
	}
	if wallet.Balance != 600+2*matchdomain.BotRandomWinnerCoins {
		t.Fatalf("concurrent reward balance=%d", wallet.Balance)
	}
	if count, err := database.Collection(cardCollection).CountDocuments(ctx, bson.M{"ownerId": userID}); err != nil || count != 2*matchdomain.DeckSize+2 {
		t.Fatalf("concurrent reward cards=%d err=%v", count, err)
	}
}

func insertRewardQuestionPool(t *testing.T, ctx context.Context, database *mongo.Database, now time.Time) {
	t.Helper()
	documents := make([]any, 0, 6)
	for index, difficulty := range []question.Difficulty{
		question.DifficultyEasy, question.DifficultyEasy,
		question.DifficultyMedium, question.DifficultyMedium,
		question.DifficultyHard, question.DifficultyHard,
	} {
		documents = append(documents, question.Question{
			ID: fmt.Sprintf("reward-question-%02d", index+1), Category: "science", Difficulty: difficulty,
			Prompt: "سؤال صالح لمكافأة المباراة", Options: []string{"أ", "ب", "ج", "د"},
			Status: question.StatusActive, VerifiedAt: now, Language: "ar",
		})
	}
	if _, err := database.Collection(questionBankCollection).InsertMany(ctx, documents); err != nil {
		t.Fatal(err)
	}
}

func insertBotRewardMatch(
	t *testing.T,
	ctx context.Context,
	database *mongo.Database,
	matchID, userID, firstCardID int64,
	strategy matchdomain.BotStrategy,
	winnerID int64,
	completedAt time.Time,
) {
	t.Helper()
	human := matchdomain.Player{UserID: userID, Kind: matchdomain.PlayerHuman}
	bot := matchdomain.Player{UserID: matchdomain.BotActorID, Kind: matchdomain.PlayerBot, Bot: &matchdomain.BotConfig{Strategy: strategy}}
	cards := make([]any, 0, matchdomain.DeckSize)
	for index := 0; index < matchdomain.DeckSize; index++ {
		cardID := firstCardID + int64(index)
		human.Deck = append(human.Deck, matchdomain.CardSnapshot{ID: cardID, OwnerID: userID})
		bot.Deck = append(bot.Deck, matchdomain.CardSnapshot{ID: -100 - int64(index), OwnerID: matchdomain.BotActorID})
		cards = append(cards, economy.Card{
			ID: cardID, OwnerID: userID, QuestionID: fmt.Sprintf("deck-question-%d", cardID), Edition: 1,
			Rarity: "common", Power: 1, Status: economy.CardMatchLocked, LockRef: fmt.Sprintf("match:%d", matchID),
			Version: 1, CreatedAt: completedAt.Add(-time.Minute), UpdatedAt: completedAt.Add(-time.Minute),
		})
	}
	if _, err := database.Collection(cardCollection).InsertMany(ctx, cards); err != nil {
		t.Fatal(err)
	}
	aggregate := matchdomain.Aggregate{
		ID: matchID, GameID: matchID + 10_000, OwnerID: userID, Mode: matchdomain.ModeBot,
		Players: []matchdomain.Player{human, bot}, Status: matchdomain.StatusCompleted,
		WinnerID: winnerID, WinnerIDs: []int64{winnerID}, Version: 1,
		CreatedAt: completedAt.Add(-time.Minute), CompletedAt: completedAt,
		RewardPolicyVersion: matchdomain.RewardPolicyV1,
		RewardNonce:         bytes.Repeat([]byte{byte(matchID%251 + 1)}, matchdomain.RewardNonceSize),
	}
	mustInsert(t, ctx, database.Collection(matchCollection), aggregate)
}

func insertPVPRewardMatch(
	t *testing.T,
	ctx context.Context,
	database *mongo.Database,
	matchID, winnerID, loserID, firstCardID int64,
	completedAt time.Time,
) {
	t.Helper()
	players := []matchdomain.Player{{UserID: winnerID}, {UserID: loserID}}
	cards := make([]any, 0, matchdomain.TurnCount)
	for playerIndex := range players {
		for cardIndex := 0; cardIndex < matchdomain.DeckSize; cardIndex++ {
			cardID := firstCardID + int64(playerIndex*matchdomain.DeckSize+cardIndex)
			players[playerIndex].Deck = append(players[playerIndex].Deck, matchdomain.CardSnapshot{ID: cardID, OwnerID: players[playerIndex].UserID})
			cards = append(cards, economy.Card{
				ID: cardID, OwnerID: players[playerIndex].UserID, QuestionID: fmt.Sprintf("deck-question-%d", cardID), Edition: 1,
				Rarity: "common", Power: 1, Status: economy.CardMatchLocked, LockRef: fmt.Sprintf("match:%d", matchID),
				Version: 1, CreatedAt: completedAt.Add(-time.Minute), UpdatedAt: completedAt.Add(-time.Minute),
			})
		}
	}
	if _, err := database.Collection(cardCollection).InsertMany(ctx, cards); err != nil {
		t.Fatal(err)
	}
	aggregate := matchdomain.Aggregate{
		ID: matchID, GameID: matchID + 10_000, OwnerID: winnerID, Mode: matchdomain.ModeDuel,
		Players: players, Status: matchdomain.StatusCompleted, WinnerID: winnerID, WinnerIDs: []int64{winnerID},
		Version: 1, CreatedAt: completedAt.Add(-time.Minute), CompletedAt: completedAt,
		RewardPolicyVersion: matchdomain.RewardPolicyV1,
		RewardNonce:         bytes.Repeat([]byte{byte(matchID%251 + 1)}, matchdomain.RewardNonceSize),
	}
	mustInsert(t, ctx, database.Collection(matchCollection), aggregate)
}

func integrationEconomyDatabase(t *testing.T) *mongo.Database {
	t.Helper()
	uri := strings.TrimSpace(os.Getenv(integrationMongoEnvironment))
	if uri == "" {
		t.Skipf("set %s to run MongoDB integration tests", integrationMongoEnvironment)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(10 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		t.Fatal(err)
	}
	databaseName := fmt.Sprintf("quizbattle_economy_it_%d", time.Now().UnixNano())
	database := client.Database(databaseName)
	if err := EnsureIndexes(ctx, database); err != nil {
		_ = client.Disconnect(context.Background())
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		if !strings.HasPrefix(databaseName, "quizbattle_economy_it_") {
			t.Errorf("refusing to drop unexpected integration database %q", databaseName)
		} else if err := database.Drop(cleanupCtx); err != nil {
			t.Errorf("drop integration database: %v", err)
		}
		if err := client.Disconnect(cleanupCtx); err != nil {
			t.Errorf("disconnect integration client: %v", err)
		}
	})
	return database
}

func mustInsert(t *testing.T, ctx context.Context, collection *mongo.Collection, documents ...any) {
	t.Helper()
	if len(documents) == 1 {
		if _, err := collection.InsertOne(ctx, documents[0]); err != nil {
			t.Fatal(err)
		}
		return
	}
	if _, err := collection.InsertMany(ctx, documents); err != nil {
		t.Fatal(err)
	}
}
