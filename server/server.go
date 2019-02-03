package server

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/buger/jsonparser"
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
	BASE_CURRENCY_URI = "http://free.currencyconverterapi.com/api/v3/convert?q=USD_%s&compact=ultra"
)

var (
	tryRate = 0.0
	aedRate = 0.0

	diffs                                                                              map[string]float64
	prices, spreads                                                                    map[string]float64
	minDiffs, maxDiffs                                                                 map[string]float64
	dogeVolumes                                                                        map[string]float64
	minSymbol, maxSymbol                                                               map[string]string
	usdPrices, poloniexPrices, bittrexPrices, binancePrices            			   map[string]Price
	btcTurkETHBTCAskBid, btcTurkETHBTCBidAsk                                           float64
	koineksETHBTCAskBid, koineksETHBTCBidAsk, koineksLTCBTCAskBid, koineksLTCBTCBidAsk float64
	koinimLTCBTCAskBid, koinimLTCBTCBidAsk                                             float64
	fiatNotificationEnabled							                                   = true
	warning                                                                            string

	ALL_SYMBOLS = []string{"BTC", "ETH", "LTC", "BCH", "ETC", "ZRX", "XRP", "XLM", "USDT", "DOGE", "XEM"}
)

func Run() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*")

	router.GET("/", PrintTableWithBittrex)
	router.GET("/poloniex", PrintTableWithPoloniex)
	router.GET("/binance", PrintTableWithBinance)
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
		getPrices()
	}()

	wg.Wait()
}

func getCurrencies() {
	for {
		getCurrencyRates()
		time.Sleep(1 * time.Hour)
	}
}

func getPrices() {
	for {
		calculatePrices()
		time.Sleep(5 * time.Second)
	}
}

func calculatePrices() {
	var err error
	gdaxPrices, err := getGdaxPrices()
	if err != nil || len(gdaxPrices) != len(gdaxCurrencies) {
		message := fmt.Sprintf("Error reading GDAX prices : %s", err)
		warning += message + "\n"
		fmt.Println(message)
		log.Println(message)
		return
	}

	poloniexPrices, err = getPoloniexPrices()
	if err != nil || len(poloniexPrices) != len(poloniexCurrencies) {
		message := fmt.Sprintf("Error reading Poloniex prices : %s", err)
		warning += message + "\n"
		fmt.Println(message)
		log.Println(message)
	}

	bittrexPrices, err = getBittrexPrices()
	if err != nil || len(bittrexPrices) != len(bittrexCurrencies) {
		message := fmt.Sprintf("Error reading Bittrex prices : %s", err)
		warning += message + "\n"
		fmt.Println(message)
		log.Println(message)
	}

	binancePrices, err = getBinancePrices()
	if err != nil || len(binancePrices) != len(binanceCurrencies) {
		message := fmt.Sprintf("Error reading Binance prices : %s", err)
		warning += message + "\n"
		fmt.Println(message)
		log.Println(message)
	}

	paribuPrices, err := getParibuPrices()
	if err != nil {
		message := fmt.Sprintf("Error reading Paribu prices : %s", err)
		warning += message + "\n"
		fmt.Println(message)
		log.Println(message)
	}

	btcTurkPrices, err := getBTCTurkPrices()
	if err != nil {
		message := fmt.Sprintf("Error reading BTCTurk prices : %s", err)
		warning += message + "\n"
		fmt.Println(message)
		log.Println(message)
	}

	koineksPrices, err := getKoineksPrices()
	if err != nil {
		message := fmt.Sprintf("Error reading Koineks prices : %s", err)
		warning += message + "\n"
		fmt.Println(message)
		log.Println(message)
	}

	koinimPrices, err := getKoinimPrices()
	if err != nil {
		message := fmt.Sprintf("Error reading Koinim prices : %s", err)
		warning += message + "\n"
		fmt.Println(message)
		log.Println(message)
	}

	vebitcoinPrices, err := getVebitcoinPrices()
	if err != nil {
		message := fmt.Sprintf("Error reading Vebitcoin prices : %s", err)
		warning += message + "\n"
		fmt.Println(message)
		log.Println(message)
	}

	bitoasisPrices, err := getBitoasisPrices()
	if err != nil {
		message := fmt.Sprintf("Error reading Bitoasis prices : %s", err)
		warning += message + "\n"
		fmt.Println(message)
		log.Println(message)
	}

	if err := getPoloniexDOGEVolumes(); err != nil {
		message := fmt.Sprintf("Error reading Poloniex DOGE volumes : %s", err)
		warning += message + "\n"
		fmt.Println(message)
		log.Println(message)
	}

	if err := getBittrexDOGEVolumes(); err != nil {
		message := fmt.Sprintf("Error reading Bittrex DOGE volumes : %s", err)
		warning += message + "\n"
		fmt.Println(message)
		log.Println(message)
	}

	findAltcoinPrices(gdaxPrices, bittrexPrices, paribuPrices, btcTurkPrices, koineksPrices, koinimPrices, vebitcoinPrices, bitoasisPrices)
	findAltcoinPrices(gdaxPrices, poloniexPrices, paribuPrices, btcTurkPrices, koineksPrices, koinimPrices, vebitcoinPrices, bitoasisPrices)
	findAltcoinPrices(gdaxPrices, binancePrices, paribuPrices, btcTurkPrices, koineksPrices, koinimPrices, vebitcoinPrices, bitoasisPrices)

	sendMessages()

	resetDiffsAndSymbols()
}

