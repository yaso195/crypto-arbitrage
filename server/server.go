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
	BASE_CURRENCY_URI = "http://free.currencyconverterapi.com/api/v3/convert?q=USD_%s,USD_%s&compact=ultra"
)

var (
	tryRate = 0.0
	jpyRate = 0.0

	diffs                                                                              map[string]float64
	crossDiffs                                                                         map[string]float64
	prices, spreads                                                                    map[string]float64
	usdPrices, poloniexPrices, bittrexPrices                                           map[string]Price
	btcTurkETHBTCAskBid, btcTurkETHBTCBidAsk                                           float64
	koineksETHBTCAskBid, koineksETHBTCBidAsk, koineksLTCBTCAskBid, koineksLTCBTCBidAsk float64
	koinimLTCBTCAskBid, koinimLTCBTCBidAsk                                             float64
	fiatNotificationEnabled, pairNotificationEnabled                                   = false, false

	ALL_SYMBOLS = []string{"BTC", "ETH", "LTC", "DOGE", "DASH", "XRP", "XLM", "XEM"}
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
	if err != nil || len(gdaxPrices) != 5 {
		fmt.Println("Error reading GDAX prices : ", err)
		log.Println("Error reading GDAX prices : ", err)
		return
	}

	bittrexPrices, err = getBittrexPrices()
	if err != nil || len(bittrexPrices) != len(bittrexCurrencies) {
		fmt.Println("Error reading Bittrex prices : ", err)
		log.Println("Error reading Bittrex prices : ", err)
		return
	}

	bitcoinPrice := gdaxPrices[0].Ask
	for _, p := range bittrexPrices {
		tempP := p
		tempP.Ask *= bitcoinPrice
		tempP.Bid *= bitcoinPrice
		gdaxPrices = append(gdaxPrices, tempP)
	}

	updateUSDPrices(gdaxPrices)

	paribuPrices, err := getParibuPrices()
	if err != nil {
		fmt.Println("Error reading Paribu prices : ", err)
		log.Println("Error reading Paribu prices : ", err)
		return
	}

	btcTurkPrices, err := getBTCTurkPrices()
	if err != nil {
		fmt.Println("Error reading BTCTurk prices : ", err)
		log.Println("Error reading BTCTurk prices : ", err)
		return
	}

	koineksPrices, err := getKoineksPrices()
	if err != nil {
		fmt.Println("Error reading Koineks prices : ", err)
		log.Println("Error reading Koineks prices : ", err)
		return
	}

	koinimPrices, err := getKoinimPrices()
	if err != nil {
		fmt.Println("Error reading Koinim prices : ", err)
		log.Println("Error reading Koinim prices : ", err)
		return
	}

	bitflyerPrices, err := getBitflyerPrices()
	if err != nil {
		fmt.Println("Error reading Bitflyer prices : ", err)
		log.Println("Error reading Bitflyer prices : ", err)
		return
	}

	findPriceDifferences(gdaxPrices, paribuPrices, btcTurkPrices, koineksPrices, koinimPrices, bitflyerPrices)

	sendMessages()
}

