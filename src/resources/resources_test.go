package resources

import (
	"encoding/json"
	"testing"
)

func TestLargeIdentifiersAreEncodedAsJSONStrings(t *testing.T) {
	const gameID int64 = 1 << 60
	const ownerID int64 = 1<<60 + 1
	const memberID int64 = 1<<60 + 2
	payload, err := json.Marshal(Game{
		ID:          gameID,
		Owner:       UserModel{ID: ownerID, FullName: "Owner"},
		JoinedUsers: []UserModel{{ID: memberID, FullName: "Member"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		ID    string `json:"id"`
		Owner struct {
			ID string `json:"id"`
		} `json:"owner"`
		JoinedUsers []struct {
			ID string `json:"id"`
		} `json:"joinedUsers"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != "1152921504606846976" || decoded.Owner.ID != "1152921504606846977" || len(decoded.JoinedUsers) != 1 || decoded.JoinedUsers[0].ID != "1152921504606846978" {
		t.Fatalf("nested identifiers would lose precision in JavaScript: %s", payload)
	}

	event, err := json.Marshal(GameEvent{Type: "created", GameID: gameID})
	if err != nil {
		t.Fatal(err)
	}
	var decodedEvent struct {
		GameID string `json:"gameId"`
	}
	if err := json.Unmarshal(event, &decodedEvent); err != nil || decodedEvent.GameID != "1152921504606846976" {
		t.Fatalf("event identifier would lose precision in JavaScript: %s", event)
	}

	account, err := json.Marshal(UserAccount{UserID: memberID})
	if err != nil {
		t.Fatal(err)
	}
	var decodedAccount struct {
		UserID string `json:"userId"`
	}
	if err := json.Unmarshal(account, &decodedAccount); err != nil || decodedAccount.UserID != "1152921504606846978" {
		t.Fatalf("account identifier would lose precision in JavaScript: %s", account)
	}
}
