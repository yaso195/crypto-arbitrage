package server

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/buger/jsonparser"

	coinbasepro "github.com/preichenberger/go-coinbasepro"
	ws "github.com/gorilla/websocket"

)

const (
	PARIBU_URI               = "https://www.paribu.com/ticker"
	BTCTURK_URI              = "https://api.btcturk.com/api/v2/ticker"
	KOINEKS_URI              = "https://api.thodex.com/v1/public/order-depth?market=%sTRY&limit=1"
	KOINIM_URI               = "http://koinim.com/api/v1/ticker/%s_TRY/"
	VEBITCOIN_URI            = "https://prod-data-publisher.azurewebsites.net/api/ticker"
	BINANCE_URI              = "https://api.binance.com/api/v3/ticker/bookTicker?symbol=%s%s"
	BITTREX_URI              = "https://bittrex.com/api/v1.1/public/getticker?market=%s-%s"
	BITTREX_DOGE_VOLUME_URI  = "https://bittrex.com/api/v1.1/public/getorderbook?market=BTC-DOGE&type=both"
	BITOASIS_URI             = "https://api.bitoasis.net/v1/exchange/ticker/%s-AED"
	BITFINEX_URI             = "https://api.bitfinex.com/v1/pubticker/%sUSD"
	CEXIO_URI                = "https://cex.io/api/ticker/%s/USD"

	GDAX      = "GDAX"
	BINANCE   = "Binance"
	BITOASIS  = "Bitoasis"
	BITTREX   = "Bittrex"
	BITFINEX  = "Bitfinex"
	CEXIO     = "Cexio"
	PARIBU    = "Paribu"
	BTCTURK   = "BTCTurk"
	KOINEKS   = "Koineks"
	KOINIM    = "Koinim"
	VEBITCOIN = "Vebitcoin"
)

var (
	symbolToExchangeNames map[string][]string

	ALL_EXCHANGES      = []string{PARIBU, BTCTURK, KOINEKS, KOINIM, VEBITCOIN}
	bittrexCurrencies  = []string{"USDT", "DOGE", "XRP", "XLM", "XEM"}
	binanceCurrencies  = []string{"USDT", "DOGE", "XEM"}
	bitoasisCurrencies = []string{"BTC", "ETH", "LTC", "XLM", "XRP", "BCH"}
	coinbaseProCurrencies = []string{
		"BTC-USD", "BCH-USD", "ETH-USD", "LTC-USD", "ETC-USD", "ZRX-USD", "XRP-USD", "XLM-USD", "EOS-USD", "LINK-USD",
		"DASH-USD",
	}

	bitfinexCurrencies = []string{"BTC", "ETH", "LTC", "XRP", "XLM"}
	cexioCurrencies    = []string{"BTC", "ETH", "LTC", "BCH", "XRP", "XLM"}

	wsDialer ws.Dialer
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

	coinbaseProPrices = map[string]*Price{}
	for _, symbol := range ALL_SYMBOLS {

		binanceCurrency := false
		for _, c := range binanceCurrencies {
			if c == symbol {
				binanceCurrency = true
				break
			}
		}
		exchange := GDAX
		if binanceCurrency {
			exchange = BINANCE
		}

		coinbaseProPrices[symbol] = &Price{Exchange: exchange, Currency: "USD", ID: symbol}

	}

	diffs = map[string]float64{}
	prices = map[string]float64{}
	spreads = map[string]float64{}
	dogeVolumes = map[string]float64{}

	minDiffs, maxDiffs = map[string]float64{}, map[string]float64{}
	minSymbol, maxSymbol = map[string]string{}, map[string]string{}

	PUSHOVER_USER = os.Getenv("PUSHOVER_USER")
	PUSHOVER_APP_TOKEN = os.Getenv("PUSHOVER_APP_TOKEN")
}

