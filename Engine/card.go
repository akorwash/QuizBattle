package engine

import (
	"QuizBattle/actor"
	"math/rand"
)

//Card to do
type Card struct {
	id    int
	power float32
	owner string

	likes int
	hits  int

	questions []Question
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

//GetQuestions get name of Bot
func (card *Card) GetQuestions() []Question {
	var res []Question

	return res
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
