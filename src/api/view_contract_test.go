package api

import (
	"os"
	"strings"
	"testing"
)

func TestArenaCreationViewsExposeEverySupportedMode(t *testing.T) {
	t.Parallel()
	required := []string{
		`id="createArenaForm"`,
		`value="duel"`,
		`value="team_2v2"`,
		`value="team_4v4"`,
		`value="open"`,
		`id="openMaxPlayers"`,
		`id="creategame"`,
		`type="submit"`,
	}
	for _, path := range []string{"view/gameplay.html", "view/user.html"} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			page := string(content)
			for _, contract := range required {
				if !strings.Contains(page, contract) {
					t.Errorf("%s is missing arena creation contract %q", path, contract)
				}
			}
		})
	}
}

func TestPlayerAvatarEditorContract(t *testing.T) {
	t.Parallel()
	content, err := os.ReadFile("view/user.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(content)
	required := []string{
		`id="avatarForm"`,
		`method="post"`,
		`action="/api/v1/user/avatar"`,
		`id="avatarInput"`,
		`name="avatar"`,
		`accept=".jpg,.jpeg,.png,image/jpeg,image/png"`,
		`id="avatarUploadButton"`,
		`id="avatarDeleteButton"`,
		`id="profileAvatarImage"`,
		`src="/static/account.js`,
	}
	for _, contract := range required {
		if !strings.Contains(page, contract) {
			t.Errorf("view/user.html is missing avatar contract %q", contract)
		}
	}
}

func TestBattleViewExposesReviewableAutoDeckControls(t *testing.T) {
	t.Parallel()
	content, err := os.ReadFile("view/battle.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(content)
	required := []string{
		`id="autoSelectDeckButton"`,
		`id="clearDeckSelectionButton"`,
		`id="deckAssistStatus"`,
		`src="/static/deck-ranking.js`,
		`src="/static/battle.js`,
	}
	for _, contract := range required {
		if !strings.Contains(page, contract) {
			t.Errorf("view/battle.html is missing automatic deck contract %q", contract)
		}
	}
}
