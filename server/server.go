package server

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
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
	prices                                                                             map[string]float64
	gdaxPrices                                                                         []Price
	btcTurkETHBTCAskBid, btcTurkETHBTCBidAsk                                           float64
	koineksETHBTCAskBid, koineksETHBTCBidAsk, koineksLTCBTCAskBid, koineksLTCBTCBidAsk float64
	koinimLTCBTCAskBid, koinimLTCBTCBidAsk                                             float64

	ALL_SYMBOLS = []string{"BTC", "ETH", "LTC"}
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
	gdaxPrices, err = getGdaxPrices()
	if err != nil {
		fmt.Println("Error reading GDAX prices : ", err)
		log.Println("Error reading GDAX prices : ", err)
		return
	}

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
	if len(gdaxPrices) < 5 {
		c.String(http.StatusInternalServerError, "Failed to fetch prices")
		return
	}

	crossDiffs["BTCTurkETHBTCAskBid"] = Round((btcTurkETHBTCAskBid-gdaxPrices[3].Ask)*100/gdaxPrices[3].Ask, .5, 2)
	crossDiffs["BTCTurkETHBTCBidAsk"] = Round((btcTurkETHBTCBidAsk-gdaxPrices[3].Ask)*100/gdaxPrices[3].Ask, .5, 2)
	crossDiffs["KoineksETHBTCAskBid"] = Round((koineksETHBTCAskBid-gdaxPrices[3].Ask)*100/gdaxPrices[3].Ask, .5, 2)
	crossDiffs["KoineksETHBTCBidAsk"] = Round((koineksETHBTCBidAsk-gdaxPrices[3].Ask)*100/gdaxPrices[3].Ask, .5, 2)
	crossDiffs["KoineksLTCBTCAskBid"] = Round((koineksLTCBTCAskBid-gdaxPrices[4].Ask)*100/gdaxPrices[4].Ask, .5, 2)
	crossDiffs["KoineksLTCBTCBidAsk"] = Round((koineksLTCBTCBidAsk-gdaxPrices[4].Ask)*100/gdaxPrices[4].Ask, .5, 2)
	crossDiffs["KoinimLTCBTCAskBid"] = Round((koinimLTCBTCAskBid-gdaxPrices[4].Ask)*100/gdaxPrices[4].Ask, .5, 2)
	crossDiffs["KoinimLTCBTCBidAsk"] = Round((koinimLTCBTCBidAsk-gdaxPrices[4].Ask)*100/gdaxPrices[4].Ask, .5, 2)

	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"USDTRY":              tryRate,
		"USDJPY":              jpyRate,
		"GdaxBTC":             gdaxPrices[0].Ask,
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
		"GdaxETH":             gdaxPrices[1].Ask,
		"BTCTurkETHAsk":       diffs["BTCTurkETHAsk"],
		"BTCTurkETHBid":       diffs["BTCTurkETHBid"],
		"KoineksETHAsk":       diffs["KoineksETHAsk"],
		"KoineksETHBid":       diffs["KoineksETHBid"],
		"GdaxLTC":             gdaxPrices[2].Ask,
		"KoineksLTCAsk":       diffs["KoineksLTCAsk"],
		"KoineksLTCBid":       diffs["KoineksLTCBid"],
		"KoinimLTCAsk":        diffs["KoinimLTCAsk"],
		"KoinimLTCBid":        diffs["KoinimLTCBid"],
		"GdaxETHBTC":          gdaxPrices[3].Ask,
		"GdaxLTCBTC":          gdaxPrices[4].Ask,
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
