package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/minio/pkg/env"
)

var (
	logFile   string
	address   string
	authToken = env.Get("WEBHOOK_AUTH_TOKEN", "")
)

func processJSONData(jsonData map[string]interface{}) {
	// Convert jsonData to []byte
	data, err := json.Marshal(jsonData)
	if err != nil {
		log.Println("Error marshaling jsonData:", err)
		return
	}

	// Create a Backend instance

	// Call ProcessJSONData function from sendevent.go
	event, err := ProcessJSONData(data)
	if err != nil {
		log.Println("Error processing JSON data:", err)
		return
	}

	// Print the event
	fmt.Println(event)
}

func main() {
	flag.StringVar(&logFile, "log-file", "", "path to the file where webhook will log incoming events")
	flag.StringVar(&address, "address", ":8080", "bind to a specific ADDRESS:PORT, ADDRESS can be an IP or hostname")

	flag.Parse()

	var mu sync.Mutex

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP)

	go func() {
		for _ = range sigs {
			mu.Lock()
			mu.Unlock()
		}
	}()

	err := http.ListenAndServe(address, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authToken != "" {
			if authToken != r.Header.Get("Authorization") {
				http.Error(w, "authorization header missing", http.StatusBadRequest)
				return
			}
		}
		switch r.Method {
		case http.MethodPost:
			mu.Lock()
			data, err := ioutil.ReadAll(r.Body)
			if err != nil {
				mu.Unlock()
				return
			}

			var jsonData map[string]interface{}
			err = json.Unmarshal(data, &jsonData)
			if err != nil {
				mu.Unlock()
				return
			}

			processJSONData(jsonData)

			mu.Unlock()
		default:
		}
	}))
	if err != nil {
		log.Fatal(err)
	}
}
