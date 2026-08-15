// Command e2e exercises the complete local MVP and its multiplayer arena modes
// against a running QuizBattle server. It deliberately refuses non-loopback
// targets unless the operator explicitly opts in through
// QUIZBATTLE_E2E_ALLOW_REMOTE=1.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const testPassword = "QuizBattle!2026"

type account struct {
	UserID   string `json:"userId"`
	FullName string `json:"fullName"`
	Username string `json:"username"`
}

type wallet struct {
	Balance   int64 `json:"balance"`
	Locked    int64 `json:"locked"`
	Available int64 `json:"available"`
}

type card struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type collection struct {
	Wallet wallet `json:"wallet"`
	Cards  []card `json:"cards"`
}

type game struct {
	ID         string `json:"id"`
	Mode       string `json:"mode"`
	MaxPlayers int    `json:"maxPlayers"`
}

type turn struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
}

type matchPlayer struct {
	UserID    string `json:"userId"`
	Team      int    `json:"team"`
	DeckReady bool   `json:"deckReady"`
}

type matchSnapshot struct {
	Mode          string        `json:"mode"`
	Status        string        `json:"status"`
	Version       int64         `json:"version"`
	Players       []matchPlayer `json:"players"`
	CurrentTurn   *turn         `json:"currentTurn"`
	RewardCoins   int64         `json:"rewardCoins"`
	TotalTurns    int           `json:"totalTurns"`
	TurnNumber    int           `json:"turnNumber"`
	CanStart      bool          `json:"canStart"`
	StartBlockers []string      `json:"startBlockers"`
	RewardsReady  bool          `json:"rewardsSettled"`
}

type listing struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type trade struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type chatMessage struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Message  string `json:"message"`
	UserID   string `json:"userId"`
	FullName string `json:"fullName"`
}

type voiceSignal struct {
	Type       string `json:"type"`
	FromUserID string `json:"fromUserId"`
}

type apiClient struct {
	baseURL *url.URL
	http    *http.Client
	account account
}

