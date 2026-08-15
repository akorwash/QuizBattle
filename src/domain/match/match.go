package match

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	DeckSize = 5
	// TurnCount is kept as the legacy duel turn count. New arenas derive their
	// main turn count from DeckSize * len(Players).
	TurnCount              = DeckSize * 2
	TurnDuration           = 20 * time.Second
	RevealDuration         = 3 * time.Second
	maximumProcessed       = 256
	minimumCommandIDLength = 8
	maximumCommandIDLength = 128
)

type Mode string

const (
	ModeDuel    Mode = "duel"
	ModeTeam2v2 Mode = "team_2v2"
	ModeTeam4v4 Mode = "team_4v4"
	ModeOpen    Mode = "open"
)

func NormalizeMode(value string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "duel", "1v1":
		return ModeDuel, nil
	case "team_2v2", "2v2":
		return ModeTeam2v2, nil
	case "team_4v4", "4v4":
		return ModeTeam4v4, nil
	case "open":
		return ModeOpen, nil
	default:
		return "", ErrInvalidMode
	}
}

func MaxPlayers(mode Mode) int {
	mode, err := NormalizeMode(string(mode))
	if err != nil {
		return 0
	}
	switch mode {
	case ModeDuel:
		return 2
	case ModeTeam2v2:
		return 4
	case ModeTeam4v4, ModeOpen:
		return 8
	default:
		return 0
	}
}

func MinPlayers(mode Mode) int {
	mode, err := NormalizeMode(string(mode))
	if err != nil {
		return 0
	}
	switch mode {
	case ModeDuel, ModeOpen:
		return 2
	case ModeTeam2v2:
		return 4
	case ModeTeam4v4:
		return 8
	default:
		return 0
	}
}

func TeamSize(mode Mode) int {
	mode, err := NormalizeMode(string(mode))
	if err != nil {
		return 0
	}
	switch mode {
	case ModeDuel:
		return 1
	case ModeTeam2v2:
		return 2
	case ModeTeam4v4:
		return 4
	default:
		return 0
	}
}

func isTeamMode(mode Mode) bool {
	return mode == ModeTeam2v2 || mode == ModeTeam4v4
}

type Status string

const (
	StatusCollectingDecks Status = "collecting_decks"
	StatusActive          Status = "active"
	StatusTieBreak        Status = "tie_break"
	StatusCompleted       Status = "completed"
	StatusForfeited       Status = "forfeited"
)

type TurnStatus string

const (
	TurnPending  TurnStatus = "pending"
	TurnActive   TurnStatus = "active"
	TurnResolved TurnStatus = "resolved"
)

type TurnKind string

const (
	TurnMain     TurnKind = "main"
	TurnTieBreak TurnKind = "tie_break"
)

type TieBreakPhase string

const (
	TieBreakNone     TieBreakPhase = ""
	TieBreakTeams    TieBreakPhase = "teams"
	TieBreakChampion TieBreakPhase = "champion"
)

var (
	ErrInvalidMatch          = errors.New("invalid match")
	ErrInvalidMode           = errors.New("invalid arena mode")
	ErrNotPlayer             = errors.New("user is not a match player")
	ErrNotEligible           = errors.New("player is not eligible for this turn")
	ErrNotOwner              = errors.New("only the lobby owner can perform this action")
	ErrInvalidDeck           = errors.New("deck must contain five distinct owned cards")
	ErrDecksNotReady         = errors.New("all player decks must be committed")
	ErrInvalidState          = errors.New("command is not valid in the current match state")
	ErrInvalidCommandID      = errors.New("invalid command ID")
	ErrInvalidTurn           = errors.New("turn is not active")
	ErrTurnClosed            = errors.New("turn deadline has passed")
	ErrAlreadyAnswered       = errors.New("player already answered this turn")
	ErrInvalidOption         = errors.New("answer option must be between zero and three")
	ErrInvalidTieBreakPool   = errors.New("tie-break pool must contain new, distinct, valid questions")
	ErrTieBreakPoolExhausted = errors.New("tie-break needs more questions")
)

type QuestionSnapshot struct {
	ID            string   `bson:"id" json:"id"`
	Prompt        string   `bson:"prompt" json:"prompt"`
	Options       []string `bson:"options" json:"options"`
	CorrectOption int      `bson:"correctOption" json:"-"`
	Explanation   string   `bson:"explanation" json:"-"`
	Category      string   `bson:"category" json:"category"`
	Difficulty    string   `bson:"difficulty" json:"difficulty"`
	ContentHash   string   `bson:"contentHash" json:"contentHash"`
}

type CardSnapshot struct {
	ID       int64            `bson:"id" json:"id,string"`
	OwnerID  int64            `bson:"ownerId" json:"ownerId,string"`
	Rarity   string           `bson:"rarity" json:"rarity"`
	Power    int              `bson:"power" json:"power"`
	Question QuestionSnapshot `bson:"question" json:"question"`
}

