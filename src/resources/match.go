package resources

type CommitDeckModel struct {
	CardIDs   []string `json:"cardIds"`
	CommandID string   `json:"commandId"`
}

type SubmitAnswerModel struct {
	TurnID    string `json:"turnId"`
	Option    int    `json:"option"`
	CommandID string `json:"commandId"`
}
