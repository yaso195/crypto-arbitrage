package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/buger/jsonparser"

	gdax "github.com/preichenberger/go-gdax"
)

const (
	PARIBU_URI               = "https://www.paribu.com/ticker"
	BTCTURK_URI              = "https://www.btcturk.com/api/ticker"
	KOINEKS_URI              = "https://koineks.com/ticker"
	KOINIM_URI               = "http://koinim.com/api/v1/ticker/%s_TRY/"
	VEBITCOIN_URI            = "https://us-central1-vebitcoin-market.cloudfunctions.net/app/api/ticker"
	BINANCE_URI              = "https://api.binance.com/api/v3/ticker/bookTicker?symbol=%s%s"
	POLONIEX_URI             = "https://poloniex.com/public?command=returnTicker"
	POLONIEX_DOGE_VOLUME_URI = "https://poloniex.com/public?command=returnOrderBook&currencyPair=BTC_DOGE&depth=1"
	BITTREX_URI              = "https://bittrex.com/api/v1.1/public/getticker?market=%s-%s"
	BITTREX_DOGE_VOLUME_URI  = "https://bittrex.com/api/v1.1/public/getorderbook?market=BTC-DOGE&type=both"
	COINEGG_DOGE_VOLUME_URI  = "https://api.coinegg.im/api/v1/ticker/region/btc?coin=doge"

	GDAX      = "GDAX"
	BINANCE   = "Binance"
	BITTREX   = "Bittrex"
	POLONIEX  = "Poloniex"
	PARIBU    = "Paribu"
	BTCTURK   = "BTCTurk"
	KOINEKS   = "Koineks"
	KOINIM    = "Koinim"
	VEBITCOIN = "Vebitcoin"
)

var (
	symbolToExchangeNames map[string][]string

	ALL_EXCHANGES      = []string{PARIBU, BTCTURK, KOINEKS, KOINIM, VEBITCOIN}
	poloniexCurrencies = []string{"USDT", "DOGE", "DASH", "XRP", "STR", "XEM"}
	bittrexCurrencies  = []string{"USDT", "DOGE", "DASH", "XRP", "XLM", "XEM"}
	binanceCurrencies  = []string{"USDT", "DASH", "XRP", "XLM", "XEM"}
	gdaxCurrencies     = []string{"BTC-USD", "ETH-USD", "LTC-USD", "BCH-USD", "ETC-USD"}
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

	for _, id := range gdaxCurrencies {
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

	var returnError error
	jsonparser.ArrayEach(responseData, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		pairName, err := jsonparser.GetString(value, "pair")
		if err != nil {
			returnError = fmt.Errorf("failed to read BTCTurk pairname from the response data : %s", err)
			return
		}

		if !strings.Contains(pairName, "TRY") {
			return
		}

		var pair string
		if pairName != "USDTTRY" {
			pair = pairName[0:3]
		} else {
			pair = pairName[0:4]
		}

		priceAsk, err := jsonparser.GetFloat(value, "ask")
		if err != nil {
			returnError = fmt.Errorf("failed to read the %s ask price from the BTCTurk response data: %s", pair, err)
			return
		}

		priceBid, err := jsonparser.GetFloat(value, "bid")
		if err != nil {
			returnError = fmt.Errorf("failed to read the %s bid price from the BTCTurk response data: %s", pair, err)
			return
		}
		prices = append(prices, Price{Exchange: BTCTURK, Currency: "TRY", ID: pair, Ask: priceAsk, Bid: priceBid})

	})

	if returnError != nil {
		return nil, returnError
	}

	return prices, nil
}

