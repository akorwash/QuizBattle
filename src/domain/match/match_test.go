package match

import (
	"errors"
	"testing"
	"time"
)

func TestMatchLifecycleScoringAndSanitization(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	aggregate := mustMatch(t, now)
	commitBothDecks(t, aggregate, now)
	changed, err := aggregate.Start(11, "start-0001", now.Add(time.Second))
	if err != nil || !changed {
		t.Fatalf("start: changed=%v err=%v", changed, err)
	}

	before, err := aggregate.SafeSnapshot(11)
	if err != nil {
		t.Fatal(err)
	}
	if before.CurrentTurn == nil || before.CurrentTurn.CorrectOption != nil || before.CurrentTurn.Explanation != "" {
		t.Fatalf("correct answer leaked before resolution: %#v", before.CurrentTurn)
	}

	turnID := before.CurrentTurn.ID
	answerAt := before.CurrentTurn.StartedAt.Add(10 * time.Second)
	if _, err := aggregate.SubmitAnswer(11, turnID, 0, "answer-a-001", answerAt); err != nil {
		t.Fatal(err)
	}
	afterOne, _ := aggregate.SafeSnapshot(22)
	if afterOne.CurrentTurn.YourOption != nil || len(afterOne.CurrentTurn.Answers) != 0 {
		t.Fatal("opponent answer leaked while turn active")
	}
	if _, err := aggregate.SubmitAnswer(22, turnID, 1, "answer-b-001", answerAt); err != nil {
		t.Fatal(err)
	}
	resolved, _ := aggregate.SafeSnapshot(22)
	if resolved.CurrentTurn.Status != TurnResolved || resolved.CurrentTurn.CorrectOption == nil || *resolved.CurrentTurn.CorrectOption != 0 {
		t.Fatalf("expected resolved answer, got %#v", resolved.CurrentTurn)
	}
	if aggregate.Players[0].Score != 125 || aggregate.Players[1].Score != 0 {
		t.Fatalf("unexpected scores: %+v", aggregate.Players)
	}
}

func TestDeadlinesAdvanceAndCompleteDeterministically(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	aggregate := mustMatch(t, now)
	commitBothDecks(t, aggregate, now)
	_, _ = aggregate.Start(11, "start-0001", now)

	// One late Tick catches up through every unanswered turn and completes.
	end := now.Add(time.Duration(TurnCount)*(TurnDuration+RevealDuration) + time.Second)
	if !aggregate.Tick(end) {
		t.Fatal("expected timeout transitions")
	}
	if aggregate.Status != StatusCompleted || !aggregate.IsTie || aggregate.CompletedAt.IsZero() {
		t.Fatalf("unexpected completion: status=%s tie=%v at=%v", aggregate.Status, aggregate.IsTie, aggregate.CompletedAt)
	}
	if rewards := aggregate.Rewards(); rewards[11] != 75 || rewards[22] != 75 {
		t.Fatalf("unexpected tie rewards: %#v", rewards)
	}
	version := aggregate.Version
	if aggregate.Tick(end.Add(time.Hour)) || aggregate.Version != version {
		t.Fatal("completed match changed after a later tick")
	}
}

func TestCommandsAreIdempotentAndBounded(t *testing.T) {
	now := time.Now().UTC()
	aggregate := mustMatch(t, now)
	cards := deck(11, 100)
	changed, err := aggregate.CommitDeck(11, cards, "deck-a-0001", now)
	if err != nil || !changed {
		t.Fatalf("first commit: %v %v", changed, err)
	}
	version := aggregate.Version
	changed, err = aggregate.CommitDeck(11, deck(11, 200), "deck-a-0001", now.Add(time.Second))
	if err != nil || changed || aggregate.Version != version || aggregate.Players[0].Deck[0].ID != cards[0].ID {
		t.Fatal("replayed command was not a no-op")
	}

	for index := 0; index < maximumProcessed+20; index++ {
		aggregate.recordCommand(commandID(index), 11, "test", now)
	}
	if len(aggregate.ProcessedCommands) != maximumProcessed {
		t.Fatalf("processed command history is unbounded: %d", len(aggregate.ProcessedCommands))
	}
}

