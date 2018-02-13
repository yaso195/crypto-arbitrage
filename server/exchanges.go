package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/buger/jsonparser"

	gdax "github.com/preichenberger/go-gdax"
)

const (
	PARIBU_URI   = "https://www.paribu.com/ticker"
	BTCTURK_URI  = "https://www.btcturk.com/api/ticker"
	KOINEKS_URI  = "https://koineks.com/ticker"
	BITFLYER_URI = "https://api.bitflyer.jp/v1/ticker"

	PARIBU   = "Paribu"
	BTCTURK  = "BTCTurk"
	KOINEKS  = "Koineks"
	BITFLYER = "Bitflyer"
)

var (
	symbolToExchangeNames map[string][]string

	ALL_EXCHANGES = []string{PARIBU, BTCTURK, KOINEKS, BITFLYER}
)

func init() {
	notificationFlags = map[string]bool{}
	notificationTimes = map[string]time.Time{}
	for _, exchange := range ALL_EXCHANGES {
		for _, symbol := range ALL_SYMBOLS {
			exchangeSymbol := fmt.Sprintf("%s%s", exchange, symbol)
			notificationFlags[exchangeSymbol] = false
			notificationTimes[exchangeSymbol] = time.Time{}
		}
	}

	diffs = map[string]float64{}

	PUSHOVER_USER = os.Getenv("PUSHOVER_USER")
	PUSHOVER_APP_TOKEN = os.Getenv("PUSHOVER_APP_TOKEN")
}

func getGdaxPrices() ([]Price, error) {

	client := gdax.NewClient("", "", "")
	var prices []Price

	ids := []string{"BTC-USD", "ETH-USD", "LTC-USD", "ETH-BTC"}

	for _, id := range ids {
		ticker, err := client.GetTicker(id)
		if err != nil {
			return nil, fmt.Errorf("Error reading %s price : %s\n", id, err)
		}

		p := Price{Exchange: "GDAX", Currency: "USD", ID: id[0:3], Ask: ticker.Ask, Bid: ticker.Bid}
		prices = append(prices, p)
	}

	return prices, nil

}

func getParibuPrices() ([]Price, error) {
	var prices []Price

	response, err := http.Get(PARIBU_URI)
	if err != nil {
		return nil, fmt.Errorf("failed to get Paribu response : %s", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Paribu response data : %s", err)
	}

	priceAsk, err := jsonparser.GetFloat(responseData, "BTC_TL", "lowestAsk")
	if err != nil {
		return nil, fmt.Errorf("failed to read the ask price from the Paribu response data: %s", err)
	}

	priceBid, err := jsonparser.GetFloat(responseData, "BTC_TL", "highestBid")
	if err != nil {
		return nil, fmt.Errorf("failed to read the bid price from the Paribu response data: %s", err)
	}

	prices = append(prices, Price{Exchange: PARIBU, Currency: "TRY", ID: "BTC", Ask: priceAsk, Bid: priceBid})
	return prices, nil
}

func getBTCTurkPrices() ([]Price, error) {
	var prices []Price

	response, err := http.Get(BTCTURK_URI)
	if err != nil {
		return nil, fmt.Errorf("failed to get BTCTurk response : %s", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read BTCTurk response data : %s", err)
	}

	btcPriceAsk, err := jsonparser.GetFloat(responseData, "[0]", "ask")
	if err != nil {
		return nil, fmt.Errorf("failed to read the BTC ask price from the BTCTurk response data: %s", err)
	}

	btcPriceBid, err := jsonparser.GetFloat(responseData, "[0]", "bid")
	if err != nil {
		return nil, fmt.Errorf("failed to read the BTC bid price from the BTCTurk response data: %s", err)
	}

	prices = append(prices, Price{Exchange: "BTCTurk", Currency: "TRY", ID: "BTC", Ask: btcPriceAsk, Bid: btcPriceBid})

	ethPriceAsk, err := jsonparser.GetFloat(responseData, "[2]", "ask")
	if err != nil {
		return nil, fmt.Errorf("failed to read the ETH ask price from the BTCTurk response data: %s", err)
	}

	ethPriceBid, err := jsonparser.GetFloat(responseData, "[2]", "bid")
	if err != nil {
		return nil, fmt.Errorf("failed to read the ETH bid price from the BTCTurk response data: %s", err)
	}

	prices = append(prices, Price{Exchange: BTCTURK, Currency: "TRY", ID: "ETH", Ask: ethPriceAsk, Bid: ethPriceBid})

	btcTurkETHBTCAskBid = ethPriceAsk / btcPriceBid
	btcTurkETHBTCBidAsk = ethPriceBid / btcPriceAsk

	return prices, nil
}

func getKoineksPrices() ([]Price, error) {
	var prices []Price

	response, err := http.Get(KOINEKS_URI)
	if err != nil {
		return nil, fmt.Errorf("failed to get Koineks response : %s", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Koineks response data : %s", err)
	}

	ids := []string{"BTC", "ETH", "LTC"}

	for _, id := range ids {

		priceAsk, err := jsonparser.GetString(responseData, id, "ask")
		if err != nil {
			return nil, fmt.Errorf("failed to read the ask price from the Koineks response data: %s", err)
		}

		askF, _ := strconv.ParseFloat(priceAsk, 64)

		priceBid, err := jsonparser.GetString(responseData, id, "bid")
		if err != nil {
			return nil, fmt.Errorf("failed to read the bid price from the Koineks response data: %s", err)
		}

		askB, _ := strconv.ParseFloat(priceBid, 64)

		prices = append(prices, Price{Exchange: KOINEKS, Currency: "TRY", ID: id, Ask: askF, Bid: askB})
	}

	return prices, nil
}

func getBitflyerPrices() ([]Price, error) {
	var prices []Price

	response, err := http.Get(BITFLYER_URI)
	if err != nil {
		return nil, fmt.Errorf("failed to get Bitflyer response : %s", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Bitflyer response data : %s", err)
	}

	priceAsk, err := jsonparser.GetFloat(responseData, "best_ask")
	if err != nil {
		return nil, fmt.Errorf("failed to read the ask price from the Bitflyer response data: %s", err)
	}

	priceBid, err := jsonparser.GetFloat(responseData, "best_bid")
	if err != nil {
		return nil, fmt.Errorf("failed to read the bid price from the Bitflyer response data: %s", err)
	}

	prices = append(prices, Price{Exchange: BITFLYER, Currency: "JPY", ID: "BTC", Ask: priceAsk, Bid: priceBid})

	return prices, nil
}
