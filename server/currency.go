package server

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/buger/jsonparser"
)

var (
	tryRate = 0.0
	aedRate = 0.0
)

func getCurrencies() {
	for {
		getCurrencyRates()
		time.Sleep(1 * time.Hour)
	}
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

	tryRateFloat, err := jsonparser.GetString(responseData, "Realtime Currency Exchange Rate", "5. Exchange Rate")
	if err != nil {
		fmt.Println("failed to read the TRY currency price from the response data: ", err)
		log.Println("failed to read the TRY currency price from the response data: ", err)
	}

	tempTryRate, _ := strconv.ParseFloat(tryRateFloat, 64)
	if tempTryRate != 0.0 {
		tryRate = tempTryRate
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

	aedRateFloat, err := jsonparser.GetString(responseData, "Realtime Currency Exchange Rate", "5. Exchange Rate")
	if err != nil {
		fmt.Println("failed to read the AED currency price from the response data: ", err)
		log.Println("failed to read the AED currency price from the response data: ", err)
	}
	tempAedRate, _ := strconv.ParseFloat(aedRateFloat, 64)
	if tempAedRate != 0.0 {
		aedRate = tempAedRate
	}
}