type Answer struct {
	UserID      int64     `bson:"userId" json:"userId,string"`
	Option      int       `bson:"option" json:"option"`
	SubmittedAt time.Time `bson:"submittedAt" json:"submittedAt"`
	Correct     bool      `bson:"correct" json:"correct"`
	Points      int       `bson:"points" json:"points"`
}

type Turn struct {
	ID              string            `bson:"id" json:"id"`
	Number          int               `bson:"number" json:"number"`
	Round           int               `bson:"round" json:"round"`
	Kind            TurnKind          `bson:"kind,omitempty" json:"kind,omitempty"`
	Card            CardSnapshot      `bson:"card" json:"card"`
	Question        *QuestionSnapshot `bson:"question,omitempty" json:"-"`
	EligibleUserIDs []int64           `bson:"eligibleUserIds,omitempty" json:"-"`
	TieBreakRound   int               `bson:"tieBreakRound,omitempty" json:"tieBreakRound,omitempty"`
	Status          TurnStatus        `bson:"status" json:"status"`
	StartedAt       time.Time         `bson:"startedAt,omitempty" json:"startedAt,omitempty"`
	Deadline        time.Time         `bson:"deadline,omitempty" json:"deadline,omitempty"`
	ResolvedAt      time.Time         `bson:"resolvedAt,omitempty" json:"resolvedAt,omitempty"`
	RevealUntil     time.Time         `bson:"revealUntil,omitempty" json:"revealUntil,omitempty"`
	Answers         []Answer          `bson:"answers" json:"-"`
}

type Player struct {
	UserID int64          `bson:"userId" json:"userId,string"`
	Team   int            `bson:"team,omitempty" json:"team,omitempty"`
	Deck   []CardSnapshot `bson:"deck" json:"-"`
	Score  int            `bson:"score" json:"score"`
}

type ProcessedCommand struct {
	ID        string    `bson:"id" json:"id"`
	UserID    int64     `bson:"userId" json:"userId,string"`
	Action    string    `bson:"action" json:"action"`
	AppliedAt time.Time `bson:"appliedAt" json:"appliedAt"`
}

// TieBreakState keeps the neutral question pool server-side. A tie-break first
// resolves a tied team result, then (inside the winning team) resolves one
// individual champion. Duel/open arenas go directly to the champion phase.
type TieBreakState struct {
	Enabled          bool               `bson:"enabled" json:"enabled"`
	Active           bool               `bson:"active" json:"active"`
	Phase            TieBreakPhase      `bson:"phase,omitempty" json:"phase,omitempty"`
	Round            int                `bson:"round" json:"round"`
	ContenderIDs     []int64            `bson:"contenderIds,omitempty" json:"-"`
	ContenderTeams   []int              `bson:"contenderTeams,omitempty" json:"contenderTeams,omitempty"`
	QuestionPool     []QuestionSnapshot `bson:"questionPool,omitempty" json:"-"`
	NextQuestion     int                `bson:"nextQuestion" json:"-"`
	AwaitingQuestion bool               `bson:"awaitingQuestion" json:"awaitingQuestion"`
}

type Aggregate struct {
	ID                int64              `bson:"id" json:"id,string"`
	GameID            int64              `bson:"gameId" json:"gameId,string"`
	OwnerID           int64              `bson:"ownerId" json:"ownerId,string"`
	Mode              Mode               `bson:"mode,omitempty" json:"mode,omitempty"`
	Players           []Player           `bson:"players" json:"players"`
	Status            Status             `bson:"status" json:"status"`
	Turns             []Turn             `bson:"turns" json:"-"`
	CurrentTurn       int                `bson:"currentTurn" json:"currentTurn"`
	WinnerID          int64              `bson:"winnerId,omitempty" json:"winnerId,omitempty,string"`
	WinnerTeam        int                `bson:"winnerTeam,omitempty" json:"winnerTeam,omitempty"`
	WinnerIDs         []int64            `bson:"winnerIds,omitempty" json:"-"`
	IsTie             bool               `bson:"isTie" json:"isTie"`
	TieBreak          TieBreakState      `bson:"tieBreak,omitempty" json:"-"`
	Version           int64              `bson:"version" json:"version"`
	CreatedAt         time.Time          `bson:"createdAt" json:"createdAt"`
	StartedAt         time.Time          `bson:"startedAt,omitempty" json:"startedAt,omitempty"`
	CompletedAt       time.Time          `bson:"completedAt,omitempty" json:"completedAt,omitempty"`
	RewardsSettled    bool               `bson:"rewardsSettled" json:"rewardsSettled"`
	ProcessedCommands []ProcessedCommand `bson:"processedCommands" json:"-"`
}

// New preserves the original two-player construction contract.
func New(id, gameID, ownerID, guestID int64, now time.Time) (*Aggregate, error) {
	return NewArena(id, gameID, ownerID, ModeDuel, []int64{ownerID, guestID}, now)
}

