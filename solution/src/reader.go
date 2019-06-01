package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	_ "github.com/go-kivik/couchdb"
	"github.com/go-kivik/kivik"
	"golang.org/x/net/context"
	"io"
	"log"
	"os"
	"runtime"
	static "staticdata"
	"time"
)

func worker(eventChan chan []string, doneChan <-chan bool) {
	fmt.Println("worker started")
	client, _ := kivik.New("couch", "http://admin:pass@localhost:5984/")
	db := client.DB(context.TODO(), "finding")
	for {
		select {
		case event, _ := <-eventChan:
			layout := "2006-01-02 15:04:05" // January 2nd :roll_eyes:
			datetime, _ := time.Parse(layout, event[1])
			vulnerability := static.GetVulnerability(event[2])
			finding := static.Finding{
				Id:            event[0],
				Time:          datetime,
				Vulnerability: &vulnerability,
			}

			findingJson, _ := json.Marshal(finding)
			rev, err := db.Put(context.TODO(), event[0], finding)
			if err != nil {
				// document was existing, try to update it
				fmt.Println("first err ", err, rev, finding)
				row := db.Get(context.TODO(), event[0])
				var m map[string]interface{}
				json.Unmarshal(findingJson, &m)
				fmt.Println(row.Rev, m)
				m["_rev"] = row.Rev
				findingJson, _ = json.Marshal(m)
				rev, err := db.Put(context.TODO(), event[0], m)
				if err != nil {
					// update failed (probably another concurrent update), requeue the message
					fmt.Println("second err ", err, rev, finding)
					eventChan <- ([]string{event[0], event[1], event[2]})
				} else {
					fmt.Println("first done ", err, rev, finding)
				}
			}
		case done, _ := <-doneChan:
			if done {
				fmt.Println("worker exiting")
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
	workerscounter := 1
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
				fmt.Println("added worker and job")
			}
		} else {
			if err == io.EOF {
				if workerscounter > 1 {
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
