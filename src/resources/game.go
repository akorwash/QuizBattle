package resources

// Game List of Games running by the game engin
type Game struct {
	ID           int64       `json:"id,string"`
	IsPublic     bool        `json:"isPublic"`
	Owner        UserModel   `json:"owner"`
	Mode         string      `json:"mode"`
	OpponentType string      `json:"opponentType"`
	BotStrategy  string      `json:"botStrategy,omitempty"`
	MinPlayers   int         `json:"minPlayers"`
	MaxPlayers   int         `json:"maxPlayers"`
	TeamSize     int         `json:"teamSize"`
	Timeline     []string    `json:"timeline,omitempty"`
	IsActive     bool        `json:"isActive"`
	JoinedUsers  []UserModel `json:"joinedUsers"`
	State        string      `json:"state"`
	MatchID      int64       `json:"matchId,omitempty,string"`
}

// CreateGameModel to create new game
type CreateGameModel struct {
	IsPublic     bool   `json:"isPublic"`
	Mode         string `json:"mode"`
	MaxPlayers   int    `json:"maxPlayers,omitempty"`
	OpponentType string `json:"opponentType,omitempty"`
	BotStrategy  string `json:"botStrategy,omitempty"`
}

// GameEvent is a server-authored notification for lobby clients.
type GameEvent struct {
	Type         string `json:"type"`
	GameID       int64  `json:"gameId,string"`
	MatchVersion int64  `json:"matchVersion,omitempty"`
}

// Answer deliberately excludes IsCorrect. Answers are evaluated by the server.
type Answer struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

// Question is the safe client representation of a question card.
type Question struct {
	ID      int      `json:"id"`
	Header  string   `json:"header"`
	Answers []Answer `json:"answers"`
}
