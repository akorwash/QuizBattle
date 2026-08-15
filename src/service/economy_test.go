package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/akorwash/QuizBattle/domain/economy"
	"github.com/akorwash/QuizBattle/domain/question"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
)

var errEconomyBoundaryStop = errors.New("economy boundary stop")

type economyBoundaryRepository struct {
	events       []string
	walletReady  bool
	starterUsers []int64
}

func (fake *economyBoundaryRepository) EnsureStarter(_ context.Context, userID int64, _ []question.Question) error {
	fake.events = append(fake.events, fmt.Sprintf("ensure:%d", userID))
	fake.starterUsers = append(fake.starterUsers, userID)
	fake.walletReady = true
	return nil
}

func (fake *economyBoundaryRepository) GetWallet(_ context.Context, userID int64) (*economy.Wallet, error) {
	fake.events = append(fake.events, fmt.Sprintf("wallet:%d", userID))
	if !fake.walletReady {
		return nil, repository.ErrNotFound
	}
	return &economy.Wallet{UserID: userID, Balance: economy.StarterBalance, Version: 1}, nil
}

func (fake *economyBoundaryRepository) ListCards(context.Context, int64) ([]economy.Card, error) {
	return nil, nil
}

func (fake *economyBoundaryRepository) GetCardsByIDs(context.Context, []int64) (map[int64]economy.Card, error) {
	return nil, nil
}

func (fake *economyBoundaryRepository) CreateListing(context.Context, int64, int64, int64, string) (*economy.Listing, error) {
	fake.events = append(fake.events, "create_listing")
	return nil, errEconomyBoundaryStop
}

func (fake *economyBoundaryRepository) ExpireListings(context.Context, int64) (int64, error) {
	fake.events = append(fake.events, "expire_listings")
	return 0, nil
}

func (fake *economyBoundaryRepository) ListActiveListings(context.Context, int64) ([]economy.Listing, error) {
	fake.events = append(fake.events, "list_listings")
	return nil, errEconomyBoundaryStop
}

func (fake *economyBoundaryRepository) BuyListing(context.Context, int64, int64, string) (*economy.Listing, error) {
	fake.events = append(fake.events, "buy_listing")
	return nil, errEconomyBoundaryStop
}

func (fake *economyBoundaryRepository) CancelListing(context.Context, int64, int64, string) (*economy.Listing, error) {
	return nil, errEconomyBoundaryStop
}

func (fake *economyBoundaryRepository) CreateTrade(context.Context, int64, int64, []int64, []int64, int64, int64, string) (*economy.TradeOffer, error) {
	fake.events = append(fake.events, "create_trade")
	return nil, errEconomyBoundaryStop
}

func (fake *economyBoundaryRepository) ExpireTrades(context.Context, int64) (int64, error) {
	fake.events = append(fake.events, "expire_trades")
	return 0, nil
}

func (fake *economyBoundaryRepository) ListTrades(context.Context, int64) ([]economy.TradeOffer, error) {
	fake.events = append(fake.events, "list_trades")
	return nil, errEconomyBoundaryStop
}

func (fake *economyBoundaryRepository) AcceptTrade(context.Context, int64, int64, string) (*economy.TradeOffer, error) {
	fake.events = append(fake.events, "accept_trade")
	return nil, errEconomyBoundaryStop
}

func (fake *economyBoundaryRepository) CloseTrade(context.Context, int64, int64, string, string) (*economy.TradeOffer, error) {
	return nil, errEconomyBoundaryStop
}

func TestEconomyCommandsEnsureAuthenticatedActorAndSweepBeforeSettlement(t *testing.T) {
	tests := []struct {
		name string
		call func(*EconomyService) error
		want []string
	}{
		{
			name: "create listing",
			call: func(service *EconomyService) error {
				_, err := service.CreateListing(context.Background(), 11, resources.CreateListingModel{CardID: 7, Price: 100, CommandID: "listing-0001"})
				return err
			},
			want: []string{"wallet:11", "create_listing"},
		},
		{
			name: "buy listing",
			call: func(service *EconomyService) error {
				_, err := service.BuyListing(context.Background(), 11, 8, "purchase-001")
				return err
			},
			want: []string{"wallet:11", "expire_listings", "buy_listing"},
		},
		{
			name: "create trade does not initialize receiver",
			call: func(service *EconomyService) error {
				_, err := service.CreateTrade(context.Background(), 11, resources.CreateTradeModel{ReceiverID: 22, OfferedCoins: 10, CommandID: "trade-create-001"})
				return err
			},
			want: []string{"wallet:11", "create_trade"},
		},
		{
			name: "accept trade",
			call: func(service *EconomyService) error {
				_, err := service.AcceptTrade(context.Background(), 11, 9, "trade-accept-001")
				return err
			},
			want: []string{"wallet:11", "expire_trades", "accept_trade"},
		},
		{
			name: "market read",
			call: func(service *EconomyService) error {
				_, err := service.Market(context.Background())
				return err
			},
			want: []string{"expire_listings", "list_listings"},
		},
		{
			name: "trade read",
			call: func(service *EconomyService) error {
				_, err := service.Trades(context.Background(), 11)
				return err
			},
			want: []string{"expire_trades", "list_trades"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &economyBoundaryRepository{walletReady: true}
			service := NewEconomyService(fake, nil)
			if err := test.call(service); !errors.Is(err, errEconomyBoundaryStop) {
				t.Fatalf("got error %v", err)
			}
			if !reflect.DeepEqual(fake.events, test.want) {
				t.Fatalf("events=%v want=%v", fake.events, test.want)
			}
		})
	}
}

func TestEnsureStarterCreatesOnlyRequestedUsersEconomy(t *testing.T) {
	categories := []string{"science", "mathematics", "geography", "history", "religion"}
	items := make([]question.Question, 0, economy.StarterCards)
	for index := 0; index < economy.StarterCards; index++ {
		items = append(items, question.Question{ID: fmt.Sprintf("starter-%04d", index), Category: categories[index%len(categories)]})
	}
	fake := &economyBoundaryRepository{}
	questionService := NewQuestionBankService(questionBankStub{items: items})
	service := NewEconomyService(fake, questionService)

	wallet, err := service.ensureStarter(context.Background(), 77)
	if err != nil {
		t.Fatal(err)
	}
	if wallet.UserID != 77 || !reflect.DeepEqual(fake.starterUsers, []int64{77}) {
		t.Fatalf("wallet=%+v initialized=%v", wallet, fake.starterUsers)
	}
	wantEvents := []string{"wallet:77", "ensure:77", "wallet:77"}
	if !reflect.DeepEqual(fake.events, wantEvents) {
		t.Fatalf("events=%v want=%v", fake.events, wantEvents)
	}
}
