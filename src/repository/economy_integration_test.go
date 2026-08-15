package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/akorwash/QuizBattle/domain/economy"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
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
