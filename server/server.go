package server

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
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

	wg.Wait()
}

func getCurrencies() {
	for {
		getCurrencyRates()
		time.Sleep(1 * time.Hour)
	}
}

func PrintTable(c *gin.Context) {
	gdaxPrices, err := getGdaxPrices()
	if err != nil {
		fmt.Println("Error reading GDAX prices : ", err)
		log.Println("Error reading GDAX prices : ", err)
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	paribuPrices, err := getParibuPrices()
	if err != nil {
		fmt.Println("Error reading Paribu prices : ", err)
		log.Println("Error reading Paribu prices : ", err)
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	btcTurkPrices, err := getBTCTurkPrices()
	if err != nil {
		fmt.Println("Error reading BTCTurk prices : ", err)
		log.Println("Error reading BTCTurk prices : ", err)
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	koineksPrices, err := getKoineksPrices()
	if err != nil {
		fmt.Println("Error reading Koineks prices : ", err)
		log.Println("Error reading Koineks prices : ", err)
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	bitflyerPrices, err := getBitflyerPrices()
	if err != nil {
		fmt.Println("Error reading Bitflyer prices : ", err)
		log.Println("Error reading Bitflyer prices : ", err)
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	btcDiffs := findPriceDifferences("BTC", tryRate, gdaxPrices, paribuPrices, btcTurkPrices, koineksPrices, bitflyerPrices)
	ethDiffs := findPriceDifferences("ETH", tryRate, gdaxPrices, btcTurkPrices, koineksPrices)
	ltcDiffs := findPriceDifferences("LTC", tryRate, gdaxPrices, koineksPrices)

	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"USDTRY":         tryRate,
		"USDJPY":         jpyRate,
		"GdaxBTC":        gdaxPrices[0].Ask,
		"ParibuBTCAsk":   btcDiffs[0],
		"ParibuBTCBid":   btcDiffs[1],
		"BTCTurkBTCAsk":  btcDiffs[2],
		"BTCTurkBTCBid":  btcDiffs[3],
		"KoineksBTCAsk":  btcDiffs[4],
		"KoineksBTCBid":  btcDiffs[5],
		"BitflyerBTCAsk": btcDiffs[6],
		"BitflyerBTCBid": btcDiffs[7],
		"GdaxETH":        gdaxPrices[1].Ask,
		"BTCTurkETHAsk":  ethDiffs[0],
		"BTCTurkETHBid":  ethDiffs[1],
		"KoineksETHAsk":  ethDiffs[2],
		"KoineksETHBid":  ethDiffs[3],
		"GdaxLTC":        gdaxPrices[2].Ask,
		"KoineksLTCAsk":  ltcDiffs[0],
		"KoineksLTCBid":  ltcDiffs[1],
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

func findPriceDifferences(symbol string, tryRate float64, priceLists ...[]Price) []string {
	var tryList []Price
	var jpyList []Price
	var returnPercentages []string
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

			returnPercentages = append(returnPercentages, fmt.Sprintf("%.2f", askPercentage), fmt.Sprintf("%.2f", bidPercentage))
		}
	}

	for i, p := range jpyList {
		if i == 0 {
			firstAsk = p.Ask
		} else {
			askPercentage := (p.Ask - firstAsk) * 100 / firstAsk
			bidPercentage := (p.Bid - firstAsk) * 100 / firstAsk

			returnPercentages = append(returnPercentages, fmt.Sprintf("%.2f", askPercentage), fmt.Sprintf("%.2f", bidPercentage))
		}
	}
	//fmt.Print(out)

	return returnPercentages
}

var clear map[string]func() //create a map for storing clear funcs

func init() {
	clear = make(map[string]func()) //Initialize it
	clear["linux"] = func() {
		cmd := exec.Command("clear") //Linux example, its tested
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
	clear["darwin"] = func() {
		cmd := exec.Command("clear") //Linux example, its tested
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
	clear["windows"] = func() {
		cmd := exec.Command("cmd", "/c", "cls") //Windows example, its tested
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

func CallClear() {
	value, ok := clear[runtime.GOOS] //runtime.GOOS -> linux, windows, darwin etc.
	if ok {                          //if we defined a clear func for that platform:
		value() //we execute it
	} else { //unsupported platform
		panic("Your platform is unsupported! I can't clear terminal screen :(")
	}
}