// NewArena freezes the prepared lobby roster. Fixed team modes require their
// full roster; open arenas allow any prepared roster from two through eight.
func NewArena(id, gameID, ownerID int64, mode Mode, playerIDs []int64, now time.Time) (*Aggregate, error) {
	mode, err := NormalizeMode(string(mode))
	if err != nil {
		return nil, err
	}
	if id <= 0 || gameID <= 0 || ownerID <= 0 || now.IsZero() {
		return nil, ErrInvalidMatch
	}
	orderedIDs, err := normalizeRoster(ownerID, playerIDs)
	if err != nil || len(orderedIDs) < MinPlayers(mode) || len(orderedIDs) > MaxPlayers(mode) {
		return nil, ErrInvalidMatch
	}
	if mode != ModeOpen && len(orderedIDs) != MaxPlayers(mode) {
		return nil, ErrInvalidMatch
	}
	players := make([]Player, 0, len(orderedIDs))
	for index, userID := range orderedIDs {
		team := 0
		switch mode {
		case ModeTeam2v2, ModeTeam4v4:
			team = index%2 + 1
		}
		players = append(players, Player{UserID: userID, Team: team})
	}
	return &Aggregate{
		ID:          id,
		GameID:      gameID,
		OwnerID:     ownerID,
		Mode:        mode,
		Players:     players,
		Status:      StatusCollectingDecks,
		CurrentTurn: -1,
		Version:     1,
		CreatedAt:   now.UTC(),
	}, nil
}

func (aggregate *Aggregate) CommitDeck(userID int64, cards []CardSnapshot, commandID string, now time.Time) (bool, error) {
	if aggregate.commandSeen(commandID, userID, "commit_deck") {
		return false, nil
	}
	if err := validateCommandID(commandID); err != nil {
		return false, err
	}
	if aggregate.Status != StatusCollectingDecks {
		return false, ErrInvalidState
	}
	player := aggregate.player(userID)
	if player == nil {
		return false, ErrNotPlayer
	}
	if err := validateDeck(userID, cards); err != nil {
		return false, err
	}
	for _, candidate := range cards {
		for index := range aggregate.Players {
			if aggregate.Players[index].UserID == userID {
				continue
			}
			for _, committed := range aggregate.Players[index].Deck {
				if committed.ID == candidate.ID {
					return false, ErrInvalidDeck
				}
			}
		}
	}
	player.Deck = cloneCards(cards)
	aggregate.recordCommand(commandID, userID, "commit_deck", now)
	aggregate.Version++
	return true, nil
}

// Start preserves legacy callers. It can still end in a draw because no
// neutral question pool was supplied. New gameplay should call
// StartWithTieBreak so a draw opens a smaller contest.
func (aggregate *Aggregate) Start(userID int64, commandID string, now time.Time) (bool, error) {
	return aggregate.start(userID, nil, commandID, now, false)
}

func (aggregate *Aggregate) StartWithTieBreak(userID int64, questions []QuestionSnapshot, commandID string, now time.Time) (bool, error) {
	return aggregate.start(userID, questions, commandID, now, true)
}

func (aggregate *Aggregate) start(userID int64, questions []QuestionSnapshot, commandID string, now time.Time, requireTieBreak bool) (bool, error) {
	if aggregate.commandSeen(commandID, userID, "start") {
		return false, nil
	}
	if err := validateCommandID(commandID); err != nil {
		return false, err
	}
	if aggregate.Status != StatusCollectingDecks {
		return false, ErrInvalidState
	}
	if userID != aggregate.OwnerID {
		return false, ErrNotOwner
	}
	if !aggregate.validRoster() {
		return false, ErrInvalidMatch
	}
	for _, player := range aggregate.Players {
		if len(player.Deck) != DeckSize {
			return false, ErrDecksNotReady
		}
	}
	if requireTieBreak {
		if err := aggregate.validateTieBreakQuestions(questions); err != nil {
			return false, err
		}
		aggregate.TieBreak = TieBreakState{Enabled: true, QuestionPool: cloneQuestions(questions)}
	} else {
		aggregate.TieBreak = TieBreakState{}
	}

	totalTurns := DeckSize * len(aggregate.Players)
	aggregate.Turns = make([]Turn, 0, totalTurns+len(questions))
	eligible := aggregate.allPlayerIDs()
	for cardIndex := 0; cardIndex < DeckSize; cardIndex++ {
		for playerIndex := range aggregate.Players {
			number := len(aggregate.Turns) + 1
			aggregate.Turns = append(aggregate.Turns, Turn{
				ID:              fmt.Sprintf("%d-%02d", aggregate.ID, number),
				Number:          number,
				Round:           cardIndex + 1,
				Kind:            TurnMain,
				Card:            cloneCard(aggregate.Players[playerIndex].Deck[cardIndex]),
				EligibleUserIDs: append([]int64(nil), eligible...),
				Status:          TurnPending,
				Answers:         make([]Answer, 0, len(eligible)),
			})
		}
	}
	aggregate.Status = StatusActive
	aggregate.StartedAt = now.UTC()
	aggregate.CurrentTurn = 0
	aggregate.startTurn(&aggregate.Turns[0], now.UTC())
	aggregate.recordCommand(commandID, userID, "start", now)
	aggregate.Version++
	return true, nil
}