func main() {
	base := flag.String("base", "http://127.0.0.1:8080", "QuizBattle base URL")
	flag.Parse()

	baseURL, err := url.Parse(strings.TrimRight(*base, "/"))
	check(err)
	if err := requireSafeTarget(baseURL); err != nil {
		check(err)
	}

	suffix := fmt.Sprintf("%09d", time.Now().UnixNano()%1_000_000_000)
	owner := newClient(baseURL)
	guest := newClient(baseURL)
	owner.account = owner.signUp("لاعب الاختبار الأول", "owner"+suffix, "owner"+suffix+"@example.test", "+201"+suffix)
	guest.account = guest.signUp("لاعب الاختبار الثاني", "guest"+suffix, "guest"+suffix+"@example.test", "+202"+suffix)
	fmt.Printf("accounts: owner=%s guest=%s\n", owner.account.UserID, guest.account.UserID)
	clients := []*apiClient{owner, guest}
	for index := 0; index < 6; index++ {
		client := newClient(baseURL)
		label := fmt.Sprintf("extra%d", index+1)
		client.account = client.signUp(
			fmt.Sprintf("لاعب الاختبار %d", index+3),
			label+suffix,
			label+suffix+"@example.test",
			fmt.Sprintf("+20%d%s", index+3, suffix),
		)
		clients = append(clients, client)
	}
	require(len(clients) == 8, "multiplayer verification must use exactly eight isolated clients")
	fmt.Println("accounts: eight isolated clients are ready for multiplayer arenas")

	testWorldChat(owner, guest)

	ownerCollection := owner.getCollection()
	guestCollection := guest.getCollection()
	require(len(ownerCollection.Cards) == 10 && len(guestCollection.Cards) == 10, "starter collections must contain 10 cards each")
	require(ownerCollection.Wallet.Available == 600 && guestCollection.Wallet.Available == 600, "starter wallets must contain 600 available coins")
	fmt.Println("starter economy: 10 cards and 600 coins per player")

	var battle game
	owner.request(http.MethodPost, "/api/v1/game", map[string]any{"isPublic": true}, &battle)
	require(battle.ID != "", "created battle has no ID")
	guest.request(http.MethodPost, "/api/v1/game/"+battle.ID+"/join", nil, &game{})
	fmt.Printf("battle: %s created and joined\n", battle.ID)
	testVoiceSignaling(owner, guest, battle.ID)
	owner.request(http.MethodPost, "/api/v1/game/"+battle.ID+"/prepare", map[string]any{
		"commandId": commandID("prepare"),
	}, &matchSnapshot{})

	ownerDeck := firstAvailable(ownerCollection.Cards, 5)
	guestDeck := firstAvailable(guestCollection.Cards, 5)
	var snapshot matchSnapshot
	owner.request(http.MethodPut, "/api/v1/game/"+battle.ID+"/deck", map[string]any{
		"cardIds": ownerDeck, "commandId": commandID("owner-deck"),
	}, &snapshot)
	guest.request(http.MethodPut, "/api/v1/game/"+battle.ID+"/deck", map[string]any{
		"cardIds": guestDeck, "commandId": commandID("guest-deck"),
	}, &snapshot)
	owner.request(http.MethodPost, "/api/v1/game/"+battle.ID+"/start", map[string]any{
		"commandId": commandID("start"),
	}, &snapshot)
	require(snapshot.Status == "active" && snapshot.TotalTurns == 10, "match did not start with 10 turns")

	completed := playMatch(owner, guest, battle.ID, snapshot)
	require(completed.Status == "completed", "match did not complete")
	require(completed.RewardCoins > 0, "match reward was not exposed to the player")
	fmt.Printf("match: completed 10 turns; owner reward=%d coins\n", completed.RewardCoins)

	ownerAfterMatch := owner.getCollection()
	guestAfterMatch := guest.getCollection()
	require(ownerAfterMatch.Wallet.Balance > 600 && guestAfterMatch.Wallet.Balance > 600, "match rewards were not settled for both players")
	require(allAvailable(ownerAfterMatch.Cards) && allAvailable(guestAfterMatch.Cards), "match cards were not released after settlement")

	marketCard := firstAvailable(ownerAfterMatch.Cards, 1)[0]
	var marketListing listing
	owner.request(http.MethodPost, "/api/v1/market/listings", map[string]any{
		"cardId": marketCard, "price": 50, "commandId": commandID("listing"),
	}, &marketListing)
	guest.request(http.MethodPost, "/api/v1/market/listings/"+marketListing.ID+"/buy", map[string]any{
		"commandId": commandID("buy"),
	}, &marketListing)
	require(marketListing.Status == "sold", "market listing was not sold")
	ownerAfterSale := owner.getCollection()
	guestAfterPurchase := guest.getCollection()
	require(!owns(ownerAfterSale, marketCard) && owns(guestAfterPurchase, marketCard), "market purchase did not transfer card ownership")
	fmt.Printf("market: card %s sold for 50 coins\n", marketCard)

	guestTradeCard := marketCard
	ownerTradeCard := firstAvailable(ownerAfterSale.Cards, 1)[0]
	var offer trade
	guest.request(http.MethodPost, "/api/v1/trades", map[string]any{
		"receiverId":       owner.account.UserID,
		"offeredCardIds":   []string{guestTradeCard},
		"requestedCardIds": []string{ownerTradeCard},
		"offeredCoins":     0,
		"requestedCoins":   0,
		"commandId":        commandID("trade-create"),
	}, &offer)
	owner.request(http.MethodPost, "/api/v1/trades/"+offer.ID+"/accept", map[string]any{
		"commandId": commandID("trade-accept"),
	}, &offer)
	require(offer.Status == "accepted", "trade was not accepted")
	ownerAfterTrade := owner.getCollection()
	guestAfterTrade := guest.getCollection()
	require(owns(ownerAfterTrade, guestTradeCard) && owns(guestAfterTrade, ownerTradeCard), "trade did not atomically swap ownership")
	fmt.Printf("trade: cards %s and %s swapped atomically\n", guestTradeCard, ownerTradeCard)

	testForfeitReleasesCards(owner, guest, ownerAfterTrade, guestAfterTrade)
	testMultiplayerArenas(clients)

	fmt.Println("PASS: full duel, 2v2, 4v4, open arena, chat, rewards, forfeit recovery, market, and trade scenario")
}

