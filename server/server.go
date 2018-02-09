package server

import (
    "bytes"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "os/exec"
    "runtime"
    "strconv"

    gdax "github.com/preichenberger/go-gdax"

    "github.com/buger/jsonparser"
    "github.com/gin-gonic/gin"
)

type Price struct {
  Exchange string
  Currency string
  ID string
  Ask float64
  Bid float64
}

const (
  BASE_CURRENCY_URI = "http://free.currencyconverterapi.com/api/v3/convert?q=USD_%s&compact=ultra"

  PARIBU_URI = "https://www.paribu.com/ticker"
  BTCTURK_URI = "https://www.btcturk.com/api/ticker"
  KOINEKS_URI = "https://koineks.com/ticker"
)

func Run() {
  port := os.Getenv("PORT")
  if port == "" {
    log.Fatal("$PORT must be set")
  }

  router := gin.New()
  router.Use(gin.Logger())

  router.GET("/", PrintTable)
}

func PrintTable(c *gin.Context) {
  gdaxPrices, err := getGdaxPrices()
  if err != nil {
    fmt.Println("Error reading GDAX prices : ", err)
  }

  paribuPrices, err := getParibuPrices()
  if err != nil {
    fmt.Println("Error reading Paribu prices : ", err)
  }

  btcTurkPrices, err := getBTCTurkPrices()
  if err != nil {
    fmt.Println("Error reading BTCTurk prices : ", err)
  }

  koineksTurkPrices, err := getKoineksPrices()
  if err != nil {
    fmt.Println("Error reading Koineks prices : ", err)
  }

  tryRate, err := getCurrencyRate("TRY")
  if err != nil {
    fmt.Println("Error reading the currency rate: ", err)
  }

  var buffer bytes.Buffer

  buffer.WriteString(findTRYPrices("BTC", tryRate, gdaxPrices, paribuPrices, btcTurkPrices, koineksTurkPrices))
  buffer.WriteString(findTRYPrices("ETH", tryRate, gdaxPrices, paribuPrices, btcTurkPrices, koineksTurkPrices))
  buffer.WriteString(findTRYPrices("LTC", tryRate, gdaxPrices, paribuPrices, btcTurkPrices, koineksTurkPrices))
}

func getCurrencyRate(symbol string) (float64, error) {

  response, err := http.Get(fmt.Sprintf(BASE_CURRENCY_URI, symbol))
  if err != nil {
    return 0, fmt.Errorf("failed to get currency response for %s : %s", symbol, err)
  }

  responseData, err := ioutil.ReadAll(response.Body)
  if err != nil {
    return 0, fmt.Errorf("failed to read currency response data : %s", err)
  }

  price, err := jsonparser.GetFloat(responseData, fmt.Sprintf("USD_%s", symbol))
  if err != nil {
    return 0, fmt.Errorf("failed to read the currency price from the response data: %s", err)
  }

  return price, nil
}

func getGdaxPrices() ([]Price, error) {

  client := gdax.NewClient("", "", "")
  var prices []Price

  ids := []string{"BTC-USD", "ETH-USD", "LTC-USD"}

  for _, id := range ids {
      ticker, err := client.GetTicker(id)
      if err != nil {
        return nil, fmt.Errorf("Error reading %s price : %s\n", id, err)
      }

      p := Price{Exchange: "GDAX", Currency : "USD", ID : id[0:3], Ask : ticker.Ask, Bid : ticker.Bid}
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

  prices = append(prices, Price{Exchange: "Paribu", Currency: "TRY", ID: "BTC", Ask: priceAsk, Bid: priceBid})
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

  prices = append(prices, Price{Exchange: "BTCTurk", Currency: "TRY", ID: "ETH", Ask: ethPriceAsk, Bid: ethPriceBid})

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

  ids := []string{"BTC", "ETH", "LTC"}

  for _, id := range ids {

    priceAsk, err := jsonparser.GetString(responseData, id, "ask")
    if err != nil {
      return nil, fmt.Errorf("failed to read the ask price from the Koineks response data: %s", err)
    }

    askF, _ := strconv.ParseFloat(priceAsk, 64)
    
    priceBid, err := jsonparser.GetString(responseData, id, "bid")
    if err != nil {
      return nil, fmt.Errorf("failed to read the bid price from the Koineks response data: %s", err)
    }

    askB, _ := strconv.ParseFloat(priceBid, 64)

    prices = append(prices, Price{Exchange: "Koineks", Currency: "TRY", ID: id, Ask: askF, Bid: askB})
  }

  return prices, nil
}

func findTRYPrices(symbol string, tryRate float64, priceLists... []Price ) string {
  var tryList []Price

  for _, list := range priceLists {
    for _, p := range list {
      if p.ID == symbol {
        if p.Currency == "USD" {
          p.Currency = "TRY"
          p.Bid *= tryRate
          p.Ask *= tryRate
        }
        tryList = append(tryList, p)
      }
    }
  }

  out := symbol + " "
  firstAsk := 0.0
  for i, p := range tryList {
    if i == 0 {
      out += fmt.Sprintf("%s Ask : %.2f ", p.Exchange, p.Ask)
      firstAsk = p.Ask
    } else {
      askPercentage := (p.Ask - firstAsk) * 100 / firstAsk 
      bidPercentage := (p.Bid - firstAsk) * 100 / firstAsk 
      out += fmt.Sprintf("%s Ask Diff. : %%%.2f Bid Diff. : %%%.2f ", p.Exchange, askPercentage, bidPercentage)
    }
  }
  fmt.Println(out)

  return out
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
    if ok { //if we defined a clear func for that platform:
        value()  //we execute it
    } else { //unsupported platform
        panic("Your platform is unsupported! I can't clear terminal screen :(")
    }
}
