package match

import (
	"sort"
	"strconv"
	"time"
)

type PlayerSnapshot struct {
	UserID      int64       `json:"userId,string"`
	Team        int         `json:"team,omitempty"`
	IsBot       bool        `json:"isBot"`
	BotStrategy BotStrategy `json:"botStrategy,omitempty"`
	Score       int         `json:"score"`
	DeckReady   bool        `json:"deckReady"`
	DeckCardIDs []string    `json:"deckCardIds,omitempty"`
}

type TeamScoreSnapshot struct {
	Team  int `json:"team"`
	Score int `json:"score"`
}

type TieBreakSnapshot struct {
	Enabled            bool     `json:"enabled"`
	Active             bool     `json:"active"`
	Phase              string   `json:"phase,omitempty"`
	Round              int      `json:"round"`
	ContenderIDs       []string `json:"contenderIds,omitempty"`
	ContenderTeams     []int    `json:"contenderTeams,omitempty"`
	AwaitingQuestion   bool     `json:"awaitingQuestion"`
	RemainingQuestions int      `json:"remainingQuestions"`
}

type AnswerSnapshot struct {
	UserID      int64     `json:"userId,string"`
	Option      int       `json:"option"`
	SubmittedAt time.Time `json:"submittedAt"`
	Correct     bool      `json:"correct"`
	Points      int       `json:"points"`
}

type TurnSnapshot struct {
	ID              string           `json:"id"`
	Number          int              `json:"number"`
	Round           int              `json:"round"`
	Kind            TurnKind         `json:"kind"`
	TieBreakRound   int              `json:"tieBreakRound,omitempty"`
	EligibleUserIDs []string         `json:"eligibleUserIds"`
	CanAnswer       bool             `json:"canAnswer"`
	Status          TurnStatus       `json:"status"`
	CardID          int64            `json:"cardId,omitempty,string"`
	CardOwnerID     int64            `json:"cardOwnerId,omitempty,string"`
	Rarity          string           `json:"rarity,omitempty"`
	Power           int              `json:"power,omitempty"`
	QuestionID      string           `json:"questionId"`
	Prompt          string           `json:"prompt"`
	Options         []string         `json:"options"`
	Category        string           `json:"category"`
	Difficulty      string           `json:"difficulty"`
	StartedAt       time.Time        `json:"startedAt"`
	Deadline        time.Time        `json:"deadline"`
	RevealUntil     time.Time        `json:"revealUntil,omitempty"`
	AnsweredUserIDs []string         `json:"answeredUserIds"`
	YourOption      *int             `json:"yourOption,omitempty"`
	CorrectOption   *int             `json:"correctOption,omitempty"`
	Explanation     string           `json:"explanation,omitempty"`
	Answers         []AnswerSnapshot `json:"answers,omitempty"`
}

type Snapshot struct {
	ID             int64               `json:"id,string"`
	GameID         int64               `json:"gameId,string"`
	OwnerID        int64               `json:"ownerId,string"`
	Mode           Mode                `json:"mode"`
	Status         Status              `json:"status"`
	Version        int64               `json:"version"`
	Players        []PlayerSnapshot    `json:"players"`
	TeamScores     []TeamScoreSnapshot `json:"teamScores"`
	TieBreak       TieBreakSnapshot    `json:"tieBreak"`
	CurrentTurn    *TurnSnapshot       `json:"currentTurn,omitempty"`
	TurnNumber     int                 `json:"turnNumber"`
	TotalTurns     int                 `json:"totalTurns"`
	WinnerID       int64               `json:"winnerId,omitempty,string"`
	WinnerTeam     int                 `json:"winnerTeam,omitempty"`
	WinnerIDs      []string            `json:"winnerIds,omitempty"`
	IsTie          bool                `json:"isTie"`
	CanStart       bool                `json:"canStart"`
	StartBlockers  []string            `json:"startBlockers"`
	StartedAt      time.Time           `json:"startedAt,omitempty"`
	CompletedAt    time.Time           `json:"completedAt,omitempty"`
	RewardCoins    int64               `json:"rewardCoins,omitempty"`
	Reward         *RewardReceipt      `json:"reward,omitempty"`
	RewardsSettled bool                `json:"rewardsSettled"`
}

