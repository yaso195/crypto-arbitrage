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
	coinbaseProPrices               				 																					 map[string]*Price
	binancePrices, paribuPrices,btcTurkPrices, koinimPrices 					 								 []Price
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

	router.GET("/", PrintTable)
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
		findAltcoinPrices(binancePrices, paribuPrices, btcTurkPrices, koinimPrices)
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
		if err != nil {
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

func findAltcoinPrices(sellExchanges ...[]Price) {
	var newPriceList [][]Price
	for _, list := range sellExchanges {
		newPriceList = append(newPriceList, list)
	}
	findPriceDifferences(newPriceList...)
}

func PrintTable(c *gin.Context) {
	printTable(c)
}

func printTable(c *gin.Context) {
	mux.Lock()
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"USDTRY":                tryRate,
		"GdaxBTC":               coinbaseProPrices["BTC"].Ask,
		"ParibuBTCAsk":          diffs[GDAX+"-Paribu-BTC-Ask"],
		"ParibuBTCBid":          diffs[GDAX+"-Paribu-BTC-Bid"],
		"BTCTurkBTCAsk":         diffs[GDAX+"-BTCTurk-BTC-Ask"],
		"BTCTurkBTCBid":         diffs[GDAX+"-BTCTurk-BTC-Bid"],
		"KoineksBTCAsk":         diffs[GDAX+"-Koineks-BTC-Ask"],
		"KoineksBTCBid":         diffs[GDAX+"-Koineks-BTC-Bid"],
		"KoinimBTCAsk":          diffs[GDAX+"-Koinim-BTC-Ask"],
		"KoinimBTCBid":          diffs[GDAX+"-Koinim-BTC-Bid"],
		"BinanceBTCAsk":       	 diffs[GDAX+"-Binance-BTC-Ask"],
		"BinanceBTCBid":       	 diffs[GDAX+"-Binance-BTC-Bid"],
		"GdaxETH":               coinbaseProPrices["ETH"].Ask,
		"ParibuETHAsk":          diffs[GDAX+"-Paribu-ETH-Ask"],
		"ParibuETHBid":          diffs[GDAX+"-Paribu-ETH-Bid"],
		"BTCTurkETHAsk":         diffs[GDAX+"-BTCTurk-ETH-Ask"],
		"BTCTurkETHBid":         diffs[GDAX+"-BTCTurk-ETH-Bid"],
		"KoinimETHAsk":          diffs[GDAX+"-Koinim-ETH-Ask"],
		"KoinimETHBid":          diffs[GDAX+"-Koinim-ETH-Bid"],
		"BinanceETHAsk":       	 diffs[GDAX+"-Binance-ETH-Ask"],
		"BinanceETHBid":       	 diffs[GDAX+"-Binance-ETH-Bid"],
		"GdaxLTC":               coinbaseProPrices["LTC"].Ask,
		"ParibuLTCAsk":          diffs[GDAX+"-Paribu-LTC-Ask"],
		"ParibuLTCBid":          diffs[GDAX+"-Paribu-LTC-Bid"],
		"BTCTurkLTCAsk":         diffs[GDAX+"-BTCTurk-LTC-Ask"],
		"BTCTurkLTCBid":         diffs[GDAX+"-BTCTurk-LTC-Bid"],
		"KoinimLTCAsk":          diffs[GDAX+"-Koinim-LTC-Ask"],
		"KoinimLTCBid":          diffs[GDAX+"-Koinim-LTC-Bid"],
		"GdaxBCH":               coinbaseProPrices["BCH"].Ask,
		"BCHSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"BCH"]),
		"ParibuBCHAsk":          diffs[GDAX+"-Paribu-BCH-Ask"],
		"ParibuBCHBid":          diffs[GDAX+"-Paribu-BCH-Bid"],
		"KoinimBCHAsk":          diffs[GDAX+"-Koinim-BCH-Ask"],
		"KoinimBCHBid":          diffs[GDAX+"-Koinim-BCH-Bid"],
		"GdaxETC":               coinbaseProPrices["ETC"].Ask,
		"ETCSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"ETC"]),
		"BinanceETCAsk":         diffs[GDAX+"-Binance-ETC-Ask"],
		"BinanceETCBid":         diffs[GDAX+"-Binance-ETC-Bid"],
		"GdaxXLM":               coinbaseProPrices["XLM"].Ask,
		"XLMSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"XLM"]),
		"ParibuXLMAsk":          diffs[GDAX+"-Paribu-XLM-Ask"],
		"ParibuXLMBid":          diffs[GDAX+"-Paribu-XLM-Bid"],
		"BTCTurkXLMAsk":         diffs[GDAX+"-BTCTurk-XLM-Ask"],
		"BTCTurkXLMBid":         diffs[GDAX+"-BTCTurk-XLM-Bid"],
		"BinanceXLMAsk":       	 diffs[GDAX+"-Binance-XLM-Ask"],
		"BinanceXLMBid":       	 diffs[GDAX+"-Binance-XLM-Bid"],
		"GdaxEOS":               coinbaseProPrices["EOS"].Ask,
		"EOSSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"EOS"]),
		"ParibuEOSAsk":          diffs[GDAX+"-Paribu-EOS-Ask"],
		"ParibuEOSBid":          diffs[GDAX+"-Paribu-EOS-Bid"],
		"BinanceEOSAsk":       	 diffs[GDAX+"-Binance-EOS-Ask"],
		"BinanceEOSBid":         diffs[GDAX+"-Binance-EOS-Bid"],
		"GdaxLINK":               coinbaseProPrices["LINK"].Ask,
		"LINKSpread":             fmt.Sprintf("%.2f", spreads[GDAX+"LINK"]),
		"ParibuLINKAsk":          diffs[GDAX+"-Paribu-LINK-Ask"],
		"ParibuLINKBid":          diffs[GDAX+"-Paribu-LINK-Bid"],
		"BinanceLINKAsk":       	diffs[GDAX+"-Binance-LINK-Ask"],
		"BinanceLINKBid":       	diffs[GDAX+"-Binance-LINK-Bid"],
		"BTCTurkLINKAsk":         diffs[GDAX+"-BTCTurk-LINK-Ask"],
		"BTCTurkLINKBid":         diffs[GDAX+"-BTCTurk-LINK-Bid"],
		"GdaxDASH":              coinbaseProPrices["DASH"].Ask,
		"DASHSpread":            fmt.Sprintf("%.2f", spreads[GDAX+"DASH"]),
		"KoinimDASHAsk":      	 diffs[GDAX+"-Koinim-DASH-Ask"],
		"KoinimDASHBid":      	 diffs[GDAX+"-Koinim-DASH-Bid"],
		"GdaxMKR":              coinbaseProPrices["MKR"].Ask,
		"MKRSpread":            fmt.Sprintf("%.2f", spreads[GDAX+"MKR"]),
		"ParibuMKRAsk":          diffs[GDAX+"-Paribu-MKR-Ask"],
		"ParibuMKRBid":          diffs[GDAX+"-Paribu-MKR-Bid"],
		"GdaxADA":               coinbaseProPrices["ADA"].Ask,
		"ADASpread":             fmt.Sprintf("%.2f", spreads[GDAX+"ADA"]),
		"BinanceADAAsk":         diffs[GDAX+"-Binance-ADA-Ask"],
		"BinanceADABid":         diffs[GDAX+"-Binance-ADA-Bid"],
		"ParibuADAAsk":          diffs[GDAX+"-Paribu-ADA-Ask"],
		"ParibuADABid":          diffs[GDAX+"-Paribu-ADA-Bid"],
		"BTCTurkADAAsk":         diffs[GDAX+"-BTCTurk-ADA-Ask"],
		"BTCTurkADABid":         diffs[GDAX+"-BTCTurk-ADA-Bid"],
		"ParibuBTCAskPrice":     prices["Paribu-BTC-Ask"],
		"ParibuBTCBidPrice":     prices["Paribu-BTC-Bid"],
		"BTCTurkBTCAskPrice":    prices["BTCTurk-BTC-Ask"],
		"BTCTurkBTCBidPrice":    prices["BTCTurk-BTC-Bid"],
		"BinanceBTCAskPrice":    prices["Binance-BTC-Ask"],
		"BinanceBTCBidPrice":    prices["Binance-BTC-Bid"],
		"KoinimBTCAskPrice":     prices["Koinim-BTC-Ask"],
		"KoinimBTCBidPrice":     prices["Koinim-BTC-Bid"],
		"ParibuETHAskPrice":     prices["Paribu-ETH-Ask"],
		"ParibuETHBidPrice":     prices["Paribu-ETH-Bid"],
		"BTCTurkETHAskPrice":    prices["BTCTurk-ETH-Ask"],
		"BTCTurkETHBidPrice":    prices["BTCTurk-ETH-Bid"],
		"BinanceETHAskPrice":    prices["Binance-ETH-Ask"],
		"BinanceETHBidPrice":    prices["Binance-ETH-Bid"],
		"KoinimETHAskPrice":     prices["Koinim-ETH-Ask"],
		"KoinimETHBidPrice":     prices["Koinim-ETH-Bid"],
		"ParibuLTCAskPrice":     prices["Paribu-LTC-Ask"],
		"ParibuLTCBidPrice":     prices["Paribu-LTC-Bid"],
		"BTCTurkLTCAskPrice":    prices["BTCTurk-LTC-Ask"],
		"BTCTurkLTCBidPrice":    prices["BTCTurk-LTC-Bid"],
		"KoinimLTCAskPrice":     prices["Koinim-LTC-Ask"],
		"KoinimLTCBidPrice":     prices["Koinim-LTC-Bid"],
		"ParibuBCHAskPrice":     prices["Paribu-BCH-Ask"],
		"ParibuBCHBidPrice":     prices["Paribu-BCH-Bid"],
		"KoinimBCHAskPrice":     prices["Koinim-BCH-Ask"],
		"KoinimBCHBidPrice":     prices["Koinim-BCH-Bid"],
		"BinanceETCAskPrice":    prices["Binance-ETC-Ask"],
		"BinanceETCBidPrice":    prices["Binance-ETC-Bid"],
		"ParibuUSDTAskPrice":     prices["Paribu-USDT-Ask"],
		"ParibuUSDTBidPrice":     prices["Paribu-USDT-Bid"],
		"BTCTurkUSDTAskPrice":   prices["BTCTurk-USDT-Ask"],
		"BTCTurkUSDTBidPrice":   prices["BTCTurk-USDT-Bid"],
		"BinanceUSDTAskPrice":   prices["Binance-USDT-Ask"],
		"BinanceUSDTBidPrice":   prices["Binance-USDT-Bid"],
		"ParibuDOGEAskPrice":     prices["Paribu-DOGE-Ask"],
		"ParibuDOGEBidPrice":     prices["Paribu-DOGE-Bid"],
		"BinanceDOGEAskPrice":   prices["Binance-DOGE-Ask"],
		"BinanceDOGEBidPrice":   prices["Binance-DOGE-Bid"],
		"KoinimDOGEAskPrice":    prices["Koinim-DOGE-Ask"],
		"KoinimDOGEBidPrice":    prices["Koinim-DOGE-Bid"],
		"ParibuXLMAskPrice":     prices["Paribu-XLM-Ask"],
		"ParibuXLMBidPrice":     prices["Paribu-XLM-Bid"],
		"BTCTurkXLMAskPrice":    prices["BTCTurk-XLM-Ask"],
		"BTCTurkXLMBidPrice":    prices["BTCTurk-XLM-Bid"],
		"BinanceXLMAskPrice":    prices["Binance-XLM-Ask"],
		"BinanceXLMBidPrice":    prices["Binance-XLM-Bid"],
		"ParibuEOSAskPrice":     prices["Paribu-EOS-Ask"],
		"ParibuEOSBidPrice":     prices["Paribu-EOS-Bid"],
		"BinanceEOSAskPrice":    prices["Binance-EOS-Ask"],
		"BinanceEOSBidPrice":    prices["Binance-EOS-Bid"],
		"ParibuLINKAskPrice":     prices["Paribu-LINK-Ask"],
		"ParibuLINKBidPrice":     prices["Paribu-LINK-Bid"],
		"BTCTurkLINKAskPrice":    prices["BTCTurk-LINK-Ask"],
		"BTCTurkLINKBidPrice":    prices["BTCTurk-LINK-Bid"],
		"BinanceLINKAskPrice":    prices["Binance-LINK-Ask"],
		"BinanceLINKBidPrice":    prices["Binance-LINK-Bid"],
		"KoinimDASHAskPrice":    prices["Koinim-DASH-Ask"],
		"KoinimDASHBidPrice":    prices["Koinim-DASH-Bid"],
		"ParibuMKRAskPrice":     prices["Paribu-MKR-Ask"],
		"ParibuMKRBidPrice":     prices["Paribu-MKR-Bid"],
		"ParibuADAAskPrice":     prices["Paribu-ADA-Ask"],
		"ParibuADABidPrice":     prices["Paribu-ADA-Bid"],
		"BTCTurkADAAskPrice":     prices["BTCTurk-ADA-Ask"],
		"BTCTurkADABidPrice":     prices["BTCTurk-ADA-Bid"],
		"GdaxUSDT":              fmt.Sprintf("%.8f", coinbaseProPrices["USDT"].Ask),
		"USDTSpread":            fmt.Sprintf("%.2f", spreads[GDAX+"USDT"]),
		"ParibuUSDTAsk":         diffs[GDAX+"-Paribu-USDT-Ask"],
		"ParibuUSDTBid":         diffs[GDAX+"-Paribu-USDT-Bid"],
		"BTCTurkUSDTAsk":        diffs[GDAX+"-BTCTurk-USDT-Ask"],
		"BTCTurkUSDTBid":        diffs[GDAX+"-BTCTurk-USDT-Bid"],
		"BinanceUSDTAsk":        diffs[GDAX+"-Binance-USDT-Ask"],
		"BinanceUSDTBid":        diffs[GDAX+"-Binance-USDT-Bid"],
		"GdaxDOGE":              fmt.Sprintf("%.8f", coinbaseProPrices["DOGE"].Ask),
		"DOGESpread":     		   fmt.Sprintf("%.2f", spreads[GDAX+"DOGE"]),
		"ParibuDOGEAsk":         diffs[GDAX+"-Paribu-DOGE-Ask"],
		"ParibuDOGEBid":         diffs[GDAX+"-Paribu-DOGE-Bid"],
		"BinanceDOGEAsk":        diffs[GDAX+"-Binance-DOGE-Ask"],
		"BinanceDOGEBid":        diffs[GDAX+"-Binance-DOGE-Bid"],
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

		originP := coinbaseProPrices[symbol]
		tryP := Price{Currency: "TRY", Exchange: originP.Exchange, ID: originP.ID, Bid: originP.Bid * tryRate, Ask: originP.Ask * tryRate}
		tryList = append(tryList, tryP)

		for _, list := range priceLists {
			for _, p := range list {
				if p.ID == symbol {
					switch p.Currency {
					case "TRY":
						tryList = append(tryList, p)
					}
				}
			}
		}

		/*for _, p := range tryList {
			fmt.Println(fmt.Sprintf("ID %s Exchange %s Ask %f", p.ID, p.Exchange, p.Ask))
		}*/
		setDiffsAndPrices(tryList)
	}
}

func setDiffsAndPrices(list []Price) {
	firstExchange := "GDAX"
	firstAsk := 0.0
	for i, p := range list {
		if i == 0 {
			firstAsk = p.Ask
		} else {
			askPercentage := (p.Ask - firstAsk) * 100 / firstAsk
			bidPercentage := (p.Bid - firstAsk) * 100 / firstAsk

			askRound := Round(askPercentage, .5, 2)
			bidRound := Round(bidPercentage, .5, 2)

			mux.Lock()
			diffs[fmt.Sprintf("%s-%s-%s-%s", firstExchange, p.Exchange, p.ID, "Ask")] = askRound
			diffs[fmt.Sprintf("%s-%s-%s-%s", firstExchange, p.Exchange, p.ID, "Bid")] = bidRound

			//fmt.Println(fmt.Sprintf("%s-%s-%s-%s", firstExchange, p.Exchange, p.ID, "Ask"))

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
