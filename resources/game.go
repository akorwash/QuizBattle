package resources

//Game List of Games running by the game engin
type Game struct {
	ID         int64
	IsPublic   bool
	User       UserModel
	TimeLine   []string
	IsActive   bool
	JoinedUser []UserModel
}

//CreateGameModel to create new game
type CreateGameModel struct {
	IsPublic bool
	UserID   uint64
}