func findAltcoinPrices(gdaxPrices []Price, exchangePrices map[string]Price, sellExchanges ...[]Price) {
	bitcoinPrice := gdaxPrices[0].Ask
	tempPrices := gdaxPrices
	for _, p := range exchangePrices {
		tempP := p
		if p.ID == "USDT" {
			if p.Exchange != BITTREX {
				tempP.Ask = bitcoinPrice / tempP.Ask
				tempP.Bid = bitcoinPrice / tempP.Bid
			}
		} else if p.Exchange == BINANCE && (p.ID == "XRP" || p.ID == "XLM") {
			tempPrices = append(tempPrices, tempP)
			continue
		} else {
			tempP.Ask *= bitcoinPrice
			tempP.Bid *= bitcoinPrice
		}
		tempPrices = append(tempPrices, tempP)
	}

	updateUSDPrices(tempPrices)
	var newPriceList [][]Price
	newPriceList = append(newPriceList, tempPrices)
	for _, list := range sellExchanges {
		newPriceList = append(newPriceList, list)
	}
	findPriceDifferences(newPriceList...)
}

func PrintTableWithPoloniex(c *gin.Context) {
	printTable(c, poloniexPrices, POLONIEX)
}

func PrintTableWithBittrex(c *gin.Context) {
	printTable(c, bittrexPrices, BITTREX)
}

func PrintTableWithBinance(c *gin.Context) {
	printTable(c, binancePrices, BINANCE)
}

