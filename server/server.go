package server

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Price struct {
	Exchange string
	Currency string
	ID       string
	Ask      float64
	Bid      float64
}

const (
	BASE_CURRENCY_URI = "https://api.apilayer.com/exchangerates_data/latest?symbols=TRY&base=USD"
)

var (
	diffs                                                                              map[string]float64
	prices, spreads                                                                    map[string]float64
	minDiffs, maxDiffs                                                                 map[string]float64
	dogeVolumes                                                                        map[string]float64
	minSymbol, maxSymbol                                                               map[string]string
	binancePrices                				 										 					 								 map[string]Price
	coinbaseProPrices               				 																					 map[string]*Price
	paribuPrices,btcTurkPrices, koineksPrices, koinimPrices, vebitcoinPrices 					 []Price
	btcTurkETHBTCAskBid, btcTurkETHBTCBidAsk                                           float64
	koineksETHBTCAskBid, koineksETHBTCBidAsk, koineksLTCBTCAskBid, koineksLTCBTCBidAsk float64
	koinimLTCBTCAskBid, koinimLTCBTCBidAsk                                             float64
	fiatNotificationEnabled                                                            = true
	warning                                                                            string

	mux sync.Mutex

	ALL_SYMBOLS = []string{"BTC", "ETH", "LTC", "BCH", "ETC", "ZRX", "XLM", "EOS", "USDT", "DOGE", "LINK", "DASH", "ZEC", "MKR", "BAT", "ADA"}
)

func Run() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*")

	router.GET("/", PrintTableWithBinance)
	router.GET("/notification", SetNotificationLimits)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		router.Run(":" + port)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		getCurrencies()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		startCoinbaseProWS()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		getPrices()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		calculateDiffs()
	}()

	wg.Wait()
}

func getPrices() {
	for {
		calculatePrices()
		time.Sleep(2 * time.Second)
	}
}

func calculateDiffs() {
	for {
		findAltcoinPrices(binancePrices, paribuPrices, btcTurkPrices, koineksPrices, koinimPrices, vebitcoinPrices)
		sendMessages()
		resetDiffsAndSymbols()
		time.Sleep(1 * time.Second)
	}
}

