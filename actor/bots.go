package actor

//BotPlayer is class represent Player User account with login data
type BotPlayer struct {
	botName string
	level   int
}

//BotList to do
type BotList []BotPlayer

//BotSet to do
var BotSet BotList

//NewBot ctor for User Account
//name is Bot Player Account
//botLevel is Bot Player Account
func NewBot(name string, botLevel int) *BotPlayer {
	bot := BotPlayer{botName: name, level: botLevel}
	return &bot
}

//GetName get name of Bot
func (bot *BotPlayer) GetName() string {
	return bot.botName
}

//GetLevel get name of Bot
func (bot *BotPlayer) GetLevel() int {
	return bot.level
}