func TestRejectsInvalidDecksAndAnswers(t *testing.T) {
	now := time.Now().UTC()
	aggregate := mustMatch(t, now)
	cases := []struct {
		name  string
		user  int64
		cards []CardSnapshot
		err   error
	}{
		{"not player", 33, deck(33, 1), ErrNotPlayer},
		{"too short", 11, deck(11, 1)[:4], ErrInvalidDeck},
		{"wrong owner", 11, deck(22, 1), ErrInvalidDeck},
		{"duplicate", 11, duplicateDeck(11), ErrInvalidDeck},
	}
	for index, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := aggregate.CommitDeck(test.user, test.cards, commandID(index), now)
			if !errors.Is(err, test.err) {
				t.Fatalf("got %v want %v", err, test.err)
			}
		})
	}

	commitBothDecks(t, aggregate, now)
	if _, err := aggregate.Start(22, "guest-start1", now); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("guest start: %v", err)
	}
	_, _ = aggregate.Start(11, "owner-start1", now)
	turnID := aggregate.Turns[0].ID
	if _, err := aggregate.SubmitAnswer(33, turnID, 0, "outsider-001", now); !errors.Is(err, ErrNotPlayer) {
		t.Fatalf("outsider answer: %v", err)
	}
	if _, err := aggregate.SubmitAnswer(11, turnID, 4, "badoption001", now); !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("invalid option: %v", err)
	}
	if _, err := aggregate.SubmitAnswer(11, turnID, 0, "late-ans-001", now.Add(TurnDuration)); !errors.Is(err, ErrTurnClosed) {
		t.Fatalf("late answer: %v", err)
	}
}

func TestWinnerRewards(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	aggregate := mustMatch(t, now)
	commitBothDecks(t, aggregate, now)
	_, _ = aggregate.Start(11, "start-0001", now)

	for turnIndex := 0; turnIndex < TurnCount; turnIndex++ {
		turn := &aggregate.Turns[aggregate.CurrentTurn]
		at := turn.StartedAt.Add(time.Second)
		_, _ = aggregate.SubmitAnswer(11, turn.ID, 0, commandID(100+turnIndex*2), at)
		_, _ = aggregate.SubmitAnswer(22, turn.ID, 1, commandID(101+turnIndex*2), at)
		aggregate.Tick(turn.RevealUntil)
	}
	if aggregate.Status != StatusCompleted || aggregate.WinnerID != 11 || aggregate.IsTie {
		t.Fatalf("unexpected winner: status=%s winner=%d tie=%v", aggregate.Status, aggregate.WinnerID, aggregate.IsTie)
	}
	rewards := aggregate.Rewards()
	if rewards[11] != 120 || rewards[22] != 45 {
		t.Fatalf("unexpected rewards: %#v", rewards)
	}
}