func calculatePrices() {
	var err error

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		binancePrices, err = getBinancePrices()
		if err != nil || len(binancePrices) != len(binanceCurrencies) {
			message := fmt.Sprintf("Error reading Binance prices : %s", err)
			warning += message + "\n"
			fmt.Println(message)
			log.Println(message)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		paribuPrices, err = getParibuPrices()
		if err != nil {
			message := fmt.Sprintf("Error reading Paribu prices : %s", err)
			warning += message + "\n"
			fmt.Println(message)
			log.Println(message)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		btcTurkPrices, err = getBTCTurkPrices()
		if err != nil {
			message := fmt.Sprintf("Error reading BTCTurk prices : %s", err)
			warning += message + "\n"
			fmt.Println(message)
			log.Println(message)
		}
	}()

	/*wg.Add(1)
	go func() {
		defer wg.Done()
		koineksPrices, err = getKoineksPrices()
		if err != nil {
			message := fmt.Sprintf("Error reading Koineks prices : %s", err)
			warning += message + "\n"
			fmt.Println(message)
			log.Println(message)
		}
	}()*/

	wg.Add(1)
	go func() {
		defer wg.Done()
		koinimPrices, err = getKoinimPrices()
		if err != nil {
			message := fmt.Sprintf("Error reading Koinim prices : %s", err)
			warning += message + "\n"
			fmt.Println(message)
			log.Println(message)
		}
	}()

	/*wg.Add(1)
	go func() {
		defer wg.Done()
		vebitcoinPrices, err = getVebitcoinPrices()
		if err != nil {
			message := fmt.Sprintf("Error reading Vebitcoin prices : %s", err)
			warning += message + "\n"
			fmt.Println(message)
			log.Println(message)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := getBittrexDOGEVolumes(); err != nil {
			message := fmt.Sprintf("Error reading Bittrex DOGE volumes : %s", err)
			warning += message + "\n"
			fmt.Println(message)
			log.Println(message)
		}

		if err := getBinanceDOGEVolumes(); err != nil {
			message := fmt.Sprintf("Error reading Binance DOGE volumes : %s", err)
			warning += message + "\n"
			fmt.Println(message)
			log.Println(message)
		}
	}()*/
	wg.Wait()
}

func findAltcoinPrices(exchangePrices map[string]Price, sellExchanges ...[]Price) {
	bitcoinPrice := coinbaseProPrices["BTC"].Ask
	for _, p := range exchangePrices {
		multiplier := 1.0
		if p.ID != "USDT" {
			multiplier = bitcoinPrice
		}

		tempP, ok := coinbaseProPrices[p.ID]
		if !ok {
			coinbaseProPrices[p.ID] = &Price{Exchange: p.Exchange, Currency: p.Currency, ID: p.ID, Ask: p.Ask*multiplier, Bid: p.Bid*multiplier}
		} else {
			tempP.Ask = p.Ask*multiplier
			tempP.Bid = p.Bid*multiplier
		}
	}

	var newPriceList [][]Price
	for _, list := range sellExchanges {
		newPriceList = append(newPriceList, list)
	}
	findPriceDifferences(newPriceList...)
}

func PrintTableWithBinance(c *gin.Context) {
	printTable(c, binancePrices, BINANCE)
}

func printTable(c *gin.Context, crossPrices map[string]Price, exchange string) {
	mux.Lock()
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"USDTRY":                tryRate,
		"USDAED":                aedRate,
		"GdaxBTC":               coinbaseProPrices["BTC"].Ask,
		"ParibuBTCAsk":          diffs[GDAX+"-Paribu-BTC-Ask"],
		"ParibuBTCBid":          diffs[GDAX+"-Paribu-BTC-Bid"],
		"BTCTurkBTCAsk":         diffs[GDAX+"-BTCTurk-BTC-Ask"],
		"BTCTurkBTCBid":         diffs[GDAX+"-BTCTurk-BTC-Bid"],
		"KoineksBTCAsk":         diffs[GDAX+"-Koineks-BTC-Ask"],
		"KoineksBTCBid":         diffs[GDAX+"-Koineks-BTC-Bid"],
		"KoinimBTCAsk":          diffs[GDAX+"-Koinim-BTC-Ask"],
		"KoinimBTCBid":          diffs[GDAX+"-Koinim-BTC-Bid"],
		"VebitcoinBTCAsk":       diffs[GDAX+"-Vebitcoin-BTC-Ask"],
		"VebitcoinBTCBid":       diffs[GDAX+"-Vebitcoin-BTC-Bid"],
		"BitfinexBTCAsk":        diffs[GDAX+"-Bitfinex-BTC-Ask"],
		"BitfinexBTCBid":        diffs[GDAX+"-Bitfinex-BTC-Bid"],
		"GdaxETH":               coinbaseProPrices["ETH"].Ask,
		"ParibuETHAsk":          diffs[GDAX+"-Paribu-ETH-Ask"],
		"ParibuETHBid":          diffs[GDAX+"-Paribu-ETH-Bid"],
		"BTCTurkETHAsk":         diffs[GDAX+"-BTCTurk-ETH-Ask"],
		"BTCTurkETHBid":         diffs[GDAX+"-BTCTurk-ETH-Bid"],
		"KoineksETHAsk":         diffs[GDAX+"-Koineks-ETH-Ask"],
		"KoineksETHBid":         diffs[GDAX+"-Koineks-ETH-Bid"],
		"KoinimETHAsk":          diffs[GDAX+"-Koinim-ETH-Ask"],
		"KoinimETHBid":          diffs[GDAX+"-Koinim-ETH-Bid"],
		"VebitcoinETHAsk":       diffs[GDAX+"-Vebitcoin-ETH-Ask"],
		"VebitcoinETHBid":       diffs[GDAX+"-Vebitcoin-ETH-Bid"],
		"BitfinexETHAsk":        diffs[GDAX+"-Bitfinex-ETH-Ask"],
		"BitfinexETHBid":        diffs[GDAX+"-Bitfinex-ETH-Bid"],
		"GdaxLTC":               coinbaseProPrices["LTC"].Ask,
		"ParibuLTCAsk":          diffs[GDAX+"-Paribu-LTC-Ask"],
		"ParibuLTCBid":          diffs[GDAX+"-Paribu-LTC-Bid"],
		"BTCTurkLTCAsk":         diffs[GDAX+"-BTCTurk-LTC-Ask"],
		"BTCTurkLTCBid":         diffs[GDAX+"-BTCTurk-LTC-Bid"],
		"KoineksLTCAsk":         diffs[GDAX+"-Koineks-LTC-Ask"],
		"KoineksLTCBid":         diffs[GDAX+"-Koineks-LTC-Bid"],
		"KoinimLTCAsk":          diffs[GDAX+"-Koinim-LTC-Ask"],
		"KoinimLTCBid":          diffs[GDAX+"-Koinim-LTC-Bid"],
		"VebitcoinLTCAsk":       diffs[GDAX+"-Vebitcoin-LTC-Ask"],
		"VebitcoinLTCBid":       diffs[GDAX+"-Vebitcoin-LTC-Bid"],
		"BitfinexLTCAsk":        diffs[GDAX+"-Bitfinex-LTC-Ask"],
		"BitfinexLTCBid":        diffs[GDAX+"-Bitfinex-LTC-Bid"],
		"GdaxBCH":               coinbaseProPrices["BCH"].Ask,
		"BCHSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"BCH"]),
		"ParibuBCHAsk":          diffs[GDAX+"-Paribu-BCH-Ask"],
		"ParibuBCHBid":          diffs[GDAX+"-Paribu-BCH-Bid"],
		"KoineksBCHAsk":         diffs[GDAX+"-Koineks-BCH-Ask"],
		"KoineksBCHBid":         diffs[GDAX+"-Koineks-BCH-Bid"],
		"KoinimBCHAsk":          diffs[GDAX+"-Koinim-BCH-Ask"],
		"KoinimBCHBid":          diffs[GDAX+"-Koinim-BCH-Bid"],
		"GdaxETC":               coinbaseProPrices["ETC"].Ask,
		"ETCSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"ETC"]),
		"KoineksETCAsk":         diffs[GDAX+"-Koineks-ETC-Ask"],
		"KoineksETCBid":         diffs[GDAX+"-Koineks-ETC-Bid"],
		"GdaxXLM":               coinbaseProPrices["XLM"].Ask,
		"XLMSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"XLM"]),
		"ParibuXLMAsk":          diffs[GDAX+"-Paribu-XLM-Ask"],
		"ParibuXLMBid":          diffs[GDAX+"-Paribu-XLM-Bid"],
		"BTCTurkXLMAsk":         diffs[GDAX+"-BTCTurk-XLM-Ask"],
		"BTCTurkXLMBid":         diffs[GDAX+"-BTCTurk-XLM-Bid"],
		"KoineksXLMAsk":         diffs[GDAX+"-Koineks-XLM-Ask"],
		"KoineksXLMBid":         diffs[GDAX+"-Koineks-XLM-Bid"],
		"VebitcoinXLMAsk":       diffs[GDAX+"-Vebitcoin-XLM-Ask"],
		"VebitcoinXLMBid":       diffs[GDAX+"-Vebitcoin-XLM-Bid"],
		"BitfinexXLMAsk":        diffs[GDAX+"-Bitfinex-XLM-Ask"],
		"BitfinexXLMBid":		 		 diffs[GDAX+"-Bitfinex-XLM-Bid"],
		"GdaxEOS":               coinbaseProPrices["EOS"].Ask,
		"EOSSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"EOS"]),
		"ParibuEOSAsk":          diffs[GDAX+"-Paribu-EOS-Ask"],
		"ParibuEOSBid":          diffs[GDAX+"-Paribu-EOS-Bid"],
		"KoineksEOSAsk":         diffs[GDAX+"-Koineks-EOS-Ask"],
		"KoineksEOSBid":         diffs[GDAX+"-Koineks-EOS-Bid"],
		"VebitcoinEOSAsk":       diffs[GDAX+"-Vebitcoin-EOS-Ask"],
		"VebitcoinEOSBid":       diffs[GDAX+"-Vebitcoin-EOS-Bid"],
		"GdaxLINK":               coinbaseProPrices["LINK"].Ask,
		"LINKSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"LINK"]),
		"ParibuLINKAsk":          diffs[GDAX+"-Paribu-LINK-Ask"],
		"ParibuLINKBid":          diffs[GDAX+"-Paribu-LINK-Bid"],
		"VebitcoinLINKAsk":       diffs[GDAX+"-Vebitcoin-LINK-Ask"],
		"VebitcoinLINKBid":       diffs[GDAX+"-Vebitcoin-LINK-Bid"],
		"BTCTurkLINKAsk":         diffs[GDAX+"-BTCTurk-LINK-Ask"],
		"BTCTurkLINKBid":         diffs[GDAX+"-BTCTurk-LINK-Bid"],
		"KoineksLINKAsk":         diffs[GDAX+"-Koineks-LINK-Ask"],
		"KoineksLINKBid":         diffs[GDAX+"-Koineks-LINK-Bid"],
		"GdaxDASH":              coinbaseProPrices["DASH"].Ask,
		"DASHSpread":            fmt.Sprintf("%.2f", spreads[GDAX+"DASH"]),
		"KoineksDASHAsk":        diffs[GDAX+"-Koineks-DASH-Ask"],
		"KoineksDASHBid":        diffs[GDAX+"-Koineks-DASH-Bid"],
		"KoinimDASHAsk":      	 diffs[GDAX+"-Koinim-DASH-Ask"],
		"KoinimDASHBid":      	 diffs[GDAX+"-Koinim-DASH-Bid"],
		"VebitcoinDASHAsk":      diffs[GDAX+"-Vebitcoin-DASH-Ask"],
		"VebitcoinDASHBid":      diffs[GDAX+"-Vebitcoin-DASH-Bid"],
		"GdaxZEC":              coinbaseProPrices["ZEC"].Ask,
		"ZECSpread":            fmt.Sprintf("%.2f", spreads[GDAX+"ZEC"]),
		"KoineksZECAsk":        diffs[GDAX+"-Koineks-ZEC-Ask"],
		"KoineksZECBid":        diffs[GDAX+"-Koineks-ZEC-Bid"],
		"VebitcoinZECAsk":      diffs[GDAX+"-Vebitcoin-ZEC-Ask"],
		"VebitcoinZECBid":      diffs[GDAX+"-Vebitcoin-ZEC-Bid"],
		"GdaxMKR":              coinbaseProPrices["MKR"].Ask,
		"MKRSpread":            fmt.Sprintf("%.2f", spreads[GDAX+"MKR"]),
		"KoineksMKRAsk":        diffs[GDAX+"-Koineks-MKR-Ask"],
		"KoineksMKRBid":        diffs[GDAX+"-Koineks-MKR-Bid"],
		"ParibuMKRAsk":          diffs[GDAX+"-Paribu-MKR-Ask"],
		"ParibuMKRBid":          diffs[GDAX+"-Paribu-MKR-Bid"],
		"GdaxADA":               coinbaseProPrices["ADA"].Ask,
		"ADASpread":             fmt.Sprintf("%.2f", spreads[GDAX+"ADA"]),
		"KoineksADAAsk":         diffs[GDAX+"-Koineks-ADA-Ask"],
		"KoineksADABid":         diffs[GDAX+"-Koineks-ADA-Bid"],
		"ParibuADAAsk":          diffs[GDAX+"-Paribu-ADA-Ask"],
		"ParibuADABid":          diffs[GDAX+"-Paribu-ADA-Bid"],
		"BTCTurkADAAsk":         diffs[GDAX+"-BTCTurk-ADA-Ask"],
		"BTCTurkADABid":         diffs[GDAX+"-BTCTurk-ADA-Bid"],
		"GdaxBAT":               coinbaseProPrices["BAT"].Ask,
		"BATSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"BAT"]),
		"KoineksBATAsk":         diffs[GDAX+"-Koineks-BAT-Ask"],
		"KoineksBATBid":         diffs[GDAX+"-Koineks-BAT-Bid"],
		"ParibuBTCAskPrice":     prices["Paribu-BTC-Ask"],
		"ParibuBTCBidPrice":     prices["Paribu-BTC-Bid"],
		"BTCTurkBTCAskPrice":    prices["BTCTurk-BTC-Ask"],
		"BTCTurkBTCBidPrice":    prices["BTCTurk-BTC-Bid"],
		"KoineksBTCAskPrice":    prices["Koineks-BTC-Ask"],
		"KoineksBTCBidPrice":    prices["Koineks-BTC-Bid"],
		"KoinimBTCAskPrice":     prices["Koinim-BTC-Ask"],
		"KoinimBTCBidPrice":     prices["Koinim-BTC-Bid"],
		"VebitcoinBTCAskPrice":  prices["Vebitcoin-BTC-Ask"],
		"VebitcoinBTCBidPrice":  prices["Vebitcoin-BTC-Bid"],
		"BitfinexBTCAskPrice":   prices["Bitfinex-BTC-Ask"],
		"BitfinexBTCBidPrice":   prices["Bitfinex-BTC-Bid"],
		"ParibuETHAskPrice":     prices["Paribu-ETH-Ask"],
		"ParibuETHBidPrice":     prices["Paribu-ETH-Bid"],
		"BTCTurkETHAskPrice":    prices["BTCTurk-ETH-Ask"],
		"BTCTurkETHBidPrice":    prices["BTCTurk-ETH-Bid"],
		"KoineksETHAskPrice":    prices["Koineks-ETH-Ask"],
		"KoineksETHBidPrice":    prices["Koineks-ETH-Bid"],
		"KoinimETHAskPrice":     prices["Koinim-ETH-Ask"],
		"KoinimETHBidPrice":     prices["Koinim-ETH-Bid"],
		"VebitcoinETHAskPrice":  prices["Vebitcoin-ETH-Ask"],
		"VebitcoinETHBidPrice":  prices["Vebitcoin-ETH-Bid"],
		"BitfinexETHAskPrice":   prices["Bitfinex-ETH-Ask"],
		"BitfinexETHBidPrice":   prices["Bitfinex-ETH-Bid"],
		"ParibuLTCAskPrice":     prices["Paribu-LTC-Ask"],
		"ParibuLTCBidPrice":     prices["Paribu-LTC-Bid"],
		"BTCTurkLTCAskPrice":    prices["BTCTurk-LTC-Ask"],
		"BTCTurkLTCBidPrice":    prices["BTCTurk-LTC-Bid"],
		"KoineksLTCAskPrice":    prices["Koineks-LTC-Ask"],
		"KoineksLTCBidPrice":    prices["Koineks-LTC-Bid"],
		"KoinimLTCAskPrice":     prices["Koinim-LTC-Ask"],
		"KoinimLTCBidPrice":     prices["Koinim-LTC-Bid"],
		"VebitcoinLTCAskPrice":  prices["Vebitcoin-LTC-Ask"],
		"VebitcoinLTCBidPrice":  prices["Vebitcoin-LTC-Bid"],
		"BitfinexLTCAskPrice":   prices["Bitfinex-LTC-Ask"],
		"BitfinexLTCBidPrice":   prices["Bitfinex-LTC-Bid"],
		"ParibuBCHAskPrice":     prices["Paribu-BCH-Ask"],
		"ParibuBCHBidPrice":     prices["Paribu-BCH-Bid"],
		"KoineksBCHAskPrice":    prices["Koineks-BCH-Ask"],
		"KoineksBCHBidPrice":    prices["Koineks-BCH-Bid"],
		"KoinimBCHAskPrice":     prices["Koinim-BCH-Ask"],
		"KoinimBCHBidPrice":     prices["Koinim-BCH-Bid"],
		"KoineksETCAskPrice":    prices["Koineks-ETC-Ask"],
		"KoineksETCBidPrice":    prices["Koineks-ETC-Bid"],
		"ParibuUSDTAskPrice":     prices["Paribu-USDT-Ask"],
		"ParibuUSDTBidPrice":     prices["Paribu-USDT-Bid"],
		"BTCTurkUSDTAskPrice":   prices["BTCTurk-USDT-Ask"],
		"BTCTurkUSDTBidPrice":   prices["BTCTurk-USDT-Bid"],
		"KoineksUSDTAskPrice":   prices["Koineks-USDT-Ask"],
		"KoineksUSDTBidPrice":   prices["Koineks-USDT-Bid"],
		"VebitcoinUSDTAskPrice":   prices["Vebitcoin-USDT-Ask"],
		"VebitcoinUSDTBidPrice":   prices["Vebitcoin-USDT-Bid"],
		"ParibuDOGEAskPrice":     prices["Paribu-DOGE-Ask"],
		"ParibuDOGEBidPrice":     prices["Paribu-DOGE-Bid"],
		"KoineksDOGEAskPrice":   prices["Koineks-DOGE-Ask"],
		"KoineksDOGEBidPrice":   prices["Koineks-DOGE-Bid"],
		"KoinimDOGEAskPrice":    prices["Koinim-DOGE-Ask"],
		"KoinimDOGEBidPrice":    prices["Koinim-DOGE-Bid"],
		"ParibuXLMAskPrice":     prices["Paribu-XLM-Ask"],
		"ParibuXLMBidPrice":     prices["Paribu-XLM-Bid"],
		"BTCTurkXLMAskPrice":    prices["BTCTurk-XLM-Ask"],
		"BTCTurkXLMBidPrice":    prices["BTCTurk-XLM-Bid"],
		"KoineksXLMAskPrice":    prices["Koineks-XLM-Ask"],
		"KoineksXLMBidPrice":    prices["Koineks-XLM-Bid"],
		"VebitcoinXLMAskPrice":  prices["Vebitcoin-XLM-Ask"],
		"VebitcoinXLMBidPrice":  prices["Vebitcoin-XLM-Bid"],
		"BitfinexXLMAskPrice":   prices["Bitfinex-XLM-Ask"],
		"BitfinexXLMBidPrice":   prices["Bitfinex-XLM-Bid"],
		"ParibuEOSAskPrice":     prices["Paribu-EOS-Ask"],
		"ParibuEOSBidPrice":     prices["Paribu-EOS-Bid"],
		"KoineksEOSAskPrice":    prices["Koineks-EOS-Ask"],
		"KoineksEOSBidPrice":    prices["Koineks-EOS-Bid"],
		"VebitcoinEOSAskPrice":    prices["Vebitcoin-EOS-Ask"],
		"VebitcoinEOSBidPrice":    prices["Vebitcoin-EOS-Bid"],
		"ParibuLINKAskPrice":     prices["Paribu-LINK-Ask"],
		"ParibuLINKBidPrice":     prices["Paribu-LINK-Bid"],
		"BTCTurkLINKAskPrice":    prices["BTCTurk-LINK-Ask"],
		"BTCTurkLINKBidPrice":    prices["BTCTurk-LINK-Bid"],
		"KoineksLINKAskPrice":    prices["Koineks-LINK-Ask"],
		"KoineksLINKBidPrice":    prices["Koineks-LINK-Bid"],
		"VebitcoinLINKAskPrice":  prices["Vebitcoin-LINK-Ask"],
		"VebitcoinLINKBidPrice":  prices["Vebitcoin-LINK-Bid"],
		"KoineksDASHAskPrice":    prices["Koineks-DASH-Ask"],
		"KoineksDASHBidPrice":    prices["Koineks-DASH-Bid"],
		"KoinimDASHAskPrice":    prices["Koinim-DASH-Ask"],
		"KoinimDASHBidPrice":    prices["Koinim-DASH-Bid"],
		"VebitcoinDASHAskPrice":  prices["Vebitcoin-DASH-Ask"],
		"VebitcoinDASHBidPrice":  prices["Vebitcoin-DASH-Bid"],
		"KoineksZECAskPrice":    prices["Koineks-ZEC-Ask"],
		"KoineksZECBidPrice":    prices["Koineks-ZEC-Bid"],
		"VebitcoinZECAskPrice":  prices["Vebitcoin-ZEC-Ask"],
		"VebitcoinZECBidPrice":  prices["Vebitcoin-ZEC-Bid"],
		"KoineksMKRAskPrice":    prices["Koineks-MKR-Ask"],
		"KoineksMKRBidPrice":    prices["Koineks-MKR-Bid"],
		"ParibuMKRAskPrice":     prices["Paribu-MKR-Ask"],
		"ParibuMKRBidPrice":     prices["Paribu-MKR-Bid"],
		"KoineksADAAskPrice":    prices["Koineks-ADA-Ask"],
		"KoineksADABidPrice":    prices["Koineks-ADA-Bid"],
		"ParibuADAAskPrice":     prices["Paribu-ADA-Ask"],
		"ParibuADABidPrice":     prices["Paribu-ADA-Bid"],
		"BTCTurkADAAskPrice":     prices["BTCTurk-ADA-Ask"],
		"BTCTurkADABidPrice":     prices["BTCTurk-ADA-Bid"],
		"KoineksBATAskPrice":    prices["Koineks-BAT-Ask"],
		"KoineksBATBidPrice":    prices["Koineks-BAT-Bid"],
		"GdaxUSDT":              fmt.Sprintf("%.8f", coinbaseProPrices["USDT"].Ask),
		"USDTSpread":            fmt.Sprintf("%.2f", spreads[GDAX+"USDT"]),
		"ParibuUSDTAsk":         diffs[GDAX+"-Paribu-USDT-Ask"],
		"ParibuUSDTBid":         diffs[GDAX+"-Paribu-USDT-Bid"],
		"BTCTurkUSDTAsk":        diffs[GDAX+"-BTCTurk-USDT-Ask"],
		"BTCTurkUSDTBid":        diffs[GDAX+"-BTCTurk-USDT-Bid"],
		"KoineksUSDTAsk":        diffs[GDAX+"-Koineks-USDT-Ask"],
		"KoineksUSDTBid":        diffs[GDAX+"-Koineks-USDT-Bid"],
		"VebitcoinUSDTAsk":      diffs[GDAX+"-Vebitcoin-USDT-Ask"],
		"VebitcoinUSDTBid":      diffs[GDAX+"-Vebitcoin-USDT-Bid"],
		"GdaxDOGE":              fmt.Sprintf("%.8f", coinbaseProPrices["DOGE"].Ask),
		"DOGEAsk":        		 	 fmt.Sprintf("%.8f", crossPrices["DOGE"].Ask),
		"DOGESpread":     		   fmt.Sprintf("%.2f", spreads[GDAX+"DOGE"]),
		"ParibuDOGEAsk":         diffs[GDAX+"-Paribu-DOGE-Ask"],
		"ParibuDOGEBid":         diffs[GDAX+"-Paribu-DOGE-Bid"],
		"KoineksDOGEAsk":        diffs[GDAX+"-Koineks-DOGE-Ask"],
		"KoineksDOGEBid":        diffs[GDAX+"-Koineks-DOGE-Bid"],
		"KoinimDOGEAsk":         diffs[GDAX+"-Koinim-DOGE-Ask"],
		"KoinimDOGEBid":         diffs[GDAX+"-Koinim-DOGE-Bid"],
		"Warning":               warning,
	})
	mux.Unlock()
}