// AddTieBreakQuestions resumes a tie-break whose original neutral pool was
// exhausted. The service must persist the changed aggregate with its normal
// optimistic-version check.
func (aggregate *Aggregate) AddTieBreakQuestions(questions []QuestionSnapshot, now time.Time) (bool, error) {
	if aggregate.Status != StatusTieBreak || !aggregate.TieBreak.Enabled || !aggregate.TieBreak.AwaitingQuestion || now.IsZero() {
		return false, ErrInvalidState
	}
	if err := aggregate.validateTieBreakQuestions(questions); err != nil {
		return false, err
	}
	aggregate.TieBreak.QuestionPool = append(aggregate.TieBreak.QuestionPool, cloneQuestions(questions)...)
	if !aggregate.scheduleTieBreak(now.UTC()) {
		return false, ErrTieBreakPoolExhausted
	}
	aggregate.Version++
	return true, nil
}

// Forfeit ends the entire prepared/active contest and deliberately awards zero
// coins to every participant. Non-duel arenas can only be cancelled by their
// owner so one participant cannot destroy a team/open contest for everyone.
// For a legacy duel, the remaining player is kept as the recorded battle
// winner for backward-compatible history only.
func (aggregate *Aggregate) Forfeit(userID int64, commandID string, now time.Time) (bool, error) {
	if aggregate.commandSeen(commandID, userID, "forfeit") {
		return false, nil
	}
	if err := validateCommandID(commandID); err != nil {
		return false, err
	}
	if now.IsZero() || aggregate.player(userID) == nil {
		return false, ErrNotPlayer
	}
	if aggregate.effectiveMode() != ModeDuel && userID != aggregate.OwnerID {
		return false, ErrNotOwner
	}
	if aggregate.Status != StatusCollectingDecks && aggregate.Status != StatusActive && aggregate.Status != StatusTieBreak {
		return false, ErrInvalidState
	}
	aggregate.WinnerID = 0
	aggregate.WinnerTeam = 0
	aggregate.WinnerIDs = nil
	if len(aggregate.Players) == 2 {
		for _, player := range aggregate.Players {
			if player.UserID != userID {
				aggregate.WinnerID = player.UserID
				aggregate.WinnerIDs = []int64{player.UserID}
				break
			}
		}
	}
	aggregate.Status = StatusForfeited
	aggregate.IsTie = false
	aggregate.TieBreak.Active = false
	aggregate.TieBreak.AwaitingQuestion = false
	aggregate.CompletedAt = now.UTC()
	aggregate.recordCommand(commandID, userID, "forfeit", now)
	aggregate.Version++
	return true, nil
}

func (aggregate *Aggregate) SubmitAnswer(userID int64, turnID string, option int, commandID string, now time.Time) (bool, error) {
	if aggregate.commandSeen(commandID, userID, "answer") {
		return false, nil
	}
	if err := validateCommandID(commandID); err != nil {
		return false, err
	}
	if aggregate.player(userID) == nil {
		return false, ErrNotPlayer
	}
	if option < 0 || option > 3 {
		return false, ErrInvalidOption
	}
	if !aggregate.playing() || aggregate.CurrentTurn < 0 || aggregate.CurrentTurn >= len(aggregate.Turns) {
		return false, ErrInvalidState
	}
	aggregate.Tick(now)
	if !aggregate.playing() || aggregate.CurrentTurn < 0 || aggregate.CurrentTurn >= len(aggregate.Turns) {
		return false, ErrTurnClosed
	}
	turn := &aggregate.Turns[aggregate.CurrentTurn]
	if turn.ID == turnID && turn.Status == TurnResolved {
		return false, ErrTurnClosed
	}
	if turn.ID != turnID || turn.Status != TurnActive {
		return false, ErrInvalidTurn
	}
	if !containsID(aggregate.eligibleFor(turn), userID) {
		return false, ErrNotEligible
	}
	if !now.Before(turn.Deadline) {
		aggregate.Tick(now)
		return false, ErrTurnClosed
	}
	if _, exists := answerFor(turn.Answers, userID); exists {
		return false, ErrAlreadyAnswered
	}
	question := aggregate.questionFor(turn)
	correct := option == question.CorrectOption
	points := 0
	if correct {
		remaining := turn.Deadline.Sub(now)
		if remaining < 0 {
			remaining = 0
		}
		if remaining > TurnDuration {
			remaining = TurnDuration
		}
		points = 100 + int((50*remaining)/TurnDuration)
	}
	turn.Answers = append(turn.Answers, Answer{
		UserID:      userID,
		Option:      option,
		SubmittedAt: now.UTC(),
		Correct:     correct,
		Points:      points,
	})
	if player := aggregate.player(userID); player != nil {
		player.Score += points
	}
	aggregate.recordCommand(commandID, userID, "answer", now)
	if len(turn.Answers) == len(aggregate.eligibleFor(turn)) {
		aggregate.resolveTurn(turn, now.UTC())
	}
	aggregate.Version++
	return true, nil
}

