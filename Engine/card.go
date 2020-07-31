package engine

import (
	"QuizBattle/actor"
)

//Card to do
type Card struct {
	id    int
	power float32
	owner string

	likes int
	Hits  int
}

//CardList to do
type CardList []Card

//CardsSet to do
var CardsSet CardList

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

//NewCard ctor for User Account
func NewCard(_id int) *Card {
	card := Card{id: _id}
	return &card
}

//AssignToUser ctor for User Account
func (card *Card) AssignToUser(owner *actor.User) *Card {
	card.owner = owner.GetUserName()
	return card
}
