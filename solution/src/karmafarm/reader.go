package main

import (
	"encoding/json"
	"fmt"
	_ "github.com/go-kivik/couchdb"
	"github.com/go-kivik/kivik"
	"log"
	"os"
	"strings"
	"github.com/hpcloud/tail"
	"golang.org/x/net/context"
	static "karmafarm/staticdata"
	"time"
)

var logger *log.Logger

func init() {
	f, err := os.OpenFile(static.LogLocation + "/karmafarm.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()

	logger = log.New(f, "", log.LstdFlags)
}

func worker(eventChan chan []string, doneChan <-chan bool) {
	logger.Println("worker started")
	fmt.Println("worker started")
	client, _ := kivik.New("couch", static.CouchdbURL)
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
			logger.Println(string(findingJson))
			fmt.Println(string(findingJson))
			rev, err := db.Put(context.TODO(), event[0], finding)
			if err != nil {
				// document was existing, try to update it
				logger.Println("first err ", err, rev, finding)
				fmt.Println("first err ", err, rev, finding)
				row := db.Get(context.TODO(), event[0])
				var m map[string]interface{}
				json.Unmarshal(findingJson, &m)
				logger.Println(row.Rev, m)
				fmt.Println(row.Rev, m)
				m["_rev"] = row.Rev
				findingJson, _ = json.Marshal(m)
				rev, err := db.Put(context.TODO(), event[0], m)
				if err != nil {
					// update failed (probably another concurrent update), requeue the message
					logger.Println("second err ", err, rev, finding)
					fmt.Println("second err ", err, rev, finding)
					eventChan <- []string{event[0], event[1], event[2]}
				} else {
					logger.Println("first done ", err, rev, finding)
					fmt.Println("first done ", err, rev, finding)
				}
			}
		case done, _ := <-doneChan:
			if done {
				logger.Println("worker exiting")
				fmt.Println("worker exiting")
				return
			}
		}
	}
}

func main() {
	// instantiate the channels
	eventChan := make(chan []string, static.MsgQueueSize)
	doneChan := make(chan bool, 1)

	// start a worker
	go worker(eventChan, doneChan)
	workerscounter := 1

	tailfile, _ := tail.TailFile(static.InputLocation + "/finding.csv", tail.Config{Follow: true,
			         															    Poll: true})
	offset, _ := tailfile.Tell()

	counter := 0
	for {
		select {
		case line := <-tailfile.Lines:
			finding := strings.Split(line.Text, ",")
			if len(finding) == 3 {
				offset, _ = tailfile.Tell()
			} else {
				tailfile, _ = tail.TailFile(static.InputLocation + "/finding.csv", tail.Config{Follow: true,
					Poll: true,
					Location: &tail.SeekInfo{Offset: offset, Whence: 0}})
				continue
			}
			counter += 1
			select {
			case eventChan <- finding:
				// channel not full
			default:
				// channel full, possibly add worker and then requeue message with a blocking call
				if workerscounter < static.MaxGoroutine {
					go worker(eventChan, doneChan)
					workerscounter += 1
				}
				eventChan <- finding
			}
		default:
			if workerscounter > 1 {
				doneChan <- true
				workerscounter -= 1
				logger.Printf("workerscounter %v\n", workerscounter)
				fmt.Printf("workerscounter %v\n", workerscounter)
			}
		}
	}
}