// Tick resolves deadlines and reveal windows using server time. It can append
// repeated tie-break turns from the prevalidated neutral question pool.
func (aggregate *Aggregate) Tick(now time.Time) bool {
	if !aggregate.playing() || aggregate.CurrentTurn < 0 || aggregate.CurrentTurn >= len(aggregate.Turns) {
		return false
	}
	changed := false
	for aggregate.playing() && aggregate.CurrentTurn >= 0 && aggregate.CurrentTurn < len(aggregate.Turns) {
		turn := &aggregate.Turns[aggregate.CurrentTurn]
		switch turn.Status {
		case TurnActive:
			if now.Before(turn.Deadline) {
				if changed {
					aggregate.Version++
				}
				return changed
			}
			aggregate.resolveTurn(turn, turn.Deadline)
			changed = true
		case TurnResolved:
			if now.Before(turn.RevealUntil) {
				if changed {
					aggregate.Version++
				}
				return changed
			}
			if aggregate.CurrentTurn < len(aggregate.Turns)-1 {
				aggregate.CurrentTurn++
				aggregate.startTurn(&aggregate.Turns[aggregate.CurrentTurn], turn.RevealUntil)
				changed = true
				continue
			}
			if aggregate.finishStage(turn, turn.RevealUntil) {
				changed = true
			}
			// A finite neutral pool may be exhausted after any repeated tie.
			// Keep the aggregate pending for AddTieBreakQuestions, but do not
			// spin forever on the same resolved turn.
			if aggregate.TieBreak.AwaitingQuestion || !aggregate.playing() {
				if changed {
					aggregate.Version++
				}
				return changed
			}
		default:
			return changed
		}
	}
	if changed {
		aggregate.Version++
	}
	return changed
}

func (aggregate *Aggregate) Rewards() map[int64]int64 {
	if len(aggregate.Players) == 0 {
		return nil
	}
	result := make(map[int64]int64, len(aggregate.Players))
	if aggregate.Status == StatusForfeited {
		for _, player := range aggregate.Players {
			result[player.UserID] = 0
		}
		return result
	}
	if aggregate.Status != StatusCompleted {
		return nil
	}
	if aggregate.IsTie {
		for _, player := range aggregate.Players {
			result[player.UserID] = 75
		}
		return result
	}
	winners := make(map[int64]struct{}, len(aggregate.WinnerIDs)+1)
	for _, userID := range aggregate.WinnerIDs {
		winners[userID] = struct{}{}
	}
	if len(winners) == 0 && aggregate.WinnerID > 0 {
		winners[aggregate.WinnerID] = struct{}{}
	}
	if len(winners) == 0 {
		return nil
	}
	mode := aggregate.effectiveMode()
	for _, player := range aggregate.Players {
		_, won := winners[player.UserID]
		switch {
		case player.UserID == aggregate.WinnerID:
			result[player.UserID] = 120
		case isTeamMode(mode) && won:
			result[player.UserID] = 90
		default:
			result[player.UserID] = 45
		}
	}
	return result
}

func (aggregate *Aggregate) StartBlockers(viewerID int64) []string {
	blockers := make([]string, 0)
	if viewerID != aggregate.OwnerID {
		blockers = append(blockers, "not_owner")
	}
	if aggregate.Status != StatusCollectingDecks {
		blockers = append(blockers, "invalid_state")
	}
	if !aggregate.validRoster() {
		blockers = append(blockers, "invalid_roster")
	}
	for _, player := range aggregate.Players {
		if len(player.Deck) != DeckSize {
			blockers = append(blockers, "deck_not_ready:"+fmt.Sprint(player.UserID))
		}
	}
	return blockers
}

func (aggregate *Aggregate) CanStart(viewerID int64) bool {
	return len(aggregate.StartBlockers(viewerID)) == 0
}

func (aggregate *Aggregate) finishStage(turn *Turn, at time.Time) bool {
	if turn.Kind == TurnTieBreak {
		aggregate.resolveTieBreak(at)
		return true
	}
	aggregate.resolveMain(at)
	return true
}

func (aggregate *Aggregate) resolveMain(at time.Time) {
	mode := aggregate.effectiveMode()
	if isTeamMode(mode) {
		teamLeaders := aggregate.leadingTeams([]int{1, 2})
		if len(teamLeaders) > 1 {
			if aggregate.TieBreak.Enabled {
				aggregate.beginTeamTieBreak(teamLeaders, at)
				return
			}
			aggregate.completeLegacyTie(at)
			return
		}
		aggregate.selectWinnerTeam(teamLeaders[0])
		aggregate.resolveChampion(at)
		return
	}
	leaders := aggregate.leadingPlayers(aggregate.allPlayerIDs())
	if len(leaders) == 1 {
		aggregate.WinnerID = leaders[0]
		aggregate.WinnerIDs = []int64{leaders[0]}
		aggregate.completeWinner(at)
		return
	}
	if aggregate.TieBreak.Enabled {
		aggregate.beginChampionTieBreak(leaders, at)
		return
	}
	aggregate.completeLegacyTie(at)
}