func PrintTable(c *gin.Context) {

	ethBTCPrice := usdPrices["ETH-BTC"].Ask
	ltcBTCPrice := usdPrices["LTC-BTC"].Ask
	crossDiffs["BTCTurkETHBTCAskBid"] = Round((btcTurkETHBTCAskBid-ethBTCPrice)*100/ethBTCPrice, .5, 2)
	crossDiffs["BTCTurkETHBTCBidAsk"] = Round((btcTurkETHBTCBidAsk-ethBTCPrice)*100/ethBTCPrice, .5, 2)
	crossDiffs["KoineksETHBTCAskBid"] = Round((koineksETHBTCAskBid-ethBTCPrice)*100/ethBTCPrice, .5, 2)
	crossDiffs["KoineksETHBTCBidAsk"] = Round((koineksETHBTCBidAsk-ethBTCPrice)*100/ethBTCPrice, .5, 2)
	crossDiffs["KoineksLTCBTCAskBid"] = Round((koineksLTCBTCAskBid-ltcBTCPrice)*100/ltcBTCPrice, .5, 2)
	crossDiffs["KoineksLTCBTCBidAsk"] = Round((koineksLTCBTCBidAsk-ltcBTCPrice)*100/ltcBTCPrice, .5, 2)
	crossDiffs["KoinimLTCBTCAskBid"] = Round((koinimLTCBTCAskBid-ltcBTCPrice)*100/ltcBTCPrice, .5, 2)
	crossDiffs["KoinimLTCBTCBidAsk"] = Round((koinimLTCBTCBidAsk-ltcBTCPrice)*100/ltcBTCPrice, .5, 2)

	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"USDTRY":              tryRate,
		"USDJPY":              jpyRate,
		"GdaxBTC":             usdPrices["BTC"].Ask,
		"ParibuBTCAsk":        diffs["ParibuBTCAsk"],
		"ParibuBTCBid":        diffs["ParibuBTCBid"],
		"BTCTurkBTCAsk":       diffs["BTCTurkBTCAsk"],
		"BTCTurkBTCBid":       diffs["BTCTurkBTCBid"],
		"KoineksBTCAsk":       diffs["KoineksBTCAsk"],
		"KoineksBTCBid":       diffs["KoineksBTCBid"],
		"KoinimBTCAsk":        diffs["KoinimBTCAsk"],
		"KoinimBTCBid":        diffs["KoinimBTCBid"],
		"BitflyerBTCAsk":      diffs["BitflyerBTCAsk"],
		"BitflyerBTCBid":      diffs["BitflyerBTCBid"],
		"GdaxETH":             usdPrices["ETH"].Ask,
		"BTCTurkETHAsk":       diffs["BTCTurkETHAsk"],
		"BTCTurkETHBid":       diffs["BTCTurkETHBid"],
		"KoineksETHAsk":       diffs["KoineksETHAsk"],
		"KoineksETHBid":       diffs["KoineksETHBid"],
		"GdaxLTC":             usdPrices["LTC"].Ask,
		"KoineksLTCAsk":       diffs["KoineksLTCAsk"],
		"KoineksLTCBid":       diffs["KoineksLTCBid"],
		"KoinimLTCAsk":        diffs["KoinimLTCAsk"],
		"KoinimLTCBid":        diffs["KoinimLTCBid"],
		"GdaxETHBTC":          ethBTCPrice,
		"GdaxLTCBTC":          ltcBTCPrice,
		"BTCTurkETHBTCAskBid": crossDiffs["BTCTurkETHBTCAskBid"],
		"BTCTurkETHBTCBidAsk": crossDiffs["BTCTurkETHBTCBidAsk"],
		"KoineksETHBTCAskBid": crossDiffs["KoineksETHBTCAskBid"],
		"KoineksETHBTCBidAsk": crossDiffs["KoineksETHBTCBidAsk"],
		"KoineksLTCBTCAskBid": crossDiffs["KoineksLTCBTCAskBid"],
		"KoineksLTCBTCBidAsk": crossDiffs["KoineksLTCBTCBidAsk"],
		"KoinimLTCBTCAskBid":  crossDiffs["KoinimLTCBTCAskBid"],
		"KoinimLTCBTCBidAsk":  crossDiffs["KoinimLTCBTCBidAsk"],
		"ParibuBTCAskPrice":   prices["ParibuBTCAsk"],
		"ParibuBTCBidPrice":   prices["ParibuBTCBid"],
		"BTCTurkBTCAskPrice":  prices["BTCTurkBTCAsk"],
		"BTCTurkBTCBidPrice":  prices["BTCTurkBTCBid"],
		"KoineksBTCAskPrice":  prices["KoineksBTCAsk"],
		"KoineksBTCBidPrice":  prices["KoineksBTCBid"],
		"KoinimBTCAskPrice":   prices["KoinimBTCAsk"],
		"KoinimBTCBidPrice":   prices["KoinimBTCBid"],
		"BitflyerBTCAskPrice": fmt.Sprintf("%.2f", prices["BitflyerBTCAsk"]),
		"BitflyerBTCBidPrice": fmt.Sprintf("%.2f", prices["BitflyerBTCBid"]),
		"BTCTurkETHAskPrice":  prices["BTCTurkETHAsk"],
		"BTCTurkETHBidPrice":  prices["BTCTurkETHBid"],
		"KoineksETHAskPrice":  prices["KoineksETHAsk"],
		"KoineksETHBidPrice":  prices["KoineksETHBid"],
		"KoineksLTCAskPrice":  prices["KoineksLTCAsk"],
		"KoineksLTCBidPrice":  prices["KoineksLTCBid"],
		"KoinimLTCAskPrice":   prices["KoinimLTCAsk"],
		"KoinimLTCBidPrice":   prices["KoinimLTCBid"],
		"KoineksDOGEAskPrice": prices["KoineksDOGEAsk"],
		"KoineksDOGEBidPrice": prices["KoineksDOGEBid"],
		"KoineksDASHAskPrice": prices["KoineksDASHAsk"],
		"KoineksDASHBidPrice": prices["KoineksDASHBid"],
		"BTCTurkXRPAskPrice":  prices["BTCTurkXRPAsk"],
		"BTCTurkXRPBidPrice":  prices["BTCTurkXRPBid"],
		"KoineksXRPAskPrice":  prices["KoineksXRPAsk"],
		"KoineksXRPBidPrice":  prices["KoineksXRPBid"],
		"KoineksXLMAskPrice":  prices["KoineksXLMAsk"],
		"KoineksXLMBidPrice":  prices["KoineksXLMBid"],
		"KoineksXEMAskPrice":  prices["KoineksXEMAsk"],
		"KoineksXEMBidPrice":  prices["KoineksXEMBid"],
		"GdaxDOGE":            fmt.Sprintf("%.8f", usdPrices["DOGE"].Ask),
		"BittrexDOGEAsk":      fmt.Sprintf("%.8f", bittrexPrices["DOGE"].Ask),
		"BittrexDOGESpread":   fmt.Sprintf("%.2f", spreads["DOGE"]),
		"KoineksDOGEAsk":      diffs["KoineksDOGEAsk"],
		"KoineksDOGEBid":      diffs["KoineksDOGEBid"],
		"GdaxDASH":            fmt.Sprintf("%.2f", usdPrices["DASH"].Ask),
		"BittrexDASHAsk":      fmt.Sprintf("%.8f", bittrexPrices["DASH"].Ask),
		"BittrexDASHSpread":   fmt.Sprintf("%.2f", spreads["DASH"]),
		"KoineksDASHAsk":      diffs["KoineksDASHAsk"],
		"KoineksDASHBid":      diffs["KoineksDASHBid"],
		"GdaxXRP":             fmt.Sprintf("%.3f", usdPrices["XRP"].Ask),
		"BittrexXRPAsk":       fmt.Sprintf("%.8f", bittrexPrices["XRP"].Ask),
		"BittrexXRPSpread":    fmt.Sprintf("%.2f", spreads["XRP"]),
		"BTCTurkXRPAsk":       diffs["BTCTurkXRPAsk"],
		"BTCTurkXRPBid":       diffs["BTCTurkXRPBid"],
		"KoineksXRPAsk":       diffs["KoineksXRPAsk"],
		"KoineksXRPBid":       diffs["KoineksXRPBid"],
		"GdaxXLM":             fmt.Sprintf("%.3f", usdPrices["XLM"].Ask),
		"BittrexXLMAsk":       fmt.Sprintf("%.8f", bittrexPrices["XLM"].Ask),
		"BittrexXLMSpread":    fmt.Sprintf("%.2f", spreads["XLM"]),
		"KoineksXLMAsk":       diffs["KoineksXLMAsk"],
		"KoineksXLMBid":       diffs["KoineksXLMBid"],
		"GdaxXEM":             fmt.Sprintf("%.3f", usdPrices["XEM"].Ask),
		"BittrexXEMAsk":       fmt.Sprintf("%.8f", bittrexPrices["XEM"].Ask),
		"BittrexXEMSpread":    fmt.Sprintf("%.2f", spreads["XEM"]),
		"KoineksXEMAsk":       diffs["KoineksXEMAsk"],
		"KoineksXEMBid":       diffs["KoineksXEMBid"],
	})
}

