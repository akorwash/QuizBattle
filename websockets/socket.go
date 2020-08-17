package websockets

import "github.com/akorwash/QuizBattle/resources"

//GameConnections all the connection to games
var GameConnections map[int64]Hub

//Games all the games running by game engin
var Games map[int64]resources.Game