func (aggregate *Aggregate) resolveTieBreak(at time.Time) {
	switch aggregate.TieBreak.Phase {
	case TieBreakTeams:
		leaders := aggregate.leadingTeams(aggregate.TieBreak.ContenderTeams)
		if len(leaders) > 1 {
			aggregate.beginTeamTieBreak(leaders, at)
			return
		}
		aggregate.selectWinnerTeam(leaders[0])
		aggregate.resolveChampion(at)
	case TieBreakChampion:
		leaders := aggregate.leadingPlayers(aggregate.TieBreak.ContenderIDs)
		if len(leaders) > 1 {
			aggregate.beginChampionTieBreak(leaders, at)
			return
		}
		aggregate.WinnerID = leaders[0]
		if aggregate.WinnerTeam == 0 {
			aggregate.WinnerIDs = []int64{leaders[0]}
		}
		aggregate.completeWinner(at)
	default:
		aggregate.completeLegacyTie(at)
	}
}

func (aggregate *Aggregate) resolveChampion(at time.Time) {
	candidates := append([]int64(nil), aggregate.WinnerIDs...)
	if len(candidates) == 0 {
		aggregate.completeLegacyTie(at)
		return
	}
	leaders := aggregate.leadingPlayers(candidates)
	if len(leaders) == 1 {
		aggregate.WinnerID = leaders[0]
		aggregate.completeWinner(at)
		return
	}
	if aggregate.TieBreak.Enabled {
		aggregate.beginChampionTieBreak(leaders, at)
		return
	}
	aggregate.completeLegacyTie(at)
}

func (aggregate *Aggregate) beginTeamTieBreak(teams []int, at time.Time) {
	aggregate.TieBreak.Active = true
	aggregate.TieBreak.Phase = TieBreakTeams
	aggregate.TieBreak.ContenderTeams = uniqueSortedTeams(teams)
	aggregate.TieBreak.ContenderIDs = aggregate.playersInTeams(aggregate.TieBreak.ContenderTeams)
	aggregate.scheduleTieBreak(at)
}

func (aggregate *Aggregate) beginChampionTieBreak(players []int64, at time.Time) {
	aggregate.TieBreak.Active = true
	aggregate.TieBreak.Phase = TieBreakChampion
	aggregate.TieBreak.ContenderTeams = nil
	aggregate.TieBreak.ContenderIDs = uniqueSortedIDs(players)
	aggregate.scheduleTieBreak(at)
}

func (aggregate *Aggregate) scheduleTieBreak(at time.Time) bool {
	aggregate.Status = StatusTieBreak
	if aggregate.TieBreak.NextQuestion >= len(aggregate.TieBreak.QuestionPool) {
		aggregate.TieBreak.AwaitingQuestion = true
		return false
	}
	question := cloneQuestion(aggregate.TieBreak.QuestionPool[aggregate.TieBreak.NextQuestion])
	aggregate.TieBreak.NextQuestion++
	aggregate.TieBreak.Round++
	aggregate.TieBreak.AwaitingQuestion = false
	number := len(aggregate.Turns) + 1
	eligible := append([]int64(nil), aggregate.TieBreak.ContenderIDs...)
	aggregate.Turns = append(aggregate.Turns, Turn{
		ID:              fmt.Sprintf("%d-tb-%02d", aggregate.ID, aggregate.TieBreak.Round),
		Number:          number,
		Round:           DeckSize + aggregate.TieBreak.Round,
		Kind:            TurnTieBreak,
		Question:        &question,
		EligibleUserIDs: eligible,
		TieBreakRound:   aggregate.TieBreak.Round,
		Status:          TurnPending,
		Answers:         make([]Answer, 0, len(eligible)),
	})
	aggregate.CurrentTurn = len(aggregate.Turns) - 1
	aggregate.startTurn(&aggregate.Turns[aggregate.CurrentTurn], at)
	return true
}

func (aggregate *Aggregate) selectWinnerTeam(team int) {
	aggregate.WinnerTeam = team
	aggregate.WinnerIDs = aggregate.playersInTeams([]int{team})
}

func (aggregate *Aggregate) completeWinner(at time.Time) {
	aggregate.Status = StatusCompleted
	aggregate.CompletedAt = at.UTC()
	aggregate.IsTie = false
	aggregate.TieBreak.Active = false
	aggregate.TieBreak.AwaitingQuestion = false
}

func (aggregate *Aggregate) completeLegacyTie(at time.Time) {
	aggregate.Status = StatusCompleted
	aggregate.CompletedAt = at.UTC()
	aggregate.WinnerID = 0
	aggregate.WinnerTeam = 0
	aggregate.WinnerIDs = nil
	aggregate.IsTie = true
	aggregate.TieBreak.Active = false
	aggregate.TieBreak.AwaitingQuestion = false
}