func startCoinbaseProWS() error {
	var wsDialer ws.Dialer
  wsConn, _, err := wsDialer.Dial("wss://ws-feed.pro.coinbase.com", nil)
  if err != nil {
    println(err.Error())
    return err
  }

  subscribe := coinbasepro.Message{
    Type:      "subscribe",
    Channels: []coinbasepro.MessageChannel{
      coinbasepro.MessageChannel{
        Name: "ticker",
        ProductIds: coinbaseProCurrencies,
      },
    },
  }
  if err := wsConn.WriteJSON(subscribe); err != nil {
    println(err.Error())
    return err
  }

  for true {
    message := coinbasepro.Message{}
    if err := wsConn.ReadJSON(&message); err != nil {
      println(err.Error())
      log.Println(fmt.Sprintf("Cannot read coinbase pro messages : ", err.Error()))
      continue
    }

		if message.ProductID !=  "" {
	    id := message.ProductID
	    tempID := ""
			if strings.HasSuffix(id, "-USD") {
				tempID = id[0:len(id)-4]
			}

			pAsk, _ := strconv.ParseFloat(message.BestAsk, 64)
			pBid, _ := strconv.ParseFloat(message.BestBid, 64)
			spreads[GDAX+tempID] = (pAsk - pBid) * 100 / pBid

			p, ok := coinbaseProPrices[tempID]
			if !ok {
				coinbaseProPrices[tempID] = &Price{Exchange: GDAX, Currency: "USD", ID: tempID, Ask: pAsk, Bid: pBid}
			} else {
				p.Ask = pAsk
				p.Bid = pBid
			}
		}
  }

  return nil
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

	ids := []string{"BTC", "ETH", "LTC", "BCH", "DOGE", "XRP", "XLM", "EOS", "USDT", "LINK"}
	for _, id := range ids {
		priceAsk, err := jsonparser.GetFloat(responseData, fmt.Sprintf("%s_TL", id), "lowestAsk")
		if err != nil {
			return nil, fmt.Errorf("failed to read the ask price from the Paribu response data: %s", err)
		}

		priceBid, err := jsonparser.GetFloat(responseData, fmt.Sprintf("%s_TL", id), "highestBid")
		if err != nil {
			return nil, fmt.Errorf("failed to read the bid price from the Paribu response data: %s", err)
		}

		prices = append(prices, Price{Exchange: PARIBU, Currency: "TRY", ID: id, Ask: priceAsk, Bid: priceBid})
	}
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

		if !strings.HasSuffix(pairName, "TRY") {
			return
		}

		var pair string
		if pairName == "USDTTRY" || pairName == "LINKTRY" {
			pair = pairName[0:4]
		} else {
			pair = pairName[0:3]
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

	}, "data")

	if returnError != nil {
		return nil, returnError
	}

	return prices, nil
}

func getKoinimPrices() ([]Price, error) {
	var prices []Price

	ids := []string{"BTC", "ETH", "LTC", "BCH", "DOGE", "DASH"}
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

	ids := []string{"BTC", "ETH", "LTC", "BCH", "USDT", "ETC", "DOGE", "XRP", "XLM", "EOS", "XEM", "DASH"}

	for _, id := range ids {

		response, err := http.Get(fmt.Sprintf(KOINEKS_URI, id))
		if err != nil {
			return nil, fmt.Errorf("failed to get Koineks response : %s", err)
		}

		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read Koineks response data : %s", err)
		}

		priceAsk, err := jsonparser.GetString(responseData, "result", "asks", "[0]", "[0]")
		if err != nil {
			return nil, fmt.Errorf("failed to read the ask price from the Koineks response data: %s", err)
		}

		pAsk, _ := strconv.ParseFloat(priceAsk, 64)

		priceBid, err := jsonparser.GetString(responseData, "result", "bids", "[0]", "[0]")
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
			pAsk, errRet := jsonparser.GetFloat(value, "Ask")
			if errRet != nil {
				err = fmt.Errorf("failed to find the ask price for %s in Vebitcoin: %s", sourceCoin, errRet)
			}
			pBid, errRet := jsonparser.GetFloat(value, "Bid")
			if errRet != nil {
				err = fmt.Errorf("failed to find the bid price for %s in Vebitcoin: %s", sourceCoin, errRet)
			}
			prices = append(prices, Price{Exchange: VEBITCOIN, Currency: "TRY", ID: sourceCoin, Ask: pAsk, Bid: pBid})
		}
	})

	return prices, err
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

	mux.Lock()
	prices["BittrexDOGEAsk"] = pAsk
	prices["BittrexDOGEBid"] = pBid
	mux.Unlock()

	return nil
}