func getKoinimPrices() ([]Price, error) {
	var prices []Price

	ids := []string{"BTC", "ETH", "LTC", "BCH", "DOGE"}
	for _, id := range ids {
		uri := fmt.Sprintf(KOINIM_URI, id)

		response, err := http.Get(uri)
		if err != nil {
			return nil, fmt.Errorf("failed to get Koinim response for %s: %s", id, err)
		}

		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read Koinim response data for %s: %s", id, err)
		}

		koinimPriceAsk, err := jsonparser.GetFloat(responseData, "ask")
		if err != nil {
			return nil, fmt.Errorf("failed to read the BTC ask price from the Koinim response data: %s", err)
		}

		koinimPriceBid, err := jsonparser.GetFloat(responseData, "bid")
		if err != nil {
			return nil, fmt.Errorf("failed to read the BTC bid price from the Koinim response data: %s", err)
		}

		prices = append(prices, Price{Exchange: KOINIM, Currency: "TRY", ID: id, Ask: koinimPriceAsk, Bid: koinimPriceBid})
	}

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

	ids := []string{"BTC", "ETH", "LTC", "BCH", "USDT", "ETC", "DOGE", "DASH", "XRP", "XLM", "XEM"}

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
	}

	return prices, nil
}

func getVebitcoinPrices() ([]Price, error) {
	var prices []Price

	response, err := http.Get(VEBITCOIN_URI)
	if err != nil {
		return nil, fmt.Errorf("failed to get Vebitcoin response: %s", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Vebitcoin response data: %s", err)
	}

	jsonparser.ArrayEach(responseData, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		targetCoin, errRet := jsonparser.GetString(value, "TargetCoinCode")
		if errRet != nil {
			err = fmt.Errorf("failed to find the code for target coin name in Vebitcoin: %s", errRet)
		}
		if targetCoin == "TRY" {
			sourceCoin, errRet := jsonparser.GetString(value, "SourceCoinCode")
			if errRet != nil {
				err = fmt.Errorf("failed to find the code for source coin name in Vebitcoin: %s", errRet)
			}
			// Vebitcoin has a bug in their API, the ask price is given in the "Bid" field, bid price is given in their
			// "Ask" field.
			pAsk, errRet := jsonparser.GetFloat(value, "Bid")
			if errRet != nil {
				err = fmt.Errorf("failed to find the ask price for %s in Vebitcoin: %s", sourceCoin, errRet)
			}
			pBid, errRet := jsonparser.GetFloat(value, "Ask")
			if errRet != nil {
				err = fmt.Errorf("failed to find the bid price for %s in Vebitcoin: %s", sourceCoin, errRet)
			}
			prices = append(prices, Price{Exchange: VEBITCOIN, Currency: "TRY", ID: sourceCoin, Ask: pAsk, Bid: pBid})
		}
	})

	return prices, err
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
		var responseSymbol string
		if currency == "USDT" {
			responseSymbol = fmt.Sprintf("%s_BTC", currency)
		} else {
			responseSymbol = fmt.Sprintf("BTC_%s", currency)
		}
		priceAsk, err := jsonparser.GetString(responseData, responseSymbol, "lowestAsk")
		if err != nil {
			return nil, fmt.Errorf("failed to read the ask price from the Poloniex response data: %s", err)
		}
		pAsk, _ := strconv.ParseFloat(priceAsk, 64)

		priceBid, err := jsonparser.GetString(responseData, responseSymbol, "highestBid")
		if err != nil {
			return nil, fmt.Errorf("failed to read the bid price from the Poloniex response data: %s", err)
		}
		pBid, _ := strconv.ParseFloat(priceBid, 64)

		if currency == "STR" {
			currency = "XLM"
		}
		prices[currency] = Price{Exchange: POLONIEX, Currency: "USD", ID: currency, Ask: pAsk, Bid: pBid}
		spreads[POLONIEX+currency] = (pAsk - pBid) * 100 / pBid
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

		var uri string
		if currency == "USDT" {
			uri = fmt.Sprintf(BITTREX_URI, "USD", currency)
		} else {
			uri = fmt.Sprintf(BITTREX_URI, "BTC", currency)
		}

		response, err := http.Get(uri)
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

		prices[currency] = Price{Exchange: BITTREX, Currency: "USD", ID: currency, Ask: pAsk, Bid: pBid}
		spreads[BITTREX+currency] = (pAsk - pBid) * 100 / pBid
	}

	return prices, nil
}

