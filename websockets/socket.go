package websockets

import "github.com/akorwash/QuizBattle/resources"

var gameConnections map[int]Hub
var games []resources.Game

//AddNew to do
func AddNew() {
	gameID := len(games) + 1
	if gameConnections == nil {
		gameConnections = make(map[int]Hub)
	}
	hub := NewHub()
	games = append(games, resources.Game{ID: gameID, IsActive: true, IsPublic: true})
	gameConnections[gameID] = *hub
	go hub.Run()
}
