package resources

import "time"

type Wallet struct {
	Balance   int64 `json:"balance"`
	Locked    int64 `json:"locked"`
	Available int64 `json:"available"`
}

type CollectibleCard struct {
	ID          int64     `json:"id,string"`
	QuestionID  string    `json:"questionId"`
	Prompt      string    `json:"prompt"`
	Category    string    `json:"category"`
	Difficulty  string    `json:"difficulty"`
	SourceTitle string    `json:"sourceTitle"`
	Rarity      string    `json:"rarity"`
	Power       int       `json:"power"`
	Plays       int       `json:"plays"`
	Wins        int       `json:"wins"`
	Status      string    `json:"status"`
	LockRef     string    `json:"lockRef,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Collection struct {
	Wallet Wallet            `json:"wallet"`
	Cards  []CollectibleCard `json:"cards"`
}

type CreateListingModel struct {
	CardID    int64  `json:"cardId,string"`
	Price     int64  `json:"price"`
	CommandID string `json:"commandId"`
}

type CommandModel struct {
	CommandID string `json:"commandId"`
}

type MarketListing struct {
	ID        int64           `json:"id,string"`
	SellerID  int64           `json:"sellerId,string"`
	BuyerID   int64           `json:"buyerId,omitempty,string"`
	Price     int64           `json:"price"`
	Fee       int64           `json:"fee,omitempty"`
	Status    string          `json:"status"`
	Card      CollectibleCard `json:"card"`
	CreatedAt time.Time       `json:"createdAt"`
	ExpiresAt time.Time       `json:"expiresAt"`
}

type CreateTradeModel struct {
	ReceiverID       int64    `json:"receiverId,string"`
	OfferedCardIDs   []string `json:"offeredCardIds"`
	RequestedCardIDs []string `json:"requestedCardIds"`
	OfferedCoins     int64    `json:"offeredCoins"`
	RequestedCoins   int64    `json:"requestedCoins"`
	CommandID        string   `json:"commandId"`
}

type TradeOffer struct {
	ID               int64     `json:"id,string"`
	SenderID         int64     `json:"senderId,string"`
	ReceiverID       int64     `json:"receiverId,string"`
	OfferedCardIDs   []string  `json:"offeredCardIds"`
	RequestedCardIDs []string  `json:"requestedCardIds"`
	OfferedCoins     int64     `json:"offeredCoins"`
	RequestedCoins   int64     `json:"requestedCoins"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"createdAt"`
	ExpiresAt        time.Time `json:"expiresAt"`
}
