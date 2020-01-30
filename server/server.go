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
	BASE_CURRENCY_URI = "https://www.alphavantage.co/query?function=CURRENCY_EXCHANGE_RATE&from_currency=USD&to_currency=%s&apikey=GOJHTH53I4S9GPIV"
)

var (
	diffs                                                                              map[string]float64
	prices, spreads                                                                    map[string]float64
	minDiffs, maxDiffs                                                                 map[string]float64
	dogeVolumes                                                                        map[string]float64
	minSymbol, maxSymbol                                                               map[string]string
	binancePrices                				 										 					 								 map[string]Price
	coinbaseProPrices               				 																					 map[string]*Price
	paribuPrices,btcTurkPrices, koineksPrices, koinimPrices, vebitcoinPrices, bitoasisPrices []Price
	btcTurkETHBTCAskBid, btcTurkETHBTCBidAsk                                           float64
	koineksETHBTCAskBid, koineksETHBTCBidAsk, koineksLTCBTCAskBid, koineksLTCBTCBidAsk float64
	koinimLTCBTCAskBid, koinimLTCBTCBidAsk                                             float64
	fiatNotificationEnabled                                                            = true
	warning                                                                            string

	ALL_SYMBOLS = []string{"BTC", "ETH", "LTC", "BCH", "ETC", "ZRX", "XRP", "XLM", "EOS", "USDT", "DOGE", "XEM", "LINK", "DASH"}
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
		findAltcoinPrices(binancePrices, paribuPrices, btcTurkPrices, koineksPrices, koinimPrices, vebitcoinPrices, bitoasisPrices)
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		koineksPrices, err = getKoineksPrices()
		if err != nil {
			message := fmt.Sprintf("Error reading Koineks prices : %s", err)
			warning += message + "\n"
			fmt.Println(message)
			log.Println(message)
		}
	}()

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

	wg.Add(1)
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
		bitoasisPrices, err = getBitoasisPrices()
		if err != nil {
			message := fmt.Sprintf("Error reading Bitoasis prices : %s", err)
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
	}()
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
		"BitoasisBTCAsk":        diffs[GDAX+"-Bitoasis-BTC-Ask"],
		"BitoasisBTCBid":        diffs[GDAX+"-Bitoasis-BTC-Bid"],
		"BitfinexBTCAsk":        diffs[GDAX+"-Bitfinex-BTC-Ask"],
		"BitfinexBTCBid":        diffs[GDAX+"-Bitfinex-BTC-Bid"],
		"CexioBTCAsk":           diffs[GDAX+"-Cexio-BTC-Ask"],
		"CexioBTCBid":           diffs[GDAX+"-Cexio-BTC-Bid"],
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
		"BitoasisETHAsk":        diffs[GDAX+"-Bitoasis-ETH-Ask"],
		"BitoasisETHBid":        diffs[GDAX+"-Bitoasis-ETH-Bid"],
		"BitfinexETHAsk":        diffs[GDAX+"-Bitfinex-ETH-Ask"],
		"BitfinexETHBid":        diffs[GDAX+"-Bitfinex-ETH-Bid"],
		"CexioETHAsk":           diffs[GDAX+"-Cexio-ETH-Ask"],
		"CexioETHBid":           diffs[GDAX+"-Cexio-ETH-Bid"],
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
		"BitoasisLTCAsk":        diffs[GDAX+"-Bitoasis-LTC-Ask"],
		"BitoasisLTCBid":        diffs[GDAX+"-Bitoasis-LTC-Bid"],
		"BitfinexLTCAsk":        diffs[GDAX+"-Bitfinex-LTC-Ask"],
		"BitfinexLTCBid":        diffs[GDAX+"-Bitfinex-LTC-Bid"],
		"CexioLTCAsk":           diffs[GDAX+"-Cexio-LTC-Ask"],
		"CexioLTCBid":           diffs[GDAX+"-Cexio-LTC-Bid"],
		"GdaxBCH":               coinbaseProPrices["BCH"].Ask,
		"BCHSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"BCH"]),
		"ParibuBCHAsk":          diffs[GDAX+"-Paribu-BCH-Ask"],
		"ParibuBCHBid":          diffs[GDAX+"-Paribu-BCH-Bid"],
		"KoineksBCHAsk":         diffs[GDAX+"-Koineks-BCH-Ask"],
		"KoineksBCHBid":         diffs[GDAX+"-Koineks-BCH-Bid"],
		"KoinimBCHAsk":          diffs[GDAX+"-Koinim-BCH-Ask"],
		"KoinimBCHBid":          diffs[GDAX+"-Koinim-BCH-Bid"],
		"VebitcoinBCHAsk":       diffs[GDAX+"-Vebitcoin-BCH-Ask"],
		"VebitcoinBCHBid":       diffs[GDAX+"-Vebitcoin-BCH-Bid"],
		"BitoasisBCHAsk":        diffs[GDAX+"-Bitoasis-BCH-Ask"],
		"BitoasisBCHBid":        diffs[GDAX+"-Bitoasis-BCH-Bid"],
		"CexioBCHAsk":           diffs[GDAX+"-Cexio-BCH-Ask"],
		"CexioBCHBid":           diffs[GDAX+"-Cexio-BCH-Bid"],
		"GdaxETC":               coinbaseProPrices["ETC"].Ask,
		"ETCSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"ETC"]),
		"KoineksETCAsk":         diffs[GDAX+"-Koineks-ETC-Ask"],
		"KoineksETCBid":         diffs[GDAX+"-Koineks-ETC-Bid"],
		"GdaxZRX":               coinbaseProPrices["ZRX"].Ask,
		"ZRXSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"ZRX"]),
		"VebitcoinZRXAsk":       diffs[GDAX+"-Vebitcoin-ZRX-Ask"],
		"VebitcoinZRXBid":       diffs[GDAX+"-Vebitcoin-ZRX-Bid"],
		"GdaxXRP":               coinbaseProPrices["XRP"].Ask,
		"XRPSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"XRP"]),
		"ParibuXRPAsk":          diffs[GDAX+"-Paribu-XRP-Ask"],
		"ParibuXRPBid":          diffs[GDAX+"-Paribu-XRP-Bid"],
		"BTCTurkXRPAsk":         diffs[GDAX+"-BTCTurk-XRP-Ask"],
		"BTCTurkXRPBid":         diffs[GDAX+"-BTCTurk-XRP-Bid"],
		"KoineksXRPAsk":         diffs[GDAX+"-Koineks-XRP-Ask"],
		"KoineksXRPBid":         diffs[GDAX+"-Koineks-XRP-Bid"],
		"VebitcoinXRPAsk":       diffs[GDAX+"-Vebitcoin-XRP-Ask"],
		"VebitcoinXRPBid":       diffs[GDAX+"-Vebitcoin-XRP-Bid"],
		"BitoasisXRPAsk":        diffs[GDAX+"-Bitoasis-XRP-Ask"],
		"BitoasisXRPBid":        diffs[GDAX+"-Bitoasis-XRP-Bid"],
		"BitfinexXRPAsk":        diffs[GDAX+"-Bitfinex-XRP-Ask"],
		"BitfinexXRPBid":        diffs[GDAX+"-Bitfinex-XRP-Bid"],
		"CexioXRPAsk":           diffs[GDAX+"-Cexio-XRP-Ask"],
		"CexioXRPBid":           diffs[GDAX+"-Cexio-XRP-Bid"],
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
		"BitoasisXLMAsk":        diffs[GDAX+"-Bitoasis-XLM-Ask"],
		"BitoasisXLMBid":        diffs[GDAX+"-Bitoasis-XLM-Bid"],
		"BitfinexXLMAsk":        diffs[GDAX+"-Bitfinex-XLM-Ask"],
		"BitfinexXLMBid":		 		 diffs[GDAX+"-Bitfinex-XLM-Bid"],
		"CexioXLMAsk":           diffs[GDAX+"-Cexio-XLM-Ask"],
		"CexioXLMBid":           diffs[GDAX+"-Cexio-XLM-Bid"],
		"GdaxEOS":               coinbaseProPrices["EOS"].Ask,
		"EOSSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"EOS"]),
		"ParibuEOSAsk":          diffs[GDAX+"-Paribu-EOS-Ask"],
		"ParibuEOSBid":          diffs[GDAX+"-Paribu-EOS-Bid"],
		"KoineksEOSAsk":         diffs[GDAX+"-Koineks-EOS-Ask"],
		"KoineksEOSBid":         diffs[GDAX+"-Koineks-EOS-Bid"],
		"GdaxLINK":               coinbaseProPrices["LINK"].Ask,
		"LINKSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"LINK"]),
		"ParibuLINKAsk":          diffs[GDAX+"-Paribu-LINK-Ask"],
		"ParibuLINKBid":          diffs[GDAX+"-Paribu-LINK-Bid"],
		"VebitcoinLINKAsk":       diffs[GDAX+"-Vebitcoin-LINK-Ask"],
		"VebitcoinLINKBid":       diffs[GDAX+"-Vebitcoin-LINK-Bid"],
		"BTCTurkLINKAsk":         diffs[GDAX+"-BTCTurk-LINK-Ask"],
		"BTCTurkLINKBid":         diffs[GDAX+"-BTCTurk-LINK-Bid"],
		"GdaxDASH":              coinbaseProPrices["DASH"].Ask,
		"DASHSpread":            fmt.Sprintf("%.2f", spreads[GDAX+"DASH"]),
		"KoineksDASHAsk":        diffs[GDAX+"-Koineks-DASH-Ask"],
		"KoineksDASHBid":        diffs[GDAX+"-Koineks-DASH-Bid"],
		"KoinimDASHAsk":      	 diffs[GDAX+"-Koinim-DASH-Ask"],
		"KoinimDASHBid":      	 diffs[GDAX+"-Koinim-DASH-Bid"],
		"VebitcoinDASHAsk":      diffs[GDAX+"-Vebitcoin-DASH-Ask"],
		"VebitcoinDASHBid":      diffs[GDAX+"-Vebitcoin-DASH-Bid"],
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
		"BitoasisBTCAskPrice":   prices["Bitoasis-BTC-Ask"],
		"BitoasisBTCBidPrice":   prices["Bitoasis-BTC-Bid"],
		"BitfinexBTCAskPrice":   prices["Bitfinex-BTC-Ask"],
		"BitfinexBTCBidPrice":   prices["Bitfinex-BTC-Bid"],
		"CexioBTCAskPrice":      prices["Cexio-BTC-Ask"],
		"CexioBTCBidPrice":      prices["Cexio-BTC-Bid"],
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
		"BitoasisETHAskPrice":   prices["Bitoasis-ETH-Ask"],
		"BitoasisETHBidPrice":   prices["Bitoasis-ETH-Bid"],
		"BitfinexETHAskPrice":   prices["Bitfinex-ETH-Ask"],
		"BitfinexETHBidPrice":   prices["Bitfinex-ETH-Bid"],
		"CexioETHAskPrice":      prices["Cexio-ETH-Ask"],
		"CexioETHBidPrice":      prices["Cexio-ETH-Bid"],
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
		"BitoasisLTCAskPrice":   prices["Bitoasis-LTC-Ask"],
		"BitoasisLTCBidPrice":   prices["Bitoasis-LTC-Bid"],
		"BitfinexLTCAskPrice":   prices["Bitfinex-LTC-Ask"],
		"BitfinexLTCBidPrice":   prices["Bitfinex-LTC-Bid"],
		"CexioLTCAskPrice":      prices["Cexio-LTC-Ask"],
		"CexioLTCBidPrice":      prices["Cexio-LTC-Bid"],
		"ParibuBCHAskPrice":     prices["Paribu-BCH-Ask"],
		"ParibuBCHBidPrice":     prices["Paribu-BCH-Bid"],
		"KoineksBCHAskPrice":    prices["Koineks-BCH-Ask"],
		"KoineksBCHBidPrice":    prices["Koineks-BCH-Bid"],
		"KoinimBCHAskPrice":     prices["Koinim-BCH-Ask"],
		"KoinimBCHBidPrice":     prices["Koinim-BCH-Bid"],
		"VebitcoinBCHAskPrice":  prices["Vebitcoin-BCH-Ask"],
		"VebitcoinBCHBidPrice":  prices["Vebitcoin-BCH-Bid"],
		"BitoasisBCHAskPrice":   prices["Bitoasis-BCH-Ask"],
		"BitoasisBCHBidPrice":   prices["Bitoasis-BCH-Bid"],
		"CexioBCHAskPrice":      prices["Cexio-BCH-Ask"],
		"CexioBCHBidPrice":      prices["Cexio-BCH-Bid"],
		"KoineksETCAskPrice":    prices["Koineks-ETC-Ask"],
		"KoineksETCBidPrice":    prices["Koineks-ETC-Bid"],
		"VebitcoinZRXAskPrice":  prices["Vebitcoin-ZRX-Ask"],
		"VebitcoinZRXBidPrice":  prices["Vebitcoin-ZRX-Bid"],
		"ParibuUSDTAskPrice":     prices["Paribu-USDT-Ask"],
		"ParibuUSDTBidPrice":     prices["Paribu-USDT-Bid"],
		"BTCTurkUSDTAskPrice":   prices["BTCTurk-USDT-Ask"],
		"BTCTurkUSDTBidPrice":   prices["BTCTurk-USDT-Bid"],
		"KoineksUSDTAskPrice":   prices["Koineks-USDT-Ask"],
		"KoineksUSDTBidPrice":   prices["Koineks-USDT-Bid"],
		"ParibuDOGEAskPrice":     prices["Paribu-DOGE-Ask"],
		"ParibuDOGEBidPrice":     prices["Paribu-DOGE-Bid"],
		"KoineksDOGEAskPrice":   prices["Koineks-DOGE-Ask"],
		"KoineksDOGEBidPrice":   prices["Koineks-DOGE-Bid"],
		"KoinimDOGEAskPrice":    prices["Koinim-DOGE-Ask"],
		"KoinimDOGEBidPrice":    prices["Koinim-DOGE-Bid"],
		"ParibuXRPAskPrice":     prices["Paribu-XRP-Ask"],
		"ParibuXRPBidPrice":     prices["Paribu-XRP-Bid"],
		"BTCTurkXRPAskPrice":    prices["BTCTurk-XRP-Ask"],
		"BTCTurkXRPBidPrice":    prices["BTCTurk-XRP-Bid"],
		"KoineksXRPAskPrice":    prices["Koineks-XRP-Ask"],
		"KoineksXRPBidPrice":    prices["Koineks-XRP-Bid"],
		"VebitcoinXRPAskPrice":  prices["Vebitcoin-XRP-Ask"],
		"VebitcoinXRPBidPrice":  prices["Vebitcoin-XRP-Bid"],
		"BitoasisXRPAskPrice":   prices["Bitoasis-XRP-Ask"],
		"BitoasisXRPBidPrice":   prices["Bitoasis-XRP-Bid"],
		"BitfinexXRPAskPrice":   prices["Bitfinex-XRP-Ask"],
		"BitfinexXRPBidPrice":   prices["Bitfinex-XRP-Bid"],
		"CexioXRPAskPrice":      prices["Cexio-XRP-Ask"],
		"CexioXRPBidPrice":      prices["Cexio-XRP-Bid"],
		"ParibuXLMAskPrice":     prices["Paribu-XLM-Ask"],
		"ParibuXLMBidPrice":     prices["Paribu-XLM-Bid"],
		"BTCTurkXLMAskPrice":    prices["BTCTurk-XLM-Ask"],
		"BTCTurkXLMBidPrice":    prices["BTCTurk-XLM-Bid"],
		"KoineksXLMAskPrice":    prices["Koineks-XLM-Ask"],
		"KoineksXLMBidPrice":    prices["Koineks-XLM-Bid"],
		"VebitcoinXLMAskPrice":  prices["Vebitcoin-XLM-Ask"],
		"VebitcoinXLMBidPrice":  prices["Vebitcoin-XLM-Bid"],
		"BitoasisXLMAskPrice":   prices["Bitoasis-XLM-Ask"],
		"BitoasisXLMBidPrice":   prices["Bitoasis-XLM-Bid"],
		"BitfinexXLMAskPrice":   prices["Bitfinex-XLM-Ask"],
		"BitfinexXLMBidPrice":   prices["Bitfinex-XLM-Bid"],
		"CexioXLMAskPrice":      prices["Cexio-XLM-Ask"],
		"CexioXLMBidPrice":      prices["Cexio-XLM-Bid"],
		"ParibuEOSAskPrice":     prices["Paribu-EOS-Ask"],
		"ParibuEOSBidPrice":     prices["Paribu-EOS-Bid"],
		"KoineksEOSAskPrice":    prices["Koineks-EOS-Ask"],
		"KoineksEOSBidPrice":    prices["Koineks-EOS-Bid"],
		"ParibuLINKAskPrice":     prices["Paribu-LINK-Ask"],
		"ParibuLINKBidPrice":     prices["Paribu-LINK-Bid"],
		"BTCTurkLINKAskPrice":    prices["BTCTurk-LINK-Ask"],
		"BTCTurkLINKBidPrice":    prices["BTCTurk-LINK-Bid"],
		"VebitcoinLINKAskPrice":  prices["Vebitcoin-LINK-Ask"],
		"VebitcoinLINKBidPrice":  prices["Vebitcoin-LINK-Bid"],
		"KoineksDASHAskPrice":    prices["Koineks-DASH-Ask"],
		"KoineksDASHBidPrice":    prices["Koineks-DASH-Bid"],
		"KoinimDASHAskPrice":    prices["Koinim-DASH-Ask"],
		"KoinimDASHBidPrice":    prices["Koinim-DASH-Bid"],
		"VebitcoinDASHAskPrice":  prices["Vebitcoin-DASH-Ask"],
		"VebitcoinDASHBidPrice":  prices["Vebitcoin-DASH-Bid"],
		"KoineksXEMAskPrice":    prices["Koineks-XEM-Ask"],
		"KoineksXEMBidPrice":    prices["Koineks-XEM-Bid"],
		"GdaxUSDT":              fmt.Sprintf("%.8f", coinbaseProPrices["USDT"].Ask),
		"USDTSpread":            fmt.Sprintf("%.2f", spreads[BINANCE+"USDT"]),
		"ParibuUSDTAsk":         diffs[BINANCE+"-Paribu-USDT-Ask"],
		"ParibuUSDTBid":         diffs[BINANCE+"-Paribu-USDT-Bid"],
		"BTCTurkUSDTAsk":        diffs[BINANCE+"-BTCTurk-USDT-Ask"],
		"BTCTurkUSDTBid":        diffs[BINANCE+"-BTCTurk-USDT-Bid"],
		"KoineksUSDTAsk":        diffs[BINANCE+"-Koineks-USDT-Ask"],
		"KoineksUSDTBid":        diffs[BINANCE+"-Koineks-USDT-Bid"],
		"GdaxDOGE":              fmt.Sprintf("%.8f", coinbaseProPrices["DOGE"].Ask),
		"DOGEAsk":        		 	 fmt.Sprintf("%.8f", crossPrices["DOGE"].Ask),
		"DOGESpread":     		   fmt.Sprintf("%.2f", spreads[BINANCE+"DOGE"]),
		"ParibuDOGEAsk":         diffs[exchange+"-Paribu-DOGE-Ask"],
		"ParibuDOGEBid":         diffs[exchange+"-Paribu-DOGE-Bid"],
		"KoineksDOGEAsk":        diffs[exchange+"-Koineks-DOGE-Ask"],
		"KoineksDOGEBid":        diffs[exchange+"-Koineks-DOGE-Bid"],
		"KoinimDOGEAsk":         diffs[exchange+"-Koinim-DOGE-Ask"],
		"KoinimDOGEBid":         diffs[exchange+"-Koinim-DOGE-Bid"],
		"GdaxXEM":               fmt.Sprintf("%.5f", coinbaseProPrices["XEM"].Ask),
		"XEMAsk":         			 fmt.Sprintf("%.8f", crossPrices["XEM"].Ask),
		"XEMSpread":             fmt.Sprintf("%.2f", spreads[BINANCE+"XEM"]),
		"KoineksXEMAsk":         diffs[BINANCE+"-Koineks-XEM-Ask"],
		"KoineksXEMBid":         diffs[BINANCE+"-Koineks-XEM-Bid"],
		"BittrexDOGEAskPrice":   fmt.Sprintf("%.8f", prices["BittrexDOGEAsk"]),
		"BittrexDOGEBidPrice":   fmt.Sprintf("%.8f", prices["BittrexDOGEBid"]),
		"BittrexDOGEAskVolume":  fmt.Sprintf("%.2f", dogeVolumes["BittrexAsk"]),
		"BittrexDOGEBidVolume":  fmt.Sprintf("%.2f", dogeVolumes["BittrexBid"]),
		"BinanceDOGEAskPrice":   fmt.Sprintf("%.8f", prices["BinanceDOGEAsk"]),
		"BinanceDOGEBidPrice":   fmt.Sprintf("%.8f", prices["BinanceDOGEBid"]),
		"BinanceDOGEAskVolume":  fmt.Sprintf("%.2f", dogeVolumes["BinanceAsk"]),
		"BinanceDOGEBidVolume":  fmt.Sprintf("%.2f", dogeVolumes["BinanceBid"]),
		"Warning":               warning,
	})
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

			diffs[fmt.Sprintf("%s-%s-%s-%s", firstExchange, p.Exchange, p.ID, "Ask")] = askRound
			diffs[fmt.Sprintf("%s-%s-%s-%s", firstExchange, p.Exchange, p.ID, "Bid")] = bidRound
			prices[fmt.Sprintf("%s-%s-%s", p.Exchange, p.ID, "Ask")] = p.Ask
			prices[fmt.Sprintf("%s-%s-%s", p.Exchange, p.ID, "Bid")] = p.Bid

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