func getBinanceDOGEVolumes() error {
	uri := fmt.Sprintf(BINANCE_URI, "DOGE", "BTC")
	response, err := http.Get(uri)
	if err != nil {
		return fmt.Errorf("failed to get Binance DOGE volume response : %s", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read Binance DOGE volume response data : %s", err)
	}

	priceAsk, err := jsonparser.GetString(responseData, "askPrice")
	if err != nil {
		return fmt.Errorf("failed to read the ask price from the Binance response data: %s", err)
	}
	pAsk, _ := strconv.ParseFloat(priceAsk, 64)

	askVolumeSizeStr, err := jsonparser.GetString(responseData, "askQty")
	if err != nil {
		return fmt.Errorf("failed to read the DOGE ask volume size from the Binance response data: %s", err)
	}
	askVolumeSize, _ := strconv.ParseFloat(askVolumeSizeStr, 64)

	priceBid, err := jsonparser.GetString(responseData, "bidPrice")
	if err != nil {
		return fmt.Errorf("failed to read the bid price from the Binance response data: %s", err)
	}
	pBid, _ := strconv.ParseFloat(priceBid, 64)

	bidVolumeSizeStr, err := jsonparser.GetString(responseData, "bidQty")
	if err != nil {
		return fmt.Errorf("failed to read the DOGE bid volume size from the Binance response data: %s", err)
	}
	bidVolumeSize, _ := strconv.ParseFloat(bidVolumeSizeStr, 64)

	dogeVolumes["BinanceAsk"] = pAsk * askVolumeSize
	dogeVolumes["BinanceBid"] = pBid * bidVolumeSize

	mux.Lock()
	prices["BinanceDOGEAsk"] = pAsk
	prices["BinanceDOGEBid"] = pBid
	mux.Unlock()

	return nil
}

func getBinancePrices() (map[string]Price, error) {
	prices := map[string]Price{}

	for _, currency := range binanceCurrencies {
		var uri string
		if currency == "USDT" {
			uri = fmt.Sprintf(BINANCE_URI, "USDC", currency)
		} else if currency == "XRP" || currency == "XLM" {
			uri = fmt.Sprintf(BINANCE_URI, currency, "USDC")
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

		if currency == "USDT" {
			prices[currency] = Price{Exchange: BINANCE, Currency: "USD", ID: currency, Ask: 1 / pAsk, Bid: 1 / pBid}
		} else {
			prices[currency] = Price{Exchange: BINANCE, Currency: "USD", ID: currency, Ask: pAsk, Bid: pBid}
		}

		mux.Lock()
		spreads[BINANCE+currency] = (pAsk - pBid) * 100 / pBid
		mux.Unlock()
	}

	return prices, nil
}

func getBitoasisPrices() ([]Price, error) {
	prices := []Price{}

	for _, currency := range bitoasisCurrencies {

		uri := fmt.Sprintf(BITOASIS_URI, currency)

		response, err := http.Get(uri)
		if err != nil {
			return nil, fmt.Errorf("failed to get BitoasIs response : %s", err)
		}

		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read Bitoasis response data : %s", err)
		}

		priceAsk, err := jsonparser.GetString(responseData, "ticker", "ask")
		if err != nil {
			return nil, fmt.Errorf("failed to read the ask price from the Bitoasis response data: %s", err)
		}
		pAsk, _ := strconv.ParseFloat(priceAsk, 64)

		priceBid, err := jsonparser.GetString(responseData, "ticker", "bid")
		if err != nil {
			return nil, fmt.Errorf("failed to read the bid price from the Bitoasis response data: %s", err)
		}
		pBid, _ := strconv.ParseFloat(priceBid, 64)

		prices = append(prices, Price{Exchange: BITOASIS, Currency: "AED", ID: currency, Ask: pAsk, Bid: pBid})
	}

	return prices, nil
}

func getBitfinexPrices() ([]Price, error) {
	prices := []Price{}

	for _, currency := range bitfinexCurrencies {
		uri := fmt.Sprintf(BITFINEX_URI, currency)
		response, err := http.Get(uri)
		if err != nil {
			return nil, fmt.Errorf("failed to get Bitfinex response : %s", err)
		}

		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read Bitfinex response data : %s", err)
		}

		priceAsk, err := jsonparser.GetString(responseData, "ask")
		if err != nil {
			return nil, fmt.Errorf("failed to read the ask price from the Bitfinex response data: %s", err)
		}
		pAsk, _ := strconv.ParseFloat(priceAsk, 64)

		priceBid, err := jsonparser.GetString(responseData, "bid")
		if err != nil {
			return nil, fmt.Errorf("failed to read the bid price from the Bitfinex response data: %s", err)
		}
		pBid, _ := strconv.ParseFloat(priceBid, 64)

		prices = append(prices, Price{Exchange: BITFINEX, Currency: "USD", ID: currency, Ask: pAsk, Bid: pBid})
	}

	return prices, nil
}

func getCexioPrices() ([]Price, error) {
	prices := []Price{}

	for _, currency := range cexioCurrencies {
		uri := fmt.Sprintf(CEXIO_URI, currency)
		response, err := http.Get(uri)
		if err != nil {
			return nil, fmt.Errorf("failed to get Cexio response : %s", err)
		}

		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read Cexio response data : %s", err)
		}

		pAsk, err := jsonparser.GetFloat(responseData, "ask")
		if err != nil {
			return nil, fmt.Errorf("failed to read the ask price from the Cexio response data: %s", err)
		}

		pBid, err := jsonparser.GetFloat(responseData, "bid")
		if err != nil {
			return nil, fmt.Errorf("failed to read the bid price from the Cexio response data: %s", err)
		}

		prices = append(prices, Price{Exchange: CEXIO, Currency: "USD", ID: currency, Ask: pAsk, Bid: pBid})
	}

	return prices, nil
}