func TestForfeitEndsMatchWithoutRewards(t *testing.T) {
	now := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	aggregate, err := New(100, 200, 10, 20, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.CommitDeck(10, deck(10, 1), "owner-deck-1", now); err != nil {
		t.Fatal(err)
	}
	changed, err := aggregate.Forfeit(10, "owner-forfeit-1", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !changed || aggregate.Status != StatusForfeited || aggregate.WinnerID != 20 {
		t.Fatalf("unexpected forfeited aggregate: %+v", aggregate)
	}
	rewards := aggregate.Rewards()
	if len(rewards) != 2 || rewards[10] != 0 || rewards[20] != 0 {
		t.Fatalf("forfeit must not mint coins: %#v", rewards)
	}
	aggregate.RewardsSettled = true
	snapshot, err := aggregate.SafeSnapshot(10)
	if err != nil || !snapshot.RewardsSettled || snapshot.RewardCoins != 0 {
		t.Fatalf("forfeit settlement was not exposed safely: snapshot=%+v err=%v", snapshot, err)
	}
	if changed, err := aggregate.Forfeit(10, "owner-forfeit-1", now.Add(2*time.Minute)); err != nil || changed {
		t.Fatalf("forfeit retry must be idempotent: changed=%v err=%v", changed, err)
	}
}

func TestArenaModePoliciesAndPreparedRosters(t *testing.T) {
	now := time.Date(2026, time.August, 15, 13, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		mode      Mode
		players   []int64
		min       int
		max       int
		teamSize  int
		wantTeams []int
		wantErr   error
	}{
		{name: "duel", mode: ModeDuel, players: []int64{22, 11}, min: 2, max: 2, teamSize: 1, wantTeams: []int{0, 0}},
		{name: "two versus two", mode: ModeTeam2v2, players: []int64{22, 11, 33, 44}, min: 4, max: 4, teamSize: 2, wantTeams: []int{1, 2, 1, 2}},
		{name: "four versus four", mode: ModeTeam4v4, players: []int64{11, 22, 33, 44, 55, 66, 77, 88}, min: 8, max: 8, teamSize: 4, wantTeams: []int{1, 2, 1, 2, 1, 2, 1, 2}},
		{name: "open partial", mode: ModeOpen, players: []int64{11, 22, 33}, min: 2, max: 8, teamSize: 0, wantTeams: []int{0, 0, 0}},
		{name: "fixed team cannot prepare short", mode: ModeTeam2v2, players: []int64{11, 22, 33}, wantErr: ErrInvalidMatch},
		{name: "open cannot exceed eight", mode: ModeOpen, players: []int64{11, 22, 33, 44, 55, 66, 77, 88, 99}, wantErr: ErrInvalidMatch},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			aggregate, err := NewArena(int64(100+index), 200, 11, test.mode, test.players, now)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("got %v want %v", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if aggregate.Players[0].UserID != 11 {
				t.Fatalf("owner was not normalized to the first player: %+v", aggregate.Players)
			}
			if MinPlayers(test.mode) != test.min || MaxPlayers(test.mode) != test.max || TeamSize(test.mode) != test.teamSize {
				t.Fatalf("unexpected policy for %s", test.mode)
			}
			for playerIndex, team := range test.wantTeams {
				if aggregate.Players[playerIndex].Team != team {
					t.Fatalf("player %d team=%d want %d", playerIndex, aggregate.Players[playerIndex].Team, team)
				}
			}
		})
	}

	aliases := map[string]Mode{"": ModeDuel, "1v1": ModeDuel, "2v2": ModeTeam2v2, "4V4": ModeTeam4v4, " open ": ModeOpen}
	for value, expected := range aliases {
		actual, err := NormalizeMode(value)
		if err != nil || actual != expected {
			t.Fatalf("NormalizeMode(%q)=%q,%v want %q", value, actual, err, expected)
		}
	}
	if _, err := NormalizeMode("battle_royale"); !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("unknown mode was accepted: %v", err)
	}
}

