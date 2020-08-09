package engine

import (
	"math/rand"

	"github.com/akorwash/QuizBattle/actor"
)

//Card to do
type Card struct {
	id    int
	power float32
	owner string

	likes int
	hits  int

	questions Question
}

//CardList to do
type CardList []Card

//CardsSet to do
var CardsSet CardList

//GetRandomCard to do
func (cardlist *CardList) GetRandomCard() *Card {
	min := 1
	max := len(CardsSet)
	return &CardsSet[rand.Intn(max-min+1)+min]
}

//GetUserCards get name of Bot
func GetUserCards(ownderUser string) *[]Card {
	var resCards []Card
	for _, curCard := range CardsSet {
		if curCard.owner == ownderUser {
			resCards = append(resCards, curCard)
		}
	}
	return &resCards
}

//HasQuestion get name of Bot
func (card *Card) HasQuestion() bool {
	if card.questions.id == 0 {
		return false
	}
	return true
}

//GetCardData get name of Bot
func (card *Card) GetCardData() (_id int, _power float32, _owner string, _likes int, _hits int) {
	return card.id, card.power, card.owner, card.likes, card.hits
}

//GetCardQuestion get name of Bot
func (card *Card) GetCardQuestion() *Question {
	return &card.questions
}

//NewCard ctor for User Account
func NewCard(_id int) *Card {
	card := Card{id: _id}
	return &card
}

//NewLoadCard ctor for User Account
func NewLoadCard(_id int, _power float32, _owner string, _likes int, _hits int) *Card {
	card := Card{id: _id, power: _power, owner: _owner, likes: _likes, hits: _hits}
	return &card
}

//AssignToUser ctor for User Account
func (card *Card) AssignToUser(owner *actor.User) *Card {
	card.owner = owner.GetUserName()
	return card
}

//AssignQuestion ctor for User Account
func (card *Card) AssignQuestion(_question Question) *Card {
	card.questions = _question
	return card
}
