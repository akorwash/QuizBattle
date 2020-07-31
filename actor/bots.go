package actor

//botPlayer is class represent Player User account with login data
type botPlayer struct {
	botName string
	level   int
}

//NewBot ctor for User Account
//botPlayer is Bot Player Account
func NewBot(name string, botLevel int) *botPlayer {
	bot := botPlayer{botName: name, level: botLevel}
	return &bot
}