func printTable(c *gin.Context, crossPrices map[string]Price, exchange string) {

	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"USDTRY":                tryRate,
		"USDAED":				 aedRate,
		"GdaxBTC":               usdPrices["GDAXBTC"].Ask,
		"ParibuBTCAsk":          diffs["GDAX-Paribu-BTC-Ask"],
		"ParibuBTCBid":          diffs["GDAX-Paribu-BTC-Bid"],
		"BTCTurkBTCAsk":         diffs["GDAX-BTCTurk-BTC-Ask"],
		"BTCTurkBTCBid":         diffs["GDAX-BTCTurk-BTC-Bid"],
		"KoineksBTCAsk":         diffs["GDAX-Koineks-BTC-Ask"],
		"KoineksBTCBid":         diffs["GDAX-Koineks-BTC-Bid"],
		"KoinimBTCAsk":          diffs["GDAX-Koinim-BTC-Ask"],
		"KoinimBTCBid":          diffs["GDAX-Koinim-BTC-Bid"],
		"VebitcoinBTCAsk":       diffs["GDAX-Vebitcoin-BTC-Ask"],
		"VebitcoinBTCBid":       diffs["GDAX-Vebitcoin-BTC-Bid"],
		"BitoasisBTCAsk":        diffs["GDAX-Bitoasis-BTC-Ask"],
		"BitoasisBTCBid":        diffs["GDAX-Bitoasis-BTC-Bid"],
		"GdaxETH":               usdPrices["GDAXETH"].Ask,
		"BTCTurkETHAsk":         diffs["GDAX-BTCTurk-ETH-Ask"],
		"BTCTurkETHBid":         diffs["GDAX-BTCTurk-ETH-Bid"],
		"KoineksETHAsk":         diffs["GDAX-Koineks-ETH-Ask"],
		"KoineksETHBid":         diffs["GDAX-Koineks-ETH-Bid"],
		"KoinimETHAsk":          diffs["GDAX-Koinim-ETH-Ask"],
		"KoinimETHBid":          diffs["GDAX-Koinim-ETH-Bid"],
		"VebitcoinETHAsk":       diffs["GDAX-Vebitcoin-ETH-Ask"],
		"VebitcoinETHBid":       diffs["GDAX-Vebitcoin-ETH-Bid"],
		"BitoasisETHAsk":        diffs["GDAX-Bitoasis-ETH-Ask"],
		"BitoasisETHBid":        diffs["GDAX-Bitoasis-ETH-Bid"],
		"GdaxLTC":               usdPrices["GDAXLTC"].Ask,
		"BTCTurkLTCAsk":         diffs["GDAX-BTCTurk-LTC-Ask"],
		"BTCTurkLTCBid":         diffs["GDAX-BTCTurk-LTC-Bid"],
		"KoineksLTCAsk":         diffs["GDAX-Koineks-LTC-Ask"],
		"KoineksLTCBid":         diffs["GDAX-Koineks-LTC-Bid"],
		"KoinimLTCAsk":          diffs["GDAX-Koinim-LTC-Ask"],
		"KoinimLTCBid":          diffs["GDAX-Koinim-LTC-Bid"],
		"VebitcoinLTCAsk":       diffs["GDAX-Vebitcoin-LTC-Ask"],
		"VebitcoinLTCBid":       diffs["GDAX-Vebitcoin-LTC-Bid"],
		"BitoasisLTCAsk":        diffs["GDAX-Bitoasis-LTC-Ask"],
		"BitoasisLTCBid":        diffs["GDAX-Bitoasis-LTC-Bid"],
		"GdaxBCH":               usdPrices["GDAXBCH"].Ask,
		"BCHSpread":      fmt.Sprintf("%.2f", spreads[GDAX+"BCH"]),
		"KoineksBCHAsk":         diffs["GDAX-Koineks-BCH-Ask"],
		"KoineksBCHBid":         diffs["GDAX-Koineks-BCH-Bid"],
		"KoinimBCHAsk":          diffs["GDAX-Koinim-BCH-Ask"],
		"KoinimBCHBid":          diffs["GDAX-Koinim-BCH-Bid"],
		"VebitcoinBCHAsk":       diffs["GDAX-Vebitcoin-BCH-Ask"],
		"VebitcoinBCHBid":       diffs["GDAX-Vebitcoin-BCH-Bid"],
		"BitoasisBCHAsk":        diffs["GDAX-Bitoasis-BCH-Ask"],
		"BitoasisBCHBid":        diffs["GDAX-Bitoasis-BCH-Bid"],
		"GdaxETC":               usdPrices["GDAXETC"].Ask,
		"ETCSpread":      fmt.Sprintf("%.2f", spreads[GDAX+"ETC"]),
		"KoineksETCAsk":         diffs["GDAX-Koineks-ETC-Ask"],
		"KoineksETCBid":         diffs["GDAX-Koineks-ETC-Bid"],
		"GdaxZRX":               usdPrices["GDAXZRX"].Ask,
		"ZRXSpread":      fmt.Sprintf("%.2f", spreads[GDAX+"ZRX"]),
		"VebitcoinZRXAsk":       diffs["GDAX-Vebitcoin-ZRX-Ask"],
		"VebitcoinZRXBid":       diffs["GDAX-Vebitcoin-ZRX-Bid"],
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
		"KoineksBCHAskPrice":    prices["Koineks-BCH-Ask"],
		"KoineksBCHBidPrice":    prices["Koineks-BCH-Bid"],
		"KoinimBCHAskPrice":     prices["Koinim-BCH-Ask"],
		"KoinimBCHBidPrice":     prices["Koinim-BCH-Bid"],
		"VebitcoinBCHAskPrice":  prices["Vebitcoin-BCH-Ask"],
		"VebitcoinBCHBidPrice":  prices["Vebitcoin-BCH-Bid"],
		"BitoasisBCHAskPrice":   prices["Bitoasis-BCH-Ask"],
		"BitoasisBCHBidPrice":   prices["Bitoasis-BCH-Bid"],
		"KoineksETCAskPrice":    prices["Koineks-ETC-Ask"],
		"KoineksETCBidPrice":    prices["Koineks-ETC-Bid"],
		"VebitcoinZRXAskPrice":  prices["Vebitcoin-ZRX-Ask"],
		"VebitcoinZRXBidPrice":  prices["Vebitcoin-ZRX-Bid"],
		"BTCTurkUSDTAskPrice":   prices["BTCTurk-USDT-Ask"],
		"BTCTurkUSDTBidPrice":   prices["BTCTurk-USDT-Bid"],
		"KoineksUSDTAskPrice":   prices["Koineks-USDT-Ask"],
		"KoineksUSDTBidPrice":   prices["Koineks-USDT-Bid"],
		"KoineksDOGEAskPrice":   prices["Koineks-DOGE-Ask"],
		"KoineksDOGEBidPrice":   prices["Koineks-DOGE-Bid"],
		"KoinimDOGEAskPrice":    prices["Koinim-DOGE-Ask"],
		"KoinimDOGEBidPrice":    prices["Koinim-DOGE-Bid"],
		"BTCTurkXRPAskPrice":    prices["BTCTurk-XRP-Ask"],
		"BTCTurkXRPBidPrice":    prices["BTCTurk-XRP-Bid"],
		"KoineksXRPAskPrice":    prices["Koineks-XRP-Ask"],
		"KoineksXRPBidPrice":    prices["Koineks-XRP-Bid"],
		"VebitcoinXRPAskPrice":  prices["Vebitcoin-XRP-Ask"],
		"VebitcoinXRPBidPrice":  prices["Vebitcoin-XRP-Bid"],
		"BitoasisXRPAskPrice":   prices["Bitoasis-XRP-Ask"],
		"BitoasisXRPBidPrice":   prices["Bitoasis-XRP-Bid"],
		"BTCTurkXLMAskPrice":    prices["BTCTurk-XLM-Ask"],
		"BTCTurkXLMBidPrice":    prices["BTCTurk-XLM-Bid"],
		"KoineksXLMAskPrice":    prices["Koineks-XLM-Ask"],
		"KoineksXLMBidPrice":    prices["Koineks-XLM-Bid"],
		"VebitcoinXLMAskPrice":  prices["Vebitcoin-XLM-Ask"],
		"VebitcoinXLMBidPrice":  prices["Vebitcoin-XLM-Bid"],
		"BitoasisXLMAskPrice":   prices["Bitoasis-XLM-Ask"],
		"BitoasisXLMBidPrice":   prices["Bitoasis-XLM-Bid"],
		"KoineksXEMAskPrice":    prices["Koineks-XEM-Ask"],
		"KoineksXEMBidPrice":    prices["Koineks-XEM-Bid"],
		"GdaxXRP":               fmt.Sprintf("%.5f", usdPrices[BINANCE+"XRP"].Ask),
		"XRPSpread":      fmt.Sprintf("%.2f", spreads[BINANCE+"XRP"]),
		"BTCTurkXRPAsk":         diffs[BINANCE+"-BTCTurk-XRP-Ask"],
		"BTCTurkXRPBid":         diffs[BINANCE+"-BTCTurk-XRP-Bid"],
		"KoineksXRPAsk":         diffs[BINANCE+"-Koineks-XRP-Ask"],
		"KoineksXRPBid":         diffs[BINANCE+"-Koineks-XRP-Bid"],
		"VebitcoinXRPAsk":       diffs[BINANCE+"-Vebitcoin-XRP-Ask"],
		"VebitcoinXRPBid":       diffs[BINANCE+"-Vebitcoin-XRP-Bid"],
		"BitoasisXRPAsk":        diffs[BINANCE+"-Bitoasis-XRP-Ask"],
		"BitoasisXRPBid":        diffs[BINANCE+"-Bitoasis-XRP-Bid"],
		"GdaxXLM":               fmt.Sprintf("%.5f", usdPrices[BINANCE+"XLM"].Ask),
		"XLMSpread":      fmt.Sprintf("%.2f", spreads[BINANCE+"XLM"]),
		"BTCTurkXLMAsk":         diffs[BINANCE+"-BTCTurk-XLM-Ask"],
		"BTCTurkXLMBid":         diffs[BINANCE+"-BTCTurk-XLM-Bid"],
		"KoineksXLMAsk":         diffs[BINANCE+"-Koineks-XLM-Ask"],
		"KoineksXLMBid":         diffs[BINANCE+"-Koineks-XLM-Bid"],
		"VebitcoinXLMAsk":       diffs[BINANCE+"-Vebitcoin-XLM-Ask"],
		"VebitcoinXLMBid":       diffs[BINANCE+"-Vebitcoin-XLM-Bid"],
		"BitoasisXLMAsk":        diffs[BINANCE+"-Bitoasis-XLM-Ask"],
		"BitoasisXLMBid":        diffs[BINANCE+"-Bitoasis-XLM-Bid"],
		"BittrexUSDT":           fmt.Sprintf("%.8f", usdPrices[exchange+"USDT"].Ask),
		"BittrexUSDTSpread":     fmt.Sprintf("%.2f", spreads[exchange+"USDT"]),
		"BTCTurkUSDTAsk":        diffs[exchange+"-BTCTurk-USDT-Ask"],
		"BTCTurkUSDTBid":        diffs[exchange+"-BTCTurk-USDT-Bid"],
		"KoineksUSDTAsk":        diffs[exchange+"-Koineks-USDT-Ask"],
		"KoineksUSDTBid":        diffs[exchange+"-Koineks-USDT-Bid"],
		"GdaxDOGE":              fmt.Sprintf("%.8f", usdPrices[exchange+"DOGE"].Ask),
		"BittrexDOGEAsk":        fmt.Sprintf("%.8f", crossPrices["DOGE"].Ask),
		"BittrexDOGESpread":     fmt.Sprintf("%.2f", spreads[exchange+"DOGE"]),
		"KoineksDOGEAsk":        diffs[exchange+"-Koineks-DOGE-Ask"],
		"KoineksDOGEBid":        diffs[exchange+"-Koineks-DOGE-Bid"],
		"KoinimDOGEAsk":         diffs[exchange+"-Koinim-DOGE-Ask"],
		"KoinimDOGEBid":         diffs[exchange+"-Koinim-DOGE-Bid"],
		"GdaxXEM":               fmt.Sprintf("%.5f", usdPrices[exchange+"XEM"].Ask),
		"BittrexXEMAsk":         fmt.Sprintf("%.8f", crossPrices["XEM"].Ask),
		"BittrexXEMSpread":      fmt.Sprintf("%.2f", spreads[exchange+"XEM"]),
		"KoineksXEMAsk":         diffs[exchange+"-Koineks-XEM-Ask"],
		"KoineksXEMBid":         diffs[exchange+"-Koineks-XEM-Bid"],
		"PoloniexDOGEAskPrice":  fmt.Sprintf("%.8f", prices["PoloniexDOGEAsk"]),
		"PoloniexDOGEBidPrice":  fmt.Sprintf("%.8f", prices["PoloniexDOGEBid"]),
		"PoloniexDOGEAskVolume": fmt.Sprintf("%.2f", dogeVolumes["PoloniexAsk"]),
		"PoloniexDOGEBidVolume": fmt.Sprintf("%.2f", dogeVolumes["PoloniexBid"]),
		"BittrexDOGEAskPrice":   fmt.Sprintf("%.8f", prices["BittrexDOGEAsk"]),
		"BittrexDOGEBidPrice":   fmt.Sprintf("%.8f", prices["BittrexDOGEBid"]),
		"BittrexDOGEAskVolume":  fmt.Sprintf("%.2f", dogeVolumes["BittrexAsk"]),
		"BittrexDOGEBidVolume":  fmt.Sprintf("%.2f", dogeVolumes["BittrexBid"]),
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

func getCurrencyRates() {
	response, err := http.Get(fmt.Sprintf(BASE_CURRENCY_URI, "TRY"))
	if err != nil {
		fmt.Println("failed to get response for currencies : ", err)
		log.Println("failed to get response for currencies : ", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Println("failed to read currency response data : ", err)
		log.Println("failed to read currency response data : ", err)
	}

	tryRate, err = jsonparser.GetFloat(responseData, "USD_TRY")
	if err != nil {
		fmt.Println("failed to read the TRY currency price from the response data: ", err)
		log.Println("failed to read the TRY currency price from the response data: ", err)
	}

	response, err = http.Get(fmt.Sprintf(BASE_CURRENCY_URI, "AED"))
	if err != nil {
		fmt.Println("failed to get response for currencies : ", err)
		log.Println("failed to get response for currencies : ", err)
	}

	responseData, err = ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Println("failed to read currency response data : ", err)
		log.Println("failed to read currency response data : ", err)
	}

	aedRate, err = jsonparser.GetFloat(responseData, "USD_AED")
	if err != nil {
		fmt.Println("failed to read the AED currency price from the response data: ", err)
		log.Println("failed to read the AED currency price from the response data: ", err)
	}
}

func findPriceDifferences(priceLists ...[]Price) {
	for _, symbol := range ALL_SYMBOLS {
		var tryList []Price
		var aedList []Price
		for _, list := range priceLists {
			for _, p := range list {
				if p.ID == symbol {
					switch p.Currency {
					case "USD":
						tryP := Price{Currency: "TRY", Exchange: p.Exchange, ID: p.ID, Bid: p.Bid * tryRate, Ask: p.Ask * tryRate}
						aedP := Price{Currency: "AED", Exchange: p.Exchange, ID: p.ID, Bid: p.Bid * aedRate, Ask: p.Ask * aedRate}
						tryList = append(tryList, tryP)
						aedList = append(aedList, aedP)
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

func updateUSDPrices(priceLists ...[]Price) {
	for _, list := range priceLists {
		for _, p := range list {
			existingPrice, ok := usdPrices[p.Exchange+p.Currency]
			if ok {
				existingPrice.Ask = p.Ask
				existingPrice.Bid = p.Bid
			} else {
				usdPrices[p.Exchange+p.ID] = Price{Currency: "USD", Exchange: p.Exchange, ID: p.ID, Ask: p.Ask, Bid: p.Bid}
			}
		}
	}
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