func SetNotificationLimits(c *gin.Context) {
	minimumStr := c.Query("minimum")
	maximumStr := c.Query("maximum")
	durationStr := c.Query("duration")
	fiatEnable := c.Query("fiatEnable")
	pThresholdStr := c.Query("pThreshold")

	if minimumStr != "" {
		minimum, err := strconv.ParseFloat(minimumStr, 64)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		MIN_NOTI_PERC = minimum
	}

	if maximumStr != "" {
		maximum, err := strconv.ParseFloat(maximumStr, 64)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		MAX_NOTI_PERC = maximum
	}

	if durationStr != "" {
		duration, err := strconv.ParseFloat(durationStr, 64)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		DURATION = duration
	}

	if pThresholdStr != "" {
		pThreshold, err := strconv.ParseFloat(pThresholdStr, 64)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		PAIR_THRESHOLD = pThreshold
	}

	switch fiatEnable {
	case "true":
		fiatNotificationEnabled = true
	case "false":
		fiatNotificationEnabled = false
	default:
		fiatNotificationEnabled = false
	}

	c.HTML(http.StatusOK, "notification.tmpl", gin.H{
		"Minimum":    MIN_NOTI_PERC,
		"Maximum":    MAX_NOTI_PERC,
		"Duration":   DURATION,
		"PThreshold": PAIR_THRESHOLD,
	})
}