func testForfeitReleasesCards(owner, guest *apiClient, ownerBefore, guestBefore collection) {
	var battle game
	owner.request(http.MethodPost, "/api/v1/game", map[string]any{"isPublic": true}, &battle)
	require(battle.ID != "", "forfeit test battle has no ID")
	guest.request(http.MethodPost, "/api/v1/game/"+battle.ID+"/join", nil, &game{})
	owner.request(http.MethodPost, "/api/v1/game/"+battle.ID+"/prepare", map[string]any{
		"commandId": commandID("forfeit-prepare"),
	}, &matchSnapshot{})

	ownerDeck := firstAvailable(ownerBefore.Cards, 5)
	var snapshot matchSnapshot
	owner.request(http.MethodPut, "/api/v1/game/"+battle.ID+"/deck", map[string]any{
		"cardIds": ownerDeck, "commandId": commandID("forfeit-owner-deck"),
	}, &snapshot)
	guest.request(http.MethodPost, "/api/v1/game/"+battle.ID+"/forfeit", map[string]any{
		"commandId": commandID("guest-forfeit"),
	}, &snapshot)
	require(snapshot.Status == "forfeited", "match was not marked forfeited")
	require(snapshot.RewardCoins == 0 && snapshot.RewardsReady, "forfeit must settle with no reward")

	ownerAfter := owner.getCollection()
	guestAfter := guest.getCollection()
	require(ownerAfter.Wallet.Balance == ownerBefore.Wallet.Balance, "forfeit changed the owner's balance")
	require(guestAfter.Wallet.Balance == guestBefore.Wallet.Balance, "forfeit changed the guest's balance")
	require(allAvailable(ownerAfter.Cards) && allAvailable(guestAfter.Cards), "forfeit did not release every card")
	fmt.Println("forfeit: no rewards were issued and all committed cards were released")
}

func testMultiplayerArenas(clients []*apiClient) {
	require(len(clients) == 8, "multiplayer scenarios require eight clients")
	testArenaLifecycle("2v2", "team_2v2", 4, clients[:4], 20)
	testArenaLifecycle("4v4", "team_4v4", 8, clients, 40)
	testArenaLifecycle("open-3", "open", 8, clients[:3], 15)
	fmt.Println("multiplayer arenas: 2v2, 4v4, and open-three preparation/start/forfeit paths passed")
}

func testArenaLifecycle(name, mode string, maximumPlayers int, players []*apiClient, expectedTurns int) {
	require(len(players) >= 2 && len(players) <= 8, name+": invalid E2E roster")
	before := make([]collection, len(players))
	for index, player := range players {
		before[index] = player.getCollection()
		require(len(before[index].Cards) >= 5, name+": player collection has fewer than five cards")
		require(allAvailable(before[index].Cards), name+": player entered arena with a locked card")
	}

	owner := players[0]
	var battle game
	owner.request(http.MethodPost, "/api/v1/game", map[string]any{
		"isPublic":   true,
		"mode":       mode,
		"maxPlayers": maximumPlayers,
	}, &battle)
	require(battle.ID != "", name+": created arena has no ID")
	require(battle.Mode == mode && battle.MaxPlayers == maximumPlayers, name+": arena policy was not persisted")
	for _, player := range players[1:] {
		player.request(http.MethodPost, "/api/v1/game/"+battle.ID+"/join", nil, &game{})
	}

	var snapshot matchSnapshot
	owner.request(http.MethodPost, "/api/v1/game/"+battle.ID+"/prepare", map[string]any{
		"commandId": commandID(name + "-prepare"),
	}, &snapshot)
	require(snapshot.Status == "collecting_decks", name+": prepare did not freeze the roster")
	require(snapshot.Mode == mode && len(snapshot.Players) == len(players), name+": prepared snapshot has the wrong roster or mode")

	// Commit the owner's deck last so the returned owner-scoped snapshot can
	// prove that every participant is ready and the owner may start.
	for index := 1; index < len(players); index++ {
		players[index].request(http.MethodPut, "/api/v1/game/"+battle.ID+"/deck", map[string]any{
			"cardIds":   firstAvailable(before[index].Cards, 5),
			"commandId": commandID(fmt.Sprintf("%s-deck-%d", name, index)),
		}, &matchSnapshot{})
	}
	owner.request(http.MethodPut, "/api/v1/game/"+battle.ID+"/deck", map[string]any{
		"cardIds":   firstAvailable(before[0].Cards, 5),
		"commandId": commandID(name + "-deck-owner"),
	}, &snapshot)
	require(snapshot.CanStart && len(snapshot.StartBlockers) == 0, name+": owner could not start after every deck was ready")
	for _, participant := range snapshot.Players {
		require(participant.DeckReady, name+": snapshot contains an unready player")
	}

	owner.request(http.MethodPost, "/api/v1/game/"+battle.ID+"/start", map[string]any{
		"commandId": commandID(name + "-start"),
	}, &snapshot)
	require(snapshot.Status == "active", name+": match did not become active")
	require(snapshot.TotalTurns == expectedTurns, fmt.Sprintf("%s: totalTurns=%d want %d", name, snapshot.TotalTurns, expectedTurns))

	nonOwnerStatus := players[len(players)-1].requestStatus(http.MethodPost, "/api/v1/game/"+battle.ID+"/forfeit", map[string]any{
		"commandId": commandID(name + "-forfeit"),
	})
	require(nonOwnerStatus == http.StatusForbidden, name+": non-owner could cancel the multiplayer arena")
	owner.request(http.MethodPost, "/api/v1/game/"+battle.ID+"/forfeit", map[string]any{
		"commandId": commandID(name + "-owner-forfeit"),
	}, &snapshot)
	require(snapshot.Status == "forfeited", name+": forfeit did not terminate the match")
	require(snapshot.RewardCoins == 0 && snapshot.RewardsReady, name+": forfeit must settle with zero reward")

	for index, player := range players {
		after := player.getCollection()
		require(after.Wallet.Balance == before[index].Wallet.Balance, name+": forfeit changed a wallet balance")
		require(allAvailable(after.Cards), name+": forfeit did not release every committed card")
	}
	fmt.Printf("arena %s: %d players, %d turns, forfeit unlock verified\n", name, len(players), expectedTurns)
}