func (aggregate *Aggregate) leadingPlayers(candidateIDs []int64) []int64 {
	leaders := make([]int64, 0, len(candidateIDs))
	best := 0
	haveBest := false
	for _, userID := range uniqueSortedIDs(candidateIDs) {
		player := aggregate.player(userID)
		if player == nil {
			continue
		}
		if !haveBest || player.Score > best {
			best = player.Score
			leaders = []int64{userID}
			haveBest = true
		} else if player.Score == best {
			leaders = append(leaders, userID)
		}
	}
	return leaders
}

func (aggregate *Aggregate) leadingTeams(candidateTeams []int) []int {
	totals := aggregate.teamScores()
	leaders := make([]int, 0, len(candidateTeams))
	best := 0
	haveBest := false
	for _, team := range uniqueSortedTeams(candidateTeams) {
		score, exists := totals[team]
		if !exists {
			continue
		}
		if !haveBest || score > best {
			best = score
			leaders = []int{team}
			haveBest = true
		} else if score == best {
			leaders = append(leaders, team)
		}
	}
	return leaders
}

func (aggregate *Aggregate) teamScores() map[int]int {
	result := make(map[int]int)
	for _, player := range aggregate.Players {
		if player.Team > 0 {
			result[player.Team] += player.Score
		}
	}
	return result
}

func (aggregate *Aggregate) playersInTeams(teams []int) []int64 {
	allowed := make(map[int]struct{}, len(teams))
	for _, team := range teams {
		allowed[team] = struct{}{}
	}
	result := make([]int64, 0, len(aggregate.Players))
	for _, player := range aggregate.Players {
		if _, ok := allowed[player.Team]; ok {
			result = append(result, player.UserID)
		}
	}
	return uniqueSortedIDs(result)
}

func (aggregate *Aggregate) validRoster() bool {
	mode := aggregate.effectiveMode()
	count := len(aggregate.Players)
	if count < MinPlayers(mode) || count > MaxPlayers(mode) || (mode != ModeOpen && count != MaxPlayers(mode)) {
		return false
	}
	seen := make(map[int64]struct{}, count)
	for _, player := range aggregate.Players {
		if player.UserID <= 0 {
			return false
		}
		if _, exists := seen[player.UserID]; exists {
			return false
		}
		seen[player.UserID] = struct{}{}
	}
	if _, exists := seen[aggregate.OwnerID]; !exists {
		return false
	}
	if isTeamMode(mode) {
		teams := make(map[int]int)
		for _, player := range aggregate.Players {
			teams[player.Team]++
		}
		return len(teams) == 2 && teams[1] == TeamSize(mode) && teams[2] == TeamSize(mode)
	}
	return true
}

func (aggregate *Aggregate) effectiveMode() Mode {
	mode, err := NormalizeMode(string(aggregate.Mode))
	if err != nil {
		return ModeDuel
	}
	return mode
}

func (aggregate *Aggregate) playing() bool {
	return aggregate.Status == StatusActive || aggregate.Status == StatusTieBreak
}

func (aggregate *Aggregate) allPlayerIDs() []int64 {
	result := make([]int64, 0, len(aggregate.Players))
	for _, player := range aggregate.Players {
		result = append(result, player.UserID)
	}
	return result
}

func (aggregate *Aggregate) player(userID int64) *Player {
	for index := range aggregate.Players {
		if aggregate.Players[index].UserID == userID {
			return &aggregate.Players[index]
		}
	}
	return nil
}

func (aggregate *Aggregate) startTurn(turn *Turn, at time.Time) {
	turn.Status = TurnActive
	turn.StartedAt = at.UTC()
	turn.Deadline = at.UTC().Add(TurnDuration)
	if turn.Answers == nil {
		turn.Answers = make([]Answer, 0, len(aggregate.eligibleFor(turn)))
	}
}

func (aggregate *Aggregate) resolveTurn(turn *Turn, at time.Time) {
	turn.Status = TurnResolved
	turn.ResolvedAt = at.UTC()
	turn.RevealUntil = at.UTC().Add(RevealDuration)
}

func (aggregate *Aggregate) eligibleFor(turn *Turn) []int64 {
	if len(turn.EligibleUserIDs) > 0 {
		return turn.EligibleUserIDs
	}
	// Old two-player documents did not persist eligibility; every player was
	// eligible for those turns.
	return aggregate.allPlayerIDs()
}

func (aggregate *Aggregate) questionFor(turn *Turn) QuestionSnapshot {
	if turn.Question != nil {
		return *turn.Question
	}
	return turn.Card.Question
}

func (aggregate *Aggregate) commandSeen(commandID string, userID int64, action string) bool {
	for _, command := range aggregate.ProcessedCommands {
		if command.ID == commandID {
			return command.UserID == userID && command.Action == action
		}
	}
	return false
}