func TestOpenArenaUsesEveryPlayerAndRequiresEveryEligibleAnswer(t *testing.T) {
	now := time.Date(2026, time.August, 15, 14, 0, 0, 0, time.UTC)
	playerIDs := []int64{11, 22, 33, 44}
	aggregate := mustArena(t, ModeOpen, playerIDs, now)
	commitArenaDecks(t, aggregate, now)

	ownerSnapshot, err := aggregate.SafeSnapshot(11)
	if err != nil || !ownerSnapshot.CanStart || len(ownerSnapshot.StartBlockers) != 0 {
		t.Fatalf("prepared owner could not start: snapshot=%+v err=%v", ownerSnapshot, err)
	}
	guestSnapshot, _ := aggregate.SafeSnapshot(22)
	if guestSnapshot.CanStart || len(guestSnapshot.StartBlockers) == 0 || guestSnapshot.StartBlockers[0] != "not_owner" {
		t.Fatalf("guest received owner start capability: %+v", guestSnapshot)
	}

	changed, err := aggregate.StartWithTieBreak(11, tieBreakQuestions(3), "open-start-0001", now)
	if err != nil || !changed {
		t.Fatalf("start open arena: changed=%v err=%v", changed, err)
	}
	if len(aggregate.Turns) != DeckSize*len(playerIDs) {
		t.Fatalf("turn count=%d want %d", len(aggregate.Turns), DeckSize*len(playerIDs))
	}
	turn := &aggregate.Turns[0]
	if len(turn.EligibleUserIDs) != len(playerIDs) {
		t.Fatalf("eligible players=%v", turn.EligibleUserIDs)
	}
	answerAt := turn.StartedAt.Add(time.Second)
	for index, userID := range playerIDs[:len(playerIDs)-1] {
		if _, err := aggregate.SubmitAnswer(userID, turn.ID, 0, commandID(300+index), answerAt); err != nil {
			t.Fatal(err)
		}
	}
	if turn.Status != TurnActive {
		t.Fatal("turn resolved before every eligible player answered")
	}
	if _, err := aggregate.SubmitAnswer(44, turn.ID, 0, "open-answer-0044", answerAt); err != nil {
		t.Fatal(err)
	}
	if turn.Status != TurnResolved {
		t.Fatal("turn did not resolve after every eligible player answered")
	}
}

