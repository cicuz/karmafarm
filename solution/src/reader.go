package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	static "staticdata"
	"time"
)

func worker(eventChan <-chan []string, doneChan <-chan bool) {
	fmt.Println("worker started")
	for {
		select {
		case event, _ := <-eventChan:
			layout := "2006-01-02 15:04:05" // January 2nd :roll_eyes:
			datetime, _ := time.Parse(layout, event[1])
			finding := static.Finding{
				Id:            event[0],
				Time:          datetime,
				Vulnerability: static.Vul[event[2]],
			}

			findingJson, _ := json.Marshal(finding)
			fmt.Println(string(findingJson))
		case done, _ := <-doneChan:
			if done {
				fmt.Println("worker exiting ")
				return
			}
		}
	}
}

func main() {
	// instantiating the channels
	eventChan := make(chan []string, 100)
	doneChan := make(chan bool, 1)

	// start a worker
	fmt.Println(runtime.NumGoroutine())
	go worker(eventChan, doneChan)
	fmt.Println(runtime.NumGoroutine())

	file, err := os.Open("input/finding.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(bufio.NewReader(file))
	workerscounter := 0
	counter := 0
	for {
		finding, err := reader.Read()
		if err == nil {
			counter += 1
			select {
			case eventChan <- finding:
				// channel not full
			default:
				// channel full, possibly add worker and then requeue message with a blocking call
				if workerscounter < 20 {
					fmt.Println("adding worker")
					go worker(eventChan, doneChan)
					workerscounter += 1
				}
				eventChan <- finding
				fmt.Println("added worker ", runtime.NumGoroutine())
			}
		} else {
			if err == io.EOF {
				if workerscounter > 2 {
					doneChan <- true
					workerscounter -= 1
				}
				fmt.Println("sleeeeeeeeping ", counter, workerscounter)
				time.Sleep(1 * time.Second)
			} else {
				break
			}
		}
	}
}
