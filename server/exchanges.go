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
	PARIBU_URI               = "https://www.paribu.com/ticker"
	BTCTURK_URI              = "https://www.btcturk.com/api/ticker"
	KOINEKS_URI              = "https://koineks.com/ticker"
	KOINIM_URI               = "https://koinim.com/ticker"
	BITFLYER_URI             = "https://api.bitflyer.jp/v1/ticker"
	POLONIEX_URI             = "https://poloniex.com/public?command=returnTicker"
	POLONIEX_DOGE_VOLUME_URI = "https://poloniex.com/public?command=returnOrderBook&currencyPair=BTC_DOGE&depth=1"
	BITTREX_URI              = "https://bittrex.com/api/v1.1/public/getticker?market=BTC-%s"

	GDAX     = "GDAX"
	PARIBU   = "Paribu"
	BTCTURK  = "BTCTurk"
	KOINEKS  = "Koineks"
	KOINIM   = "Koinim"
	BITFLYER = "Bitflyer"
)

var (
	symbolToExchangeNames map[string][]string

	ALL_EXCHANGES      = []string{PARIBU, BTCTURK, KOINEKS, KOINIM, BITFLYER}
	poloniexCurrencies = []string{"DOGE", "DASH", "XRP", "STR", "XEM"}
	bittrexCurrencies  = []string{"DOGE", "DASH", "XRP", "XLM", "XEM"}
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
	crossDiffs = map[string]float64{}
	prices = map[string]float64{}
	spreads = map[string]float64{}
	dogeVolumes = map[string]float64{}
	usdPrices = map[string]Price{}

	minDiffs, maxDiffs = map[string]float64{}, map[string]float64{}
	minSymbol, maxSymbol = map[string]string{}, map[string]string{}

	PUSHOVER_USER = os.Getenv("PUSHOVER_USER")
	PUSHOVER_APP_TOKEN = os.Getenv("PUSHOVER_APP_TOKEN")
}

