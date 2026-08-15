package service

import (
	"context"
	"testing"
	"time"

	"github.com/akorwash/QuizBattle/domain/economy"
	matchdomain "github.com/akorwash/QuizBattle/domain/match"
)

type snapshotMatchRepository struct {
	aggregate *matchdomain.Aggregate
}

func (repository *snapshotMatchRepository) CreateForGame(context.Context, *matchdomain.Aggregate) error {
	return nil
}

func (repository *snapshotMatchRepository) GetByGameID(context.Context, int64) (*matchdomain.Aggregate, error) {
	return repository.aggregate, nil
}

func (repository *snapshotMatchRepository) Update(context.Context, *matchdomain.Aggregate, int64) error {
	return nil
}

func (repository *snapshotMatchRepository) CommitDeck(context.Context, *matchdomain.Aggregate, int64, int64, []int64, []int64) error {
	return nil
}

type snapshotEconomyRepository struct {
	settlements int
	aggregate   *matchdomain.Aggregate
}

func (repository *snapshotEconomyRepository) GetCardsByIDs(context.Context, []int64) (map[int64]economy.Card, error) {
	return nil, nil
}

func (repository *snapshotEconomyRepository) SettleMatchRewards(context.Context, int64) error {
	repository.settlements++
	if repository.aggregate != nil {
		repository.aggregate.RewardsSettled = true
	}
	return nil
}

func TestSnapshotRetriesUnsettledForfeit(t *testing.T) {
	now := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	aggregate, err := matchdomain.New(100, 200, 10, 20, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.Forfeit(10, "forfeit-recovery-1", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	economyRepository := &snapshotEconomyRepository{aggregate: aggregate}
	matchService := NewMatchService(
		&snapshotMatchRepository{aggregate: aggregate},
		economyRepository,
		nil,
		nil,
		nil,
		nil,
	)

	snapshot, err := matchService.Snapshot(context.Background(), 10, 200)
	if err != nil {
		t.Fatal(err)
	}
	if economyRepository.settlements != 1 || !snapshot.RewardsSettled {
		t.Fatalf("unsettled forfeit was not recovered: settlements=%d snapshot=%+v", economyRepository.settlements, snapshot)
	}
}