func SetNotificationLimits(c *gin.Context) {
	minimumStr := c.Query("minimum")
	maximumStr := c.Query("maximum")
	durationStr := c.Query("duration")
	fiatEnable := c.Query("fiatEnable")
	pairEnable := c.Query("pairEnable")
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

	switch pairEnable {
	case "true":
		pairNotificationEnabled = true
	case "false":
		pairNotificationEnabled = false
	default:
		pairNotificationEnabled = false
	}

	c.HTML(http.StatusOK, "notification.tmpl", gin.H{
		"Minimum":    MIN_NOTI_PERC,
		"Maximum":    MAX_NOTI_PERC,
		"Duration":   DURATION,
		"PThreshold": PAIR_THRESHOLD,
	})
}

func getCurrencyRates() {
	response, err := http.Get(fmt.Sprintf(BASE_CURRENCY_URI, "TRY", "JPY"))
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

	jpyRate, err = jsonparser.GetFloat(responseData, "USD_JPY")
	if err != nil {
		fmt.Println("failed to read the TRY currency price from the response data: ", err)
		log.Println("failed to read the TRY currency price from the response data: ", err)
	}
}

func findPriceDifferences(priceLists ...[]Price) {
	for _, symbol := range ALL_SYMBOLS {
		var tryList []Price
		var jpyList []Price
		for _, list := range priceLists {
			for _, p := range list {
				if p.ID == symbol {
					switch p.Currency {
					case "USD":
						tryP := Price{Currency: "TRY", Exchange: p.Exchange, Bid: p.Bid * tryRate, Ask: p.Ask * tryRate}
						jpyP := Price{Currency: "JPY", Exchange: p.Exchange, Bid: p.Bid * jpyRate, Ask: p.Ask * jpyRate}
						tryList = append(tryList, tryP)
						jpyList = append(jpyList, jpyP)
					case "TRY":
						tryList = append(tryList, p)
					case "JPY":
						jpyList = append(jpyList, p)
					}
				}
			}
		}

		firstAsk := 0.0
		for i, p := range tryList {
			if i == 0 {
				firstAsk = p.Ask
			} else {
				askPercentage := (p.Ask - firstAsk) * 100 / firstAsk
				bidPercentage := (p.Bid - firstAsk) * 100 / firstAsk

				diffs[fmt.Sprintf("%s%s%s", p.Exchange, symbol, "Ask")] = Round(askPercentage, .5, 2)
				diffs[fmt.Sprintf("%s%s%s", p.Exchange, symbol, "Bid")] = Round(bidPercentage, .5, 2)
				prices[fmt.Sprintf("%s%s%s", p.Exchange, symbol, "Ask")] = p.Ask
				prices[fmt.Sprintf("%s%s%s", p.Exchange, symbol, "Bid")] = p.Bid
			}
		}

		for i, p := range jpyList {
			if i == 0 {
				firstAsk = p.Ask
			} else {
				askPercentage := (p.Ask - firstAsk) * 100 / firstAsk
				bidPercentage := (p.Bid - firstAsk) * 100 / firstAsk

				diffs[fmt.Sprintf("%s%s%s", p.Exchange, symbol, "Ask")] = Round(askPercentage, .5, 2)
				diffs[fmt.Sprintf("%s%s%s", p.Exchange, symbol, "Bid")] = Round(bidPercentage, .5, 2)
				prices[fmt.Sprintf("%s%s%s", p.Exchange, symbol, "Ask")] = Round(p.Ask, .5, 2)
				prices[fmt.Sprintf("%s%s%s", p.Exchange, symbol, "Bid")] = Round(p.Bid, .5, 2)
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
			existingPrice, ok := usdPrices[p.Currency]
			if ok {
				existingPrice.Ask = p.Ask
				existingPrice.Bid = p.Bid
			} else {
				usdPrices[p.ID] = Price{Currency: "USD", ID: p.ID, Ask: p.Ask, Bid: p.Bid}
			}
		}
	}
}
