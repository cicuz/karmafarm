package staticdata

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	_ "github.com/go-kivik/couchdb"
	"github.com/go-kivik/kivik"
	"golang.org/x/net/context"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

// get environment variables with fallback value if not set
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

var CouchdbURL = getEnv("COUCHDB_URL","http://admin:pass@localhost:5984/")
var MaxGoroutine, _ = strconv.Atoi(getEnv("MAX_GOROUTINE", "20"))
var MsgQueueSize, _ = strconv.Atoi(getEnv("MSG_QUEUE_SIZE", "100"))
var InputLocation = getEnv("INPUT_LOCATION", "../../../input")
var LogLocation = getEnv("LOG_LOCATION", "../../logs")

// define models
type Crowdsourcer struct {
	Id string `json:"id"`
	Name string `json:"name"`
}

type Severity struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Karma int `json:"karma"`
}

type Vulnerability struct {
	Id string `json:"id"`
	Cs *Crowdsourcer `json:"crowdsourcer"`
	Sev *Severity `json:"severity"`
}

type Finding struct {
	Id string `json:"id"`
	Time time.Time `json:"time"`
	Vulnerability *Vulnerability `json:"vulnerability"`
}

var Cs map[string]*Crowdsourcer
var Vul map[string]*Vulnerability
var Sev map[string]*Severity


func load_crowdsourcers() {
	csvFile, _ := os.Open(InputLocation + "/crowdsourcer.csv")
	reader := csv.NewReader(bufio.NewReader(csvFile))
	Cs = make(map[string]*Crowdsourcer)
	client, _ := kivik.New("couch", CouchdbURL)
	db := client.DB(context.TODO(), "crowdsourcer")
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		Cs[line[0]] = &Crowdsourcer{
			Id: line[0],
			Name:  line[1],
		}
		csJson, _ := json.Marshal(*Cs[line[0]])
		_, _ = db.Put(context.TODO(), line[0], csJson)
	}
}

func GetCrowdsourcer(cs_id string) (Crowdsourcer) {
	client, _ := kivik.New("couch", CouchdbURL)
	db := client.DB(context.TODO(), "crowdsourcer")
	row := db.Get(context.TODO(), cs_id)
	cs := Crowdsourcer{}
	_ = row.ScanDoc(&cs)
	return cs
}

func load_severities() {
	csvFile, _ := os.Open(InputLocation + "/severity.csv")
	reader := csv.NewReader(bufio.NewReader(csvFile))
	Sev = make(map[string]*Severity)
	client, _ := kivik.New("couch", CouchdbURL)
	db := client.DB(context.TODO(), "severity")
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		karma, _ := strconv.Atoi(line[2])
		Sev[line[0]] = &Severity {
			Id: line[0],
			Name:  line[1],
			Karma: karma,
		}
		sevJson, _ := json.Marshal(*Sev[line[0]])
		_, _ = db.Put(context.TODO(), line[0], sevJson)
	}
}

func GetSeverity(sev_id string) (Severity) {
	client, _ := kivik.New("couch", CouchdbURL)
	db := client.DB(context.TODO(), "severity")
	row := db.Get(context.TODO(), sev_id)
	sev := Severity{}
	_ = row.ScanDoc(&sev)
	return sev
}

func load_vulnerabilities() {
	csvFile, _ := os.Open(InputLocation + "/vulnerability.csv")
	reader := csv.NewReader(bufio.NewReader(csvFile))
	Vul = make(map[string]*Vulnerability)
	client, _ := kivik.New("couch", CouchdbURL)
	db := client.DB(context.TODO(), "vulnerability")
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		crowdsourcer := GetCrowdsourcer(line[1])
		severity := GetSeverity(line[2])
		Vul[line[0]] = &Vulnerability{
			Id: line[0],
			Cs:  &crowdsourcer,
			Sev: &severity,
		}
		vulJson, _ := json.Marshal(*Vul[line[0]])
		_, _ = db.Put(context.TODO(), line[0], vulJson)
	}
}

func GetVulnerability(vul_id string) (Vulnerability) {
	client, _ := kivik.New("couch", CouchdbURL)
	db := client.DB(context.TODO(), "vulnerability")
	row := db.Get(context.TODO(), vul_id)
	vul := Vulnerability{}
	_ = row.ScanDoc(&vul)
	return vul
}

// conditionally wipe a database, then create it anew
func initdb(dbname string, refreshdb bool) {
	client, _ := kivik.New("couch", CouchdbURL)
	dbexists, _ := client.DBExists(context.TODO(), dbname)
	if dbexists && refreshdb {
		_ = client.DestroyDB(context.TODO(), dbname)
	}
	_ = client.CreateDB(context.TODO(), dbname)
}

func add_map_and_list_views() {
	client, _ := kivik.New("couch", CouchdbURL)
	db := client.DB(context.TODO(), "finding")
	_, _ = db.Put(context.TODO(), "_design/findingDesign", map[string]interface{}{
		"_id": "_design/findingDesign",
		"views": map[string]interface{}{
			"findingcs": map[string]interface{}{
				"map":    "function (doc) {  var date_split = doc.time.slice(0, 10).split('-');  emit([doc.vulnerability.crowdsourcer.name, doc.vulnerability.severity.name, date_split[0], date_split[1], date_split[2]], doc.vulnerability.severity.karma);}",
				"reduce": "_sum",
			},
		},
		"lists": map[string]interface{}{
			"contributors": "function(head, req){    start({        'headers': {            'Content-Type': 'text/html'        }    });    send('<html><link rel=\"stylesheet\" href=\"https://cdnjs.cloudflare.com/ajax/libs/milligram/1.3.0/milligram.min.css\" /><body>');    var row = getRow();    send('<table style=\"width: 33%;margin: auto;\"><thead><tr>');    if(row.key) {        if(row.key.length > 0) send('<th>Crowdsourcer</th>');        if(row.key.length > 1) send('<th>Severity</th>');        if(row.key.length > 2) send('<th>Year</th>');        if(row.key.length > 3) send('<th>Month</th>');        if(row.key.length > 4) send('<th>Day</th>');    }    send('<th>Karma</th></tr></thead><tbody>');    while(row){        var key_string = '';        if(row.key) {            for(i=0;i<row.key.length;i++) {                key_string += ('<td>' + row.key[i] + '</td>');            }        }        send(''.concat(            '<tr>',            key_string,            '<td>' + row.value + '</td>',            '</tr>'        ));        row = getRow();    }    send('</tbody></table></body></html>');}",
		},
	})
}

func init() {
	var wg sync.WaitGroup
	// init system databases only if they don't exist already
	for _, dbname := range []string{"_users", "_replicator"} {
		go func(dbname string) {
			defer wg.Done()
			initdb(dbname, false)
		}(dbname)
		wg.Add(1)
	}
	wg.Wait()

	// wipe and initialize application databases
	for _, dbname := range []string{"finding", "crowdsourcer", "severity", "vulnerability"} {
		go func(dbname string) {
			defer wg.Done()
			initdb(dbname, true)
		}(dbname)
		wg.Add(1)
	}
	wg.Wait()

	// add view functions
	go func() {
		defer wg.Done()
		add_map_and_list_views()
	}()
	// load crowdsourcers table
	go func() {
		defer wg.Done()
		load_crowdsourcers()
	}()
	// load severities values
	go func() {
		defer wg.Done()
		load_severities()
	}()
	wg.Add(3)
	wg.Wait()

	// load vulnerabilities list (depends on croudsourcers and severities)
	load_vulnerabilities()
}