func getBittrexDOGEVolumes() error {
	response, err := http.Get(BITTREX_DOGE_VOLUME_URI)
	if err != nil {
		return fmt.Errorf("failed to get Bittrex DOGE volume response : %s", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read Bittrex DOGE volume response data : %s", err)
	}

	pAsk, err := jsonparser.GetFloat(responseData, "result", "sell", "[0]", "Rate")
	if err != nil {
		return fmt.Errorf("failed to read the DOGE ask price from the Bittrex response data: %s", err)
	}

	askVolumeSize, err := jsonparser.GetFloat(responseData, "result", "sell", "[0]", "Quantity")
	if err != nil {
		return fmt.Errorf("failed to read the DOGE ask volume size from the Bittrex response data: %s", err)
	}

	pBid, err := jsonparser.GetFloat(responseData, "result", "buy", "[0]", "Rate")
	if err != nil {
		return fmt.Errorf("failed to read the DOGE bid price from the Bittrex response data: %s", err)
	}

	bidVolumeSize, err := jsonparser.GetFloat(responseData, "result", "buy", "[0]", "Quantity")
	if err != nil {
		return fmt.Errorf("failed to read the DOGE bid volume size from the Bittrex response data: %s", err)
	}

	dogeVolumes["BittrexAsk"] = pAsk * askVolumeSize
	dogeVolumes["BittrexBid"] = pBid * bidVolumeSize
	prices["BittrexDOGEAsk"] = pAsk
	prices["BittrexDOGEBid"] = pBid

	return nil
}

func getCoineggDOGEPrices() error {
	response, err := http.Get(COINEGG_DOGE_VOLUME_URI)
	if err != nil {
		return fmt.Errorf("failed to get Coinegg DOGE price response : %s", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read Coinegg DOGE price response data : %s", err)
	}

	pBidStr, err := jsonparser.GetString(responseData, "buy")
	if err != nil {
		return fmt.Errorf("failed to read the DOGE bid price from the Coinegg response data: %s", err)
	}
	pBid, _ := strconv.ParseFloat(pBidStr, 64)

	pAskStr, err := jsonparser.GetString(responseData, "sell")
	if err != nil {
		return fmt.Errorf("failed to read the DOGE ask price from the Coinegg response data: %s", err)
	}
	pAsk, _ := strconv.ParseFloat(pAskStr, 64)

	prices["CoineggDOGEAsk"] = pAsk
	prices["CoineggDOGEBid"] = pBid

	return nil
}

func getBinancePrices() (map[string]Price, error) {
	prices := map[string]Price{}

	for _, currency := range binanceCurrencies {
		var uri string
		if currency == "USDT" {
			uri = fmt.Sprintf(BINANCE_URI, "BTC", currency)
		} else {
			uri = fmt.Sprintf(BINANCE_URI, currency, "BTC")
		}

		response, err := http.Get(uri)
		if err != nil {
			return nil, fmt.Errorf("failed to get Binance response : %s", err)
		}

		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read Binance response data : %s", err)
		}

		priceAsk, err := jsonparser.GetString(responseData, "askPrice")
		if err != nil {
			return nil, fmt.Errorf("failed to read the ask price from the Binance response data: %s", err)
		}
		pAsk, _ := strconv.ParseFloat(priceAsk, 64)

		priceBid, err := jsonparser.GetString(responseData, "bidPrice")
		if err != nil {
			return nil, fmt.Errorf("failed to read the bid price from the Binance response data: %s", err)
		}
		pBid, _ := strconv.ParseFloat(priceBid, 64)

		prices[currency] = Price{Exchange: BINANCE, Currency: "USD", ID: currency, Ask: pAsk, Bid: pBid}
		spreads[BINANCE+currency] = (pAsk - pBid) * 100 / pBid
	}

	return prices, nil
}