// SafeSnapshot exposes only information the authenticated viewer may know.
// Correct answers, explanations, and submitted options from other contestants
// remain hidden until the current turn is resolved.
func (aggregate *Aggregate) SafeSnapshot(viewerID int64) (Snapshot, error) {
	viewer := aggregate.player(viewerID)
	if viewer == nil || viewer.IsBot() {
		return Snapshot{}, ErrNotPlayer
	}
	blockers := aggregate.StartBlockers(viewerID)
	result := Snapshot{
		ID:             aggregate.ID,
		GameID:         aggregate.GameID,
		OwnerID:        aggregate.OwnerID,
		Mode:           aggregate.effectiveMode(),
		Status:         aggregate.Status,
		Version:        aggregate.Version,
		TurnNumber:     aggregate.CurrentTurn + 1,
		TotalTurns:     len(aggregate.Turns),
		WinnerID:       aggregate.WinnerID,
		WinnerTeam:     aggregate.WinnerTeam,
		IsTie:          aggregate.IsTie,
		CanStart:       len(blockers) == 0,
		StartBlockers:  blockers,
		StartedAt:      aggregate.StartedAt,
		CompletedAt:    aggregate.CompletedAt,
		RewardsSettled: aggregate.RewardsSettled,
		Players:        make([]PlayerSnapshot, 0, len(aggregate.Players)),
		TeamScores:     make([]TeamScoreSnapshot, 0),
		TieBreak: TieBreakSnapshot{
			Enabled:            aggregate.TieBreak.Enabled,
			Active:             aggregate.TieBreak.Active,
			Phase:              string(aggregate.TieBreak.Phase),
			Round:              aggregate.TieBreak.Round,
			ContenderTeams:     append([]int(nil), aggregate.TieBreak.ContenderTeams...),
			AwaitingQuestion:   aggregate.TieBreak.AwaitingQuestion,
			RemainingQuestions: max(0, len(aggregate.TieBreak.QuestionPool)-aggregate.TieBreak.NextQuestion),
		},
	}
	for _, userID := range aggregate.WinnerIDs {
		result.WinnerIDs = append(result.WinnerIDs, strconv.FormatInt(userID, 10))
	}
	for _, userID := range aggregate.TieBreak.ContenderIDs {
		result.TieBreak.ContenderIDs = append(result.TieBreak.ContenderIDs, strconv.FormatInt(userID, 10))
	}
	for _, player := range aggregate.Players {
		playerView := PlayerSnapshot{
			UserID: player.UserID, Team: player.Team, IsBot: player.IsBot(), Score: player.Score, DeckReady: len(player.Deck) == DeckSize,
		}
		if player.IsBot() && player.Bot != nil {
			playerView.BotStrategy = player.Bot.Strategy
		}
		if player.UserID == viewerID {
			for _, card := range player.Deck {
				playerView.DeckCardIDs = append(playerView.DeckCardIDs, strconv.FormatInt(card.ID, 10))
			}
		}
		result.Players = append(result.Players, playerView)
	}
	teamScores := aggregate.teamScores()
	teams := make([]int, 0, len(teamScores))
	for team := range teamScores {
		teams = append(teams, team)
	}
	sort.Ints(teams)
	for _, team := range teams {
		result.TeamScores = append(result.TeamScores, TeamScoreSnapshot{Team: team, Score: teamScores[team]})
	}
	if receipt := aggregate.RewardFor(viewerID); receipt != nil {
		result.Reward = receipt
		result.RewardCoins = receipt.CoinsGranted
	} else if aggregate.RewardPolicyVersion == "" {
		// Old persisted matches predate durable receipts and keep their original
		// coin-only projection. New policies never expose an uncommitted estimate.
		if rewards := aggregate.Rewards(); rewards != nil {
			result.RewardCoins = rewards[viewerID]
		}
	}
	if aggregate.CurrentTurn < 0 || aggregate.CurrentTurn >= len(aggregate.Turns) {
		return result, nil
	}
	turn := aggregate.Turns[aggregate.CurrentTurn]
	question := aggregate.questionFor(&turn)
	kind := turn.Kind
	if kind == "" {
		kind = TurnMain
	}
	view := &TurnSnapshot{
		ID:            turn.ID,
		Number:        turn.Number,
		Round:         turn.Round,
		Kind:          kind,
		TieBreakRound: turn.TieBreakRound,
		Status:        turn.Status,
		CardID:        turn.Card.ID,
		CardOwnerID:   turn.Card.OwnerID,
		Rarity:        turn.Card.Rarity,
		Power:         turn.Card.Power,
		QuestionID:    question.ID,
		Prompt:        question.Prompt,
		Options:       append([]string(nil), question.Options...),
		Category:      question.Category,
		Difficulty:    question.Difficulty,
		StartedAt:     turn.StartedAt,
		Deadline:      turn.Deadline,
		RevealUntil:   turn.RevealUntil,
	}
	for _, userID := range aggregate.eligibleFor(&turn) {
		view.EligibleUserIDs = append(view.EligibleUserIDs, strconv.FormatInt(userID, 10))
	}
	view.CanAnswer = turn.Status == TurnActive && containsID(aggregate.eligibleFor(&turn), viewerID)
	answers := append([]Answer(nil), turn.Answers...)
	sort.Slice(answers, func(left, right int) bool { return answers[left].UserID < answers[right].UserID })
	for _, answer := range answers {
		userID := answer.UserID
		view.AnsweredUserIDs = append(view.AnsweredUserIDs, strconv.FormatInt(userID, 10))
		if userID == viewerID {
			selected := answer.Option
			view.YourOption = &selected
			view.CanAnswer = false
		}
		if turn.Status == TurnResolved {
			view.Answers = append(view.Answers, AnswerSnapshot(answer))
		}
	}
	if turn.Status == TurnResolved {
		correct := question.CorrectOption
		view.CorrectOption = &correct
		view.Explanation = question.Explanation
	}
	result.CurrentTurn = view
	return result, nil
}