func findPriceDifferences(priceLists ...[]Price) {
	for _, symbol := range ALL_SYMBOLS {
		var tryList []Price
		var aedList []Price

		originP := coinbaseProPrices[symbol]
		tryP := Price{Currency: "TRY", Exchange: originP.Exchange, ID: originP.ID, Bid: originP.Bid * tryRate, Ask: originP.Ask * tryRate}
		aedP := Price{Currency: "AED", Exchange: originP.Exchange, ID: originP.ID, Bid: originP.Bid * aedRate, Ask: originP.Ask * aedRate}
		tryList = append(tryList, tryP)
		aedList = append(aedList, aedP)

		for _, list := range priceLists {
			for _, p := range list {
				if p.ID == symbol {
					switch p.Currency {
					case "TRY":
						tryList = append(tryList, p)
					case "AED":
						aedList = append(aedList, p)
					}
				}
			}
		}

		setDiffsAndPrices(tryList)
		setDiffsAndPrices(aedList)
	}
}

func setDiffsAndPrices(list []Price) {
	firstExchange := ""
	firstAsk := 0.0
	for i, p := range list {
		if i == 0 {
			firstAsk = p.Ask
			firstExchange = p.Exchange
		} else {
			askPercentage := (p.Ask - firstAsk) * 100 / firstAsk
			bidPercentage := (p.Bid - firstAsk) * 100 / firstAsk

			askRound := Round(askPercentage, .5, 2)
			bidRound := Round(bidPercentage, .5, 2)

			mux.Lock()
			diffs[fmt.Sprintf("%s-%s-%s-%s", firstExchange, p.Exchange, p.ID, "Ask")] = askRound
			diffs[fmt.Sprintf("%s-%s-%s-%s", firstExchange, p.Exchange, p.ID, "Bid")] = bidRound

			prices[fmt.Sprintf("%s-%s-%s", p.Exchange, p.ID, "Ask")] = p.Ask
			prices[fmt.Sprintf("%s-%s-%s", p.Exchange, p.ID, "Bid")] = p.Bid
			mux.Unlock()

			maxD := maxDiffs[p.Exchange]
			minD, ok := minDiffs[p.Exchange]
			if !ok {
				minD = 100
			}

			if askRound < minD {
				minDiffs[p.Exchange] = askRound
				minSymbol[p.Exchange] = p.ID
			}

			if bidRound > maxD {
				maxDiffs[p.Exchange] = bidRound
				maxSymbol[p.Exchange] = p.ID
			}
		}
	}
}

func Round(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}

func resetDiffsAndSymbols() {
	for key, _ := range minDiffs {
		minDiffs[key] = 100
	}

	for key, _ := range maxDiffs {
		maxDiffs[key] = -100
	}

	for key, _ := range minSymbol {
		minSymbol[key] = ""
	}

	for key, _ := range maxSymbol {
		maxSymbol[key] = ""
	}

	warning = ""
}