func getGdaxPrices() ([]Price, error) {

	client := gdax.NewClient("", "", "")
	var prices []Price

	ids := []string{"BTC-USD", "ETH-USD", "LTC-USD", "ETH-BTC", "LTC-BTC"}

	for _, id := range ids {
		ticker, err := client.GetTicker(id)
		if err != nil {
			return nil, fmt.Errorf("Error reading %s price : %s\n", id, err)
		}

		tempID := id
		if id[4:] == "USD" {
			tempID = id[0:3]
		}

		p := Price{Exchange: GDAX, Currency: "USD", ID: tempID, Ask: ticker.Ask, Bid: ticker.Bid}
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

	xrpPriceAsk, err := jsonparser.GetFloat(responseData, "[3]", "ask")
	if err != nil {
		return nil, fmt.Errorf("failed to read the XRP ask price from the BTCTurk response data: %s", err)
	}

	xrpPriceBid, err := jsonparser.GetFloat(responseData, "[3]", "bid")
	if err != nil {
		return nil, fmt.Errorf("failed to read the XRP bid price from the BTCTurk response data: %s", err)
	}

	prices = append(prices, Price{Exchange: BTCTURK, Currency: "TRY", ID: "XRP", Ask: xrpPriceAsk, Bid: xrpPriceBid})

	btcTurkETHBTCAskBid = ethPriceAsk / btcPriceBid
	btcTurkETHBTCBidAsk = ethPriceBid / btcPriceAsk

	return prices, nil
}

func getKoinimPrices() ([]Price, error) {
	var prices []Price

	response, err := http.Get(KOINIM_URI)
	if err != nil {
		return nil, fmt.Errorf("failed to get Koinim response : %s", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Koinim response data : %s", err)
	}

	koinimPriceAsk, err := jsonparser.GetFloat(responseData, "ask")
	if err != nil {
		return nil, fmt.Errorf("failed to read the BTC ask price from the Koinim response data: %s", err)
	}

	koinimPriceBid, err := jsonparser.GetFloat(responseData, "bid")
	if err != nil {
		return nil, fmt.Errorf("failed to read the BTC bid price from the Koinim response data: %s", err)
	}

	prices = append(prices, Price{Exchange: KOINIM, Currency: "TRY", ID: "BTC", Ask: koinimPriceAsk, Bid: koinimPriceBid})

	response, err = http.Get(KOINIM_URI + "/ltc")
	if err != nil {
		return nil, fmt.Errorf("failed to get Koinim response : %s", err)
	}

	responseData, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Koinim response data : %s", err)
	}

	ltcPriceAsk, err := jsonparser.GetFloat(responseData, "ask")
	if err != nil {
		return nil, fmt.Errorf("failed to read the LTC ask price from the Koinim response data: %s", err)
	}

	ltcPriceBid, err := jsonparser.GetFloat(responseData, "bid")
	if err != nil {
		return nil, fmt.Errorf("failed to read the LTC bid price from the Koinim response data: %s", err)
	}

	prices = append(prices, Price{Exchange: KOINIM, Currency: "TRY", ID: "LTC", Ask: ltcPriceAsk, Bid: ltcPriceBid})

	koinimLTCBTCAskBid = ltcPriceAsk / koinimPriceBid
	koinimLTCBTCBidAsk = ltcPriceBid / koinimPriceAsk

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

	ids := []string{"BTC", "ETH", "LTC", "DOGE", "DASH", "XRP", "XLM", "XEM"}

	var btcPriceAsk, btcPriceBid float64
	for _, id := range ids {

		priceAsk, err := jsonparser.GetString(responseData, id, "ask")
		if err != nil {
			return nil, fmt.Errorf("failed to read the ask price from the Koineks response data: %s", err)
		}

		pAsk, _ := strconv.ParseFloat(priceAsk, 64)

		priceBid, err := jsonparser.GetString(responseData, id, "bid")
		if err != nil {
			return nil, fmt.Errorf("failed to read the bid price from the Koineks response data: %s", err)
		}

		pBid, _ := strconv.ParseFloat(priceBid, 64)

		prices = append(prices, Price{Exchange: KOINEKS, Currency: "TRY", ID: id, Ask: pAsk, Bid: pBid})

		switch id {
		case "BTC":
			btcPriceAsk = pAsk
			btcPriceBid = pBid
		case "ETH":
			koineksETHBTCAskBid = pAsk / btcPriceBid
			koineksETHBTCBidAsk = pBid / btcPriceAsk
		case "LTC":
			koineksLTCBTCAskBid = pAsk / btcPriceBid
			koineksLTCBTCBidAsk = pBid / btcPriceAsk
		}
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

func getPoloniexPrices() (map[string]Price, error) {
	prices := map[string]Price{}

	response, err := http.Get(POLONIEX_URI)
	if err != nil {
		return nil, fmt.Errorf("failed to get Poloniex response : %s", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Poloniex response data : %s", err)
	}

	for _, currency := range poloniexCurrencies {
		priceAsk, err := jsonparser.GetString(responseData, fmt.Sprintf("BTC_%s", currency), "lowestAsk")
		if err != nil {
			return nil, fmt.Errorf("failed to read the ask price from the Poloniex response data: %s", err)
		}
		pAsk, _ := strconv.ParseFloat(priceAsk, 64)

		priceBid, err := jsonparser.GetString(responseData, fmt.Sprintf("BTC_%s", currency), "highestBid")
		if err != nil {
			return nil, fmt.Errorf("failed to read the bid price from the Poloniex response data: %s", err)
		}
		pBid, _ := strconv.ParseFloat(priceBid, 64)

		if currency == "STR" {
			currency = "XLM"
		}
		prices[currency] = Price{Exchange: GDAX, Currency: "USD", ID: currency, Ask: pAsk, Bid: pBid}
		spreads[currency] = (pAsk - pBid) * 100 / pBid
	}

	return prices, nil
}

func getPoloniexDOGEVolumes() error {
	response, err := http.Get(POLONIEX_DOGE_VOLUME_URI)
	if err != nil {
		return fmt.Errorf("failed to get Poloniex DOGE volume response : %s", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read Poloniex DOGE volume response data : %s", err)
	}

	priceAsk, err := jsonparser.GetString(responseData, "asks", "[0]", "[0]")
	if err != nil {
		return fmt.Errorf("failed to read the DOGE ask price from the Poloniex response data: %s", err)
	}
	pAsk, _ := strconv.ParseFloat(priceAsk, 64)

	askVolumeSize, err := jsonparser.GetFloat(responseData, "asks", "[0]", "[1]")
	if err != nil {
		return fmt.Errorf("failed to read the DOGE ask volume size from the Poloniex response data: %s", err)
	}

	priceBid, err := jsonparser.GetString(responseData, "bids", "[0]", "[0]")
	if err != nil {
		return fmt.Errorf("failed to read the DOGE bid price from the Poloniex response data: %s", err)
	}
	pBid, _ := strconv.ParseFloat(priceBid, 64)

	bidVolumeSize, err := jsonparser.GetFloat(responseData, "bids", "[0]", "[1]")
	if err != nil {
		return fmt.Errorf("failed to read the DOGE bid volume size from the Poloniex response data: %s", err)
	}

	dogeVolumes["PoloniexAsk"] = pAsk * askVolumeSize
	dogeVolumes["PoloniexBid"] = pBid * bidVolumeSize
	prices["PoloniexDOGEAsk"] = pAsk
	prices["PoloniexDOGEBid"] = pBid

	return nil
}

func getBittrexPrices() (map[string]Price, error) {
	prices := map[string]Price{}

	for _, currency := range bittrexCurrencies {

		response, err := http.Get(fmt.Sprintf(BITTREX_URI, currency))
		if err != nil {
			return nil, fmt.Errorf("failed to get Bittrex response : %s", err)
		}

		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read Bittrex response data : %s", err)
		}

		pAsk, err := jsonparser.GetFloat(responseData, "result", "Ask")
		if err != nil {
			return nil, fmt.Errorf("failed to read the ask price from the Bittrex response data: %s", err)
		}

		pBid, err := jsonparser.GetFloat(responseData, "result", "Bid")
		if err != nil {
			return nil, fmt.Errorf("failed to read the bid price from the Bittrex response data: %s", err)
		}

		prices[currency] = Price{Exchange: GDAX, Currency: "USD", ID: currency, Ask: pAsk, Bid: pBid}
		spreads[currency] = (pAsk - pBid) * 100 / pBid
	}

	return prices, nil
}