func (aggregate *Aggregate) recordCommand(commandID string, userID int64, action string, now time.Time) {
	aggregate.ProcessedCommands = append(aggregate.ProcessedCommands, ProcessedCommand{
		ID: commandID, UserID: userID, Action: action, AppliedAt: now.UTC(),
	})
	if len(aggregate.ProcessedCommands) > maximumProcessed {
		aggregate.ProcessedCommands = append([]ProcessedCommand(nil), aggregate.ProcessedCommands[len(aggregate.ProcessedCommands)-maximumProcessed:]...)
	}
}

func (aggregate *Aggregate) validateTieBreakQuestions(questions []QuestionSnapshot) error {
	if len(questions) == 0 {
		return ErrInvalidTieBreakPool
	}
	seen := aggregate.usedQuestionIDs()
	for _, question := range questions {
		if err := validateQuestion(question); err != nil {
			return ErrInvalidTieBreakPool
		}
		if _, exists := seen[question.ID]; exists {
			return ErrInvalidTieBreakPool
		}
		seen[question.ID] = struct{}{}
	}
	return nil
}

func (aggregate *Aggregate) usedQuestionIDs() map[string]struct{} {
	result := make(map[string]struct{})
	for _, player := range aggregate.Players {
		for _, card := range player.Deck {
			result[card.Question.ID] = struct{}{}
		}
	}
	for _, question := range aggregate.TieBreak.QuestionPool {
		result[question.ID] = struct{}{}
	}
	return result
}

func validateCommandID(commandID string) error {
	commandID = strings.TrimSpace(commandID)
	if len(commandID) < minimumCommandIDLength || len(commandID) > maximumCommandIDLength {
		return ErrInvalidCommandID
	}
	for _, character := range commandID {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' || character == ':' {
			continue
		}
		return ErrInvalidCommandID
	}
	return nil
}

func validateDeck(userID int64, cards []CardSnapshot) error {
	if len(cards) != DeckSize {
		return ErrInvalidDeck
	}
	seen := make(map[int64]struct{}, DeckSize)
	for _, card := range cards {
		if card.ID <= 0 || card.OwnerID != userID || validateQuestion(card.Question) != nil {
			return ErrInvalidDeck
		}
		if _, exists := seen[card.ID]; exists {
			return ErrInvalidDeck
		}
		seen[card.ID] = struct{}{}
	}
	return nil
}

func validateQuestion(question QuestionSnapshot) error {
	if strings.TrimSpace(question.ID) == "" || strings.TrimSpace(question.Prompt) == "" || len(question.Options) != 4 ||
		question.CorrectOption < 0 || question.CorrectOption > 3 {
		return ErrInvalidTieBreakPool
	}
	optionSeen := make(map[string]struct{}, 4)
	for _, option := range question.Options {
		normalized := strings.ToLower(strings.TrimSpace(option))
		if normalized == "" {
			return ErrInvalidTieBreakPool
		}
		if _, exists := optionSeen[normalized]; exists {
			return ErrInvalidTieBreakPool
		}
		optionSeen[normalized] = struct{}{}
	}
	return nil
}

func normalizeRoster(ownerID int64, playerIDs []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(playerIDs))
	for _, userID := range playerIDs {
		if userID <= 0 {
			return nil, ErrInvalidMatch
		}
		if _, exists := seen[userID]; exists {
			return nil, ErrInvalidMatch
		}
		seen[userID] = struct{}{}
	}
	if _, exists := seen[ownerID]; !exists {
		return nil, ErrInvalidMatch
	}
	result := make([]int64, 0, len(playerIDs))
	result = append(result, ownerID)
	for _, userID := range playerIDs {
		if userID != ownerID {
			result = append(result, userID)
		}
	}
	return result, nil
}

func cloneCards(cards []CardSnapshot) []CardSnapshot {
	result := make([]CardSnapshot, len(cards))
	for index, card := range cards {
		result[index] = cloneCard(card)
	}
	return result
}

func cloneCard(card CardSnapshot) CardSnapshot {
	card.Question = cloneQuestion(card.Question)
	return card
}

func cloneQuestions(questions []QuestionSnapshot) []QuestionSnapshot {
	result := make([]QuestionSnapshot, len(questions))
	for index, question := range questions {
		result[index] = cloneQuestion(question)
	}
	return result
}

func cloneQuestion(question QuestionSnapshot) QuestionSnapshot {
	question.Options = append([]string(nil), question.Options...)
	return question
}

func answerFor(answers []Answer, userID int64) (Answer, bool) {
	for _, answer := range answers {
		if answer.UserID == userID {
			return answer, true
		}
	}
	return Answer{}, false
}

func containsID(ids []int64, userID int64) bool {
	for _, candidate := range ids {
		if candidate == userID {
			return true
		}
	}
	return false
}

func uniqueSortedIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result
}

func uniqueSortedTeams(teams []int) []int {
	seen := make(map[int]struct{}, len(teams))
	result := make([]int, 0, len(teams))
	for _, team := range teams {
		if team <= 0 {
			continue
		}
		if _, exists := seen[team]; exists {
			continue
		}
		seen[team] = struct{}{}
		result = append(result, team)
	}
	sort.Ints(result)
	return result
}
