package server

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
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
		time.Sleep(6 * time.Hour)
	}
}

func getCurrencyRates() {
	req, err := http.NewRequest(http.MethodGet, BASE_CURRENCY_URI, nil)
	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
	}

	req.Header.Set("apikey", "8JOnEfDOQ6nlcGkpDrSaAB08vbJNLYrF")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("client: error making http request: %s\n", err)
	}

	fmt.Printf("client: got response!\n")
	fmt.Printf("client: status code: %d\n", res.StatusCode)

	resData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("client: could not read response body: %s\n", err)
	}
	fmt.Printf("client: response body: %s\n", resData)

	/*response, err := http.Get(fmt.Sprintf(BASE_CURRENCY_URI, "TRY"))
	if err != nil {
		fmt.Println("failed to get response for currencies : ", err)
		log.Println("failed to get response for currencies : ", err)
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Println("failed to read currency response data : ", err)
		log.Println("failed to read currency response data : ", err)
	}*/

	tryRateFloat, err := jsonparser.GetFloat(resData, "rates", "TRY")
	if err != nil {
		fmt.Println("failed to read the TRY currency price from the response data: ", err)
		log.Println("failed to read the TRY currency price from the response data: ", err)
	}

	if tryRateFloat != 0.0 {
		tryRate = tryRateFloat
	}

	fmt.Printf("TRY Rate: %f\n", tryRateFloat)
}
