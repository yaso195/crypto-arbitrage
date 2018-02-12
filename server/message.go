package server

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

const (
	PUSHOVER_URI = "https://api.pushover.net/1/messages.json"

	MIN_NOTI_PERC = -2.0
	MAX_NOTI_PERC = 2.5
)

var (
	notificationFlags  map[string]bool
	notificationTimes  map[string]time.Time
	PUSHOVER_USER      = ""
	PUSHOVER_APP_TOKEN = ""
)

func sendMessages() {
	var out string
	for _, exchange := range ALL_EXCHANGES {
		for _, symbol := range ALL_SYMBOLS {
			exchangeSymbol := fmt.Sprintf("%s%s", exchange, symbol)
			notificationFlag := notificationFlags[exchangeSymbol]
			notificationTime := notificationTimes[exchangeSymbol]
			askDiff := diffs[fmt.Sprintf("%s%s", exchangeSymbol, "Ask")]
			bidDiff := diffs[fmt.Sprintf("%s%s", exchangeSymbol, "Bid")]
			if notificationFlag && askDiff > MIN_NOTI_PERC && bidDiff < MAX_NOTI_PERC {
				notificationFlags[exchangeSymbol] = false
			}

			duration := time.Since(notificationTime)
			if !notificationFlag && duration.Minutes() >= 5 &&
				(askDiff <= MIN_NOTI_PERC || bidDiff >= MAX_NOTI_PERC) {
				notificationFlags[exchangeSymbol] = true
				notificationTimes[exchangeSymbol] = time.Now()
				if askDiff <= MIN_NOTI_PERC {
					out += fmt.Sprintf("%s %s %%%.2f\n", exchange, symbol, askDiff)
				} else {
					out += fmt.Sprintf("%s %s %%%.2f\n", exchange, symbol, bidDiff)
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