func TestTeamTieBreakFindsWinningTeamThenOneChampion(t *testing.T) {
	now := time.Date(2026, time.August, 15, 15, 0, 0, 0, time.UTC)
	playerIDs := []int64{11, 22, 33, 44}
	aggregate := mustArena(t, ModeTeam2v2, playerIDs, now)
	commitArenaDecks(t, aggregate, now)
	if _, err := aggregate.StartWithTieBreak(11, tieBreakQuestions(4), "team-start-0001", now); err != nil {
		t.Fatal(err)
	}

	mainEnd := now.Add(time.Duration(DeckSize*len(playerIDs)) * (TurnDuration + RevealDuration))
	if !aggregate.Tick(mainEnd) || aggregate.Status != StatusTieBreak || aggregate.TieBreak.Phase != TieBreakTeams {
		t.Fatalf("team tie-break did not open: status=%s tieBreak=%+v", aggregate.Status, aggregate.TieBreak)
	}
	firstTieBreak := &aggregate.Turns[aggregate.CurrentTurn]
	if firstTieBreak.Kind != TurnTieBreak || len(firstTieBreak.EligibleUserIDs) != 4 {
		t.Fatalf("unexpected team tie-break turn: %+v", firstTieBreak)
	}
	answerAt := firstTieBreak.StartedAt.Add(10 * time.Second)
	answers := map[int64]int{11: 0, 22: 1, 33: 0, 44: 1}
	for index, userID := range playerIDs {
		if _, err := aggregate.SubmitAnswer(userID, firstTieBreak.ID, answers[userID], commandID(400+index), answerAt); err != nil {
			t.Fatal(err)
		}
	}
	aggregate.Tick(firstTieBreak.RevealUntil)
	if aggregate.WinnerTeam != 1 || aggregate.TieBreak.Phase != TieBreakChampion {
		t.Fatalf("team winner/champion phase mismatch: winnerTeam=%d tieBreak=%+v", aggregate.WinnerTeam, aggregate.TieBreak)
	}
	championTurn := &aggregate.Turns[aggregate.CurrentTurn]
	if len(championTurn.EligibleUserIDs) != 2 || !containsID(championTurn.EligibleUserIDs, 11) || !containsID(championTurn.EligibleUserIDs, 33) {
		t.Fatalf("champion contest was not limited to the winning team leaders: %+v", championTurn.EligibleUserIDs)
	}
	if _, err := aggregate.SubmitAnswer(22, championTurn.ID, 0, "ineligible-answer-22", championTurn.StartedAt.Add(time.Second)); !errors.Is(err, ErrNotEligible) {
		t.Fatalf("losing team entered champion tie-break: %v", err)
	}
	if _, err := aggregate.SubmitAnswer(11, championTurn.ID, 0, "champion-answer-11", championTurn.StartedAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.SubmitAnswer(33, championTurn.ID, 1, "champion-answer-33", championTurn.StartedAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	aggregate.Tick(championTurn.RevealUntil)
	if aggregate.Status != StatusCompleted || aggregate.IsTie || aggregate.WinnerTeam != 1 || aggregate.WinnerID != 11 {
		t.Fatalf("unexpected final team result: %+v", aggregate)
	}
	if len(aggregate.WinnerIDs) != 2 || !containsID(aggregate.WinnerIDs, 11) || !containsID(aggregate.WinnerIDs, 33) {
		t.Fatalf("winning team members were not retained: %v", aggregate.WinnerIDs)
	}
	rewards := aggregate.Rewards()
	if len(rewards) != 4 || rewards[11] != 120 || rewards[33] != 90 || rewards[22] != 45 || rewards[44] != 45 {
		t.Fatalf("unexpected dynamic team rewards: %#v", rewards)
	}
	snapshot, err := aggregate.SafeSnapshot(11)
	if err != nil || snapshot.Mode != ModeTeam2v2 || snapshot.WinnerTeam != 1 || snapshot.WinnerID != 11 || len(snapshot.WinnerIDs) != 2 || len(snapshot.TeamScores) != 2 || snapshot.TieBreak.Round != 2 {
		t.Fatalf("new result fields missing from snapshot: %+v err=%v", snapshot, err)
	}
}

func TestTieBreakCanBeRefilledUntilThereIsOneWinner(t *testing.T) {
	now := time.Date(2026, time.August, 15, 16, 0, 0, 0, time.UTC)
	aggregate := mustArena(t, ModeOpen, []int64{11, 22}, now)
	commitArenaDecks(t, aggregate, now)
	questions := tieBreakQuestions(2)
	if _, err := aggregate.StartWithTieBreak(11, questions[:1], "refill-start-001", now); err != nil {
		t.Fatal(err)
	}
	mainEnd := now.Add(time.Duration(DeckSize*len(aggregate.Players)) * (TurnDuration + RevealDuration))
	aggregate.Tick(mainEnd)
	turn := &aggregate.Turns[aggregate.CurrentTurn]
	answerAt := turn.StartedAt.Add(time.Second)
	for index, userID := range []int64{11, 22} {
		if _, err := aggregate.SubmitAnswer(userID, turn.ID, 1, commandID(500+index), answerAt); err != nil {
			t.Fatal(err)
		}
	}
	aggregate.Tick(turn.RevealUntil)
	if aggregate.Status != StatusTieBreak || !aggregate.TieBreak.AwaitingQuestion {
		t.Fatalf("exhausted pool did not pause safely: %+v", aggregate.TieBreak)
	}
	if changed, err := aggregate.AddTieBreakQuestions(questions[1:], turn.RevealUntil.Add(time.Second)); err != nil || !changed {
		t.Fatalf("refill: changed=%v err=%v", changed, err)
	}
	finalTurn := &aggregate.Turns[aggregate.CurrentTurn]
	if _, err := aggregate.SubmitAnswer(11, finalTurn.ID, 0, "refill-winner-11", finalTurn.StartedAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate.SubmitAnswer(22, finalTurn.ID, 1, "refill-loser-022", finalTurn.StartedAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	aggregate.Tick(finalTurn.RevealUntil)
	if aggregate.Status != StatusCompleted || aggregate.WinnerID != 11 || aggregate.IsTie {
		t.Fatalf("refilled tie-break did not produce one winner: %+v", aggregate)
	}
}

func TestMultiplayerForfeitReturnsZeroRewardForEveryone(t *testing.T) {
	now := time.Date(2026, time.August, 15, 17, 0, 0, 0, time.UTC)
	aggregate := mustArena(t, ModeOpen, []int64{11, 22, 33, 44}, now)
	if changed, err := aggregate.Forfeit(33, "open-forfeit-33", now.Add(time.Minute)); !errors.Is(err, ErrNotOwner) || changed {
		t.Fatalf("non-owner cancelled multiplayer arena: changed=%v err=%v", changed, err)
	}
	if _, err := aggregate.Forfeit(11, "open-owner-forfeit", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	rewards := aggregate.Rewards()
	if aggregate.Status != StatusForfeited || aggregate.WinnerID != 0 || len(rewards) != 4 {
		t.Fatalf("unexpected multiplayer forfeit: aggregate=%+v rewards=%v", aggregate, rewards)
	}
	for _, reward := range rewards {
		if reward != 0 {
			t.Fatalf("forfeit minted reward: %v", rewards)
		}
	}
}

func mustMatch(t *testing.T, now time.Time) *Aggregate {
	t.Helper()
	aggregate, err := New(77, 88, 11, 22, now)
	if err != nil {
		t.Fatal(err)
	}
	return aggregate
}

func commitBothDecks(t *testing.T, aggregate *Aggregate, now time.Time) {
	t.Helper()
	if len(aggregate.Players[0].Deck) == 0 {
		if _, err := aggregate.CommitDeck(11, deck(11, 100), "deck-a-0001", now); err != nil {
			t.Fatal(err)
		}
	}
	if len(aggregate.Players[1].Deck) == 0 {
		if _, err := aggregate.CommitDeck(22, deck(22, 200), "deck-b-0001", now); err != nil {
			t.Fatal(err)
		}
	}
}

func mustArena(t *testing.T, mode Mode, playerIDs []int64, now time.Time) *Aggregate {
	t.Helper()
	aggregate, err := NewArena(707, 808, playerIDs[0], mode, playerIDs, now)
	if err != nil {
		t.Fatal(err)
	}
	return aggregate
}

func commitArenaDecks(t *testing.T, aggregate *Aggregate, now time.Time) {
	t.Helper()
	for index, player := range aggregate.Players {
		if _, err := aggregate.CommitDeck(player.UserID, deck(player.UserID, int64(100+index*10)), commandID(600+index), now); err != nil {
			t.Fatalf("commit player %d: %v", player.UserID, err)
		}
	}
}

func tieBreakQuestions(count int) []QuestionSnapshot {
	result := make([]QuestionSnapshot, 0, count)
	for index := 0; index < count; index++ {
		result = append(result, QuestionSnapshot{
			ID: "tie-break-question-" + commandID(index), Prompt: "سؤال فاصل جديد؟",
			Options: []string{"أ", "ب", "ج", "د"}, CorrectOption: 0,
			Explanation: "شرح السؤال الفاصل", Category: "فاصل", Difficulty: "medium", ContentHash: "tie-hash-" + commandID(index),
		})
	}
	return result
}

func deck(ownerID, firstID int64) []CardSnapshot {
	result := make([]CardSnapshot, 0, DeckSize)
	for index := 0; index < DeckSize; index++ {
		result = append(result, CardSnapshot{
			ID: ownerID*1000 + firstID + int64(index), OwnerID: ownerID, Rarity: "common", Power: 1,
			Question: QuestionSnapshot{
				ID: "q-" + commandID(index), Prompt: "ما الإجابة الصحيحة؟",
				Options: []string{"أ", "ب", "ج", "د"}, CorrectOption: 0,
				Explanation: "شرح موثّق", Category: "علوم", Difficulty: "easy", ContentHash: "hash",
			},
		})
	}
	return result
}

func duplicateDeck(ownerID int64) []CardSnapshot {
	result := deck(ownerID, 1)
	result[4] = result[0]
	return result
}

func commandID(index int) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	value := index
	buffer := []byte("command-")
	if value == 0 {
		return "command-0"
	}
	for value > 0 {
		buffer = append(buffer, digits[value%len(digits)])
		value /= len(digits)
	}
	return string(buffer)
}
