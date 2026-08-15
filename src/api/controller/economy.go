package controller

import (
	"net/http"

	"github.com/akorwash/QuizBattle/resources"
	"github.com/akorwash/QuizBattle/service"
)

type EconomyController struct{}

func (controller *EconomyController) Collection(economyService *service.EconomyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		collection, err := economyService.Collection(r.Context(), identity.UserID)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, collection)
	}
}

func (controller *EconomyController) Market(economyService *service.EconomyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		listings, err := economyService.Market(r.Context())
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, listings)
	}
}

func (controller *EconomyController) CreateListing(economyService *service.EconomyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		var input resources.CreateListingModel
		if err := decodeJSON(w, r, &input); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		listing, err := economyService.CreateListing(r.Context(), identity.UserID, input)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusCreated, listing)
	}
}

func (controller *EconomyController) BuyListing(economyService *service.EconomyService) http.HandlerFunc {
	return controller.listingCommand(economyService, "buy")
}

func (controller *EconomyController) CancelListing(economyService *service.EconomyService) http.HandlerFunc {
	return controller.listingCommand(economyService, "cancel")
}

func (controller *EconomyController) listingCommand(economyService *service.EconomyService, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		listingID, err := pathID(r, "id")
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "invalid listing ID")
			return
		}
		var input resources.CommandModel
		if err := decodeJSON(w, r, &input); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		var listing *resources.MarketListing
		if action == "buy" {
			listing, err = economyService.BuyListing(r.Context(), identity.UserID, listingID, input.CommandID)
		} else {
			listing, err = economyService.CancelListing(r.Context(), identity.UserID, listingID, input.CommandID)
		}
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, listing)
	}
}

func (controller *EconomyController) Trades(economyService *service.EconomyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		trades, err := economyService.Trades(r.Context(), identity.UserID)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, trades)
	}
}

func (controller *EconomyController) CreateTrade(economyService *service.EconomyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		var input resources.CreateTradeModel
		if err := decodeJSON(w, r, &input); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		trade, err := economyService.CreateTrade(r.Context(), identity.UserID, input)
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusCreated, trade)
	}
}

func (controller *EconomyController) TradeCommand(economyService *service.EconomyService, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		tradeID, err := pathID(r, "id")
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, "invalid trade ID")
			return
		}
		var input resources.CommandModel
		if err := decodeJSON(w, r, &input); err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		var trade *resources.TradeOffer
		if action == "accept" {
			trade, err = economyService.AcceptTrade(r.Context(), identity.UserID, tradeID, input.CommandID)
		} else {
			trade, err = economyService.CloseTrade(r.Context(), identity.UserID, tradeID, action, input.CommandID)
		}
		if err != nil {
			respondServiceError(w, err)
			return
		}
		responseHandler.RespondWithJSON(w, http.StatusOK, trade)
	}
}