func newClient(baseURL *url.URL) *apiClient {
	jar, err := cookiejar.New(nil)
	check(err)
	return &apiClient{
		baseURL: baseURL,
		http: &http.Client{
			Jar:     jar,
			Timeout: 20 * time.Second,
		},
	}
}

func (client *apiClient) signUp(fullName, username, email, mobile string) account {
	var result account
	client.request(http.MethodPost, "/user/createuser", map[string]any{
		"fullName": fullName, "username": username, "email": email,
		"mobileNumber": mobile, "password": testPassword,
	}, &result)
	require(result.UserID != "", "signup response has no user ID")
	return result
}

func (client *apiClient) getCollection() collection {
	var result collection
	client.request(http.MethodGet, "/api/v1/collection", nil, &result)
	return result
}

func (client *apiClient) request(method, path string, input, output any) {
	response, data := client.doRequest(method, path, input)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		check(fmt.Errorf("%s %s returned %s: %s", method, path, response.Status, strings.TrimSpace(string(data))))
	}
	if output != nil && len(bytes.TrimSpace(data)) > 0 {
		check(json.Unmarshal(data, output))
	}
}

func (client *apiClient) requestStatus(method, path string, input any) int {
	response, _ := client.doRequest(method, path, input)
	return response.StatusCode
}

func (client *apiClient) doRequest(method, path string, input any) (*http.Response, []byte) {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		check(err)
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, client.baseURL.String()+path, body)
	check(err)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", client.baseURL.String())
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.http.Do(req)
	check(err)
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	check(err)
	return response, data
}

func playMatch(owner, guest *apiClient, gameID string, snapshot matchSnapshot) matchSnapshot {
	deadline := time.Now().Add(75 * time.Second)
	answeredTurn := make(map[string]bool, 12)
	mainTurnsAnswered := 0
	for snapshot.Status != "completed" && time.Now().Before(deadline) {
		if snapshot.CurrentTurn != nil && snapshot.CurrentTurn.Status == "active" && !answeredTurn[snapshot.CurrentTurn.ID] {
			turnID := snapshot.CurrentTurn.ID
			owner.request(http.MethodPost, "/api/v1/game/"+gameID+"/answer", map[string]any{
				"turnId": turnID, "option": 0, "commandId": commandID("owner-answer"),
			}, &snapshot)
			// A visible timing difference prevents identical correct answers from
			// producing the same score. If the main result still ties, the same
			// rule safely exercises the server-created tie-break turns.
			time.Sleep(500 * time.Millisecond)
			guest.request(http.MethodPost, "/api/v1/game/"+gameID+"/answer", map[string]any{
				"turnId": turnID, "option": 0, "commandId": commandID("guest-answer"),
			}, &snapshot)
			answeredTurn[turnID] = true
			if snapshot.CurrentTurn == nil || snapshot.CurrentTurn.Kind != "tie_break" {
				mainTurnsAnswered++
			}
		}
		if snapshot.Status == "completed" {
			break
		}
		time.Sleep(500 * time.Millisecond)
		owner.request(http.MethodGet, "/api/v1/game/"+gameID+"/match", nil, &snapshot)
	}
	require(mainTurnsAnswered == 10, fmt.Sprintf("expected 10 answered main turns, got %d", mainTurnsAnswered))
	return snapshot
}

