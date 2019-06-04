package main

import (
	"encoding/json"
	_ "github.com/go-kivik/couchdb"
	"github.com/go-kivik/kivik"
	"github.com/hpcloud/tail"
	"golang.org/x/net/context"
	static "karmafarm/staticdata"
	"log"
	"os"
	"strings"
	"time"
)

var logger *log.Logger

func worker(eventChan chan []string, doneChan <-chan bool) {
	logger.Println("worker started")
	client, _ := kivik.New("couch", static.CouchdbURL)
	db := client.DB(context.TODO(), "finding")
	for {
		select {
		// fetch an event from the communication channel
		case event, _ := <-eventChan:
			layout := "2006-01-02 15:04:05" // January 2nd :roll_eyes:
			// parse the date into an object
			datetime, _ := time.Parse(layout, event[1])
			// fetch the vulnerability from the database via the id
			vulnerability := static.GetVulnerability(event[2])
			// create a finding object
			finding := static.Finding{
				Id:            event[0],
				Time:          datetime,
				Vulnerability: &vulnerability,
			}

			findingJson, _ := json.Marshal(finding)
			logger.Println(string(findingJson))
			// put the serialized object into the database
			rev, err := db.Put(context.TODO(), event[0], finding)
			if err != nil {
				// document was existing, try to update it
				logger.Println("first err ", err, rev, finding)
				// fetch the existing object (and its `_rev` field) from the database
				row := db.Get(context.TODO(), event[0])
				old_finding := static.Finding{}
				_ = row.ScanDoc(&old_finding)
				findingJson, _ := json.Marshal(old_finding)
				logger.Println(string(findingJson))
				// they are assumed to be equal if Id, Time, and Vulnerability are all the same
				if finding.Time == old_finding.Time && finding.Vulnerability.Id == old_finding.Vulnerability.Id {
					logger.Println("rows data matched; not updating")
					continue
				}

				// rows data did not match; update record
				var m map[string]interface{}
				_ = json.Unmarshal(findingJson, &m)
				logger.Println(row.Rev, m)
				// set the `_rev` field so that CouchDB will perform an update
				m["_rev"] = row.Rev
				findingJson, _ = json.Marshal(m)
				// put the newer object into the database
				rev, err := db.Put(context.TODO(), event[0], m)
				if err != nil {
					// update failed (probably another concurrent update)
					logger.Println("second err ", err, rev, finding)
					// send the message into the channes again, for another attempt
					eventChan <- []string{event[0], event[1], event[2]}
				} else {
					logger.Println("first done ", err, rev, finding)
				}
			}
		// if there's a message in the `done` channel, consume it and quit
		case done, _ := <-doneChan:
			if done {
				logger.Println("worker exiting")
				return
			}
		}
	}
}

func main() {
	// instantiate a logger
	f, err := os.OpenFile(static.LogLocation + "/karmafarm.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()

	logger = log.New(f, "", log.LstdFlags)

	// instantiate the channels
	eventChan := make(chan []string, static.MsgQueueSize)
	doneChan := make(chan bool, 1)

	// start a worker
	go worker(eventChan, doneChan)
	workerscounter := 1

	// open input file with a tail-like reader
	tailfile, _ := tail.TailFile(static.InputLocation + "/finding.csv", tail.Config{Follow: true,
			         															    Poll: true,
																				    Logger: logger})
	// get reading offset position
	offset, _ := tailfile.Tell()

	counter := 0
	for {
		select {
		// read from the input channel
		case line := <-tailfile.Lines:
			// check (quite naïvely) that the line we just read was whole
			finding := strings.Split(line.Text, ",")
			if len(finding) == 3 {
				offset, _ = tailfile.Tell()
			} else {
				// if the line was incomplete, backoff to the last known valid offset
				tailfile, _ = tail.TailFile(static.InputLocation + "/finding.csv", tail.Config{Follow: true,
																							   Poll: true,
																							   Logger: logger,
																							   Location: &tail.SeekInfo{Offset: offset, Whence: 0}})
				continue
			}
			counter += 1
			select {
			case eventChan <- finding:
				// channel not full
			default:
				// channel full, conditionally add worker and then requeue message with a blocking call
				if workerscounter < static.MaxGoroutine {
					go worker(eventChan, doneChan)
					workerscounter += 1
				}
				eventChan <- finding
			}
		// if there are no messages to be processed, reduce the number of workers
		default:
			if workerscounter > 1 {
				// first worker to consume the message in the `done` channel will quit
				doneChan <- true
				workerscounter -= 1
				logger.Printf("workerscounter %v\n", workerscounter)
			}
		}
	}
}
