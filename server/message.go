package server

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	PUSHOVER_URI = "https://api.pushover.net/1/messages.json"
)

var (
	notificationFlags  map[string]bool
	notificationTimes  map[string]time.Time
	PUSHOVER_USER      = ""
	PUSHOVER_APP_TOKEN = ""

	MIN_NOTI_PERC  = -2.0
	MAX_NOTI_PERC  = 3.25
	PAIR_THRESHOLD = 1.0
	DURATION       = 10.0
)

func sendMessages() {
	var out string
	if fiatNotificationEnabled {
		for _, exchange := range ALL_EXCHANGES {
			if exchange == PARIBU || exchange == BTCTURK {
				continue
			}

			for _, symbol := range ALL_SYMBOLS {
				exchangeSymbol := fmt.Sprintf("%s-%s", exchange, symbol)

				notificationFlag := notificationFlags[exchangeSymbol]
				notificationTime := notificationTimes[exchangeSymbol]
				duration := time.Since(notificationTime)

				commissionFee := 0.0
				firstExchange := GDAX
				if symbol == "USDT" || symbol == "DOGE" || symbol == "XEM"{
					firstExchange = BINANCE
					commissionFee = 0.1
				}
				mux.Lock()
				spread := spreads[fmt.Sprintf("%s%s", firstExchange, symbol)]

				exchangeSymbolAsk := fmt.Sprintf("%s-%s", exchangeSymbol, "Ask")
				exchangeSymbolBid := fmt.Sprintf("%s-%s", exchangeSymbol, "Bid")
				askDiff := diffs[fmt.Sprintf("%s-%s", firstExchange, exchangeSymbolAsk)]
				bidDiff := diffs[fmt.Sprintf("%s-%s", firstExchange, exchangeSymbolBid)]
				mux.Unlock()

				if bidDiff > askDiff {
					continue
				}

				if notificationFlag && askDiff > MIN_NOTI_PERC-commissionFee - spread && bidDiff < MAX_NOTI_PERC+commissionFee {
					notificationFlags[exchangeSymbol] = false
				}

				if !notificationFlag && duration.Minutes() >= DURATION &&
					(askDiff <= MIN_NOTI_PERC-commissionFee - spread || bidDiff >= MAX_NOTI_PERC+commissionFee) {
					notificationFlags[exchangeSymbol] = true
					notificationTimes[exchangeSymbol] = time.Now()

					if askDiff <= MIN_NOTI_PERC {
						askPrice := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", prices[exchangeSymbolAsk]), "0"), ".")
						out += fmt.Sprintf("%s %s %%%.2f %s\n", exchange, symbol, askDiff, askPrice)
					} else {
						bidPrice := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", prices[exchangeSymbolBid]), "0"), ".")
						out += fmt.Sprintf("%s %s %%%.2f %s\n", exchange, symbol, bidDiff, bidPrice)
					}
				}
			}
		}
	}

	sendPushoverMessage(out)
}

func sendPushoverMessage(message string) {
	if message == "" {
		return
	}

	// POST
	form := url.Values{
		"user":    {PUSHOVER_USER},
		"token":   {PUSHOVER_APP_TOKEN},
		"message": {message},
	}

	body := bytes.NewBufferString(form.Encode())
	_, err := http.Post(PUSHOVER_URI, "application/x-www-form-urlencoded", body)
	if err != nil {
		fmt.Println("Failed to send the message to pushover : ", err)
		log.Println("Failed to send the message to pushover : ", err)
	}

	log.Println("Sent message ", message)
}
