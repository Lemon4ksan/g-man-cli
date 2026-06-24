// Copyright (c) 2026 lemon4ksan. All rights reserved.
// Use of this source code is governed by a proprietary license.

package driver

// ResolvedItem represents a resolved item from the TF2 API.
type ResolvedItem struct {
	AppID          uint32 `json:"appid"`
	ContextID      string `json:"contextid"`
	AssetID        string `json:"assetid"`
	ClassID        string `json:"classid"`
	DefIndex       string `json:"defindex"`
	InstanceID     string `json:"instanceid"`
	Amount         string `json:"amount"`
	MarketHashName string `json:"market_hash_name"`
}

// ResolvedOffer represents a resolved trade offer from the TF2 API.
type ResolvedOffer struct {
	ID             string         `json:"tradeofferid"`
	OtherSteamID   string         `json:"accountid_other"`
	Message        string         `json:"message"`
	State          int            `json:"trade_offer_state"`
	IsOurOffer     bool           `json:"is_our_offer"`
	ItemsToGive    []ResolvedItem `json:"items_to_give"`
	ItemsToReceive []ResolvedItem `json:"items_to_receive"`
	TimeCreated    int64          `json:"time_created"`
	TimeUpdated    int64          `json:"time_updated"`
}