func testWorldChat(sender, receiver *apiClient) {
	senderSocket := sender.chatSocket()
	defer senderSocket.Close()
	receiverSocket := receiver.chatSocket()
	defer receiverSocket.Close()

	message := "اختبار مجلس اللاعبين " + commandID("chat")
	check(senderSocket.WriteJSON(map[string]string{"type": "text", "message": message}))
	check(receiverSocket.SetReadDeadline(time.Now().Add(5 * time.Second)))
	for {
		var received chatMessage
		check(receiverSocket.ReadJSON(&received))
		if received.Type == "text" && received.Message == message {
			require(received.UserID == sender.account.UserID, "chat message identity was not server-authenticated")
			break
		}
	}
	var history []chatMessage
	receiver.request(http.MethodGet, "/api/v1/chat/messages", nil, &history)
	persisted := false
	for _, item := range history {
		if item.Message == message && item.UserID == sender.account.UserID && item.ID != "" {
			persisted = true
			break
		}
	}
	require(persisted, "world chat message was not available after history reload")
	fmt.Println("world chat: authenticated message delivered and persisted for reload")
}

func testVoiceSignaling(sender, receiver *apiClient, gameID string) {
	senderSocket := sender.gameSocket(gameID)
	defer senderSocket.Close()
	receiverSocket := receiver.gameSocket(gameID)
	defer receiverSocket.Close()

	check(senderSocket.WriteJSON(map[string]any{
		"type":       "voice_ready",
		"fromUserId": "spoofed-client-id",
		"payload":    map[string]any{},
	}))
	check(receiverSocket.SetReadDeadline(time.Now().Add(5 * time.Second)))
	for {
		var received voiceSignal
		check(receiverSocket.ReadJSON(&received))
		if received.Type == "voice_ready" {
			require(received.FromUserID == sender.account.UserID, "voice signaling identity was not server-authenticated")
			break
		}
	}
	fmt.Println("battle voice: authenticated WebRTC signaling relayed")
}

func (client *apiClient) chatSocket() *websocket.Conn {
	return client.websocket("/ws/world-chat")
}

func (client *apiClient) gameSocket(gameID string) *websocket.Conn {
	return client.websocket("/ws/game/" + gameID)
}

func (client *apiClient) websocket(path string) *websocket.Conn {
	wsURL := *client.baseURL
	if wsURL.Scheme == "https" {
		wsURL.Scheme = "wss"
	} else {
		wsURL.Scheme = "ws"
	}
	wsURL.Path = path
	header := http.Header{}
	header.Set("Origin", client.baseURL.String())
	cookies := client.http.Jar.Cookies(client.baseURL)
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	header.Set("Cookie", strings.Join(parts, "; "))
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	connection, response, err := dialer.Dial(wsURL.String(), header)
	if err != nil {
		if response != nil {
			check(fmt.Errorf("websocket handshake returned %s: %w", response.Status, err))
		}
		check(err)
	}
	return connection
}

func firstAvailable(cards []card, count int) []string {
	result := make([]string, 0, count)
	for _, item := range cards {
		if item.Status == "available" {
			result = append(result, item.ID)
			if len(result) == count {
				return result
			}
		}
	}
	check(fmt.Errorf("needed %d available cards, found %d", count, len(result)))
	return nil
}

func allAvailable(cards []card) bool {
	for _, item := range cards {
		if item.Status != "available" {
			return false
		}
	}
	return true
}

func owns(value collection, cardID string) bool {
	for _, item := range value.Cards {
		if item.ID == cardID {
			return true
		}
	}
	return false
}

func commandID(action string) string {
	return fmt.Sprintf("e2e-%s-%d", action, time.Now().UnixNano())
}

func requireSafeTarget(target *url.URL) error {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" {
		return errors.New("invalid QuizBattle base URL")
	}
	host := target.Hostname()
	loopback := strings.EqualFold(host, "localhost")
	if address := net.ParseIP(host); address != nil {
		loopback = address.IsLoopback()
	}
	if !loopback && os.Getenv("QUIZBATTLE_E2E_ALLOW_REMOTE") != "1" {
		return errors.New("refusing to create E2E accounts on a non-loopback host; set QUIZBATTLE_E2E_ALLOW_REMOTE=1 only when intentional")
	}
	return nil
}

func require(condition bool, message string) {
	if !condition {
		check(errors.New(message))
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}
