package entites

//CardFileName to do
var CardFileName = "cards.json"

//CardEntity to do
type CardEntity struct {
	ID    int
	Power float32
	Owner string

	Likes int
	Hits  int

	Questions QuestionEntity
}
