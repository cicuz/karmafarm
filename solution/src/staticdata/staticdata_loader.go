package staticdata

import (
	"bufio"
	"encoding/csv"
	"io"
	"log"
	"os"
	"time"
)

type Crowdsourcer struct {
	Id string `json:"id"`
	Name string `json:"name"`
}

type Severity struct {
	Id string `json:"id"`
	Value string `json:"value"`
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


func load_crowdsourcer() {
	csvFile, _ := os.Open("input/crowdsourcer.csv")
	reader := csv.NewReader(bufio.NewReader(csvFile))
	Cs = make(map[string]*Crowdsourcer)
	for {
		line, error := reader.Read()
		if error == io.EOF {
			break
		} else if error != nil {
			log.Fatal(error)
		}
		Cs[line[0]] = &Crowdsourcer{
			Id: line[0],
			Name:  line[1],
		}
	}
}

func load_severity() {
	csvFile, _ := os.Open("input/severity.csv")
	reader := csv.NewReader(bufio.NewReader(csvFile))
	Sev = make(map[string]*Severity)
	for {
		line, error := reader.Read()
		if error == io.EOF {
			break
		} else if error != nil {
			log.Fatal(error)
		}
		Sev[line[0]] = &Severity {
			Id: line[0],
			Value:  line[1],
		}
	}
}

func load_vulnerability() {
	csvFile, _ := os.Open("input/vulnerability.csv")
	reader := csv.NewReader(bufio.NewReader(csvFile))
	Vul = make(map[string]*Vulnerability)
	for {
		line, error := reader.Read()
		if error == io.EOF {
			break
		} else if error != nil {
			log.Fatal(error)
		}
		Vul[line[0]] = &Vulnerability{
			Id: line[0],
			Cs:  Cs[line[1]],
			Sev: Sev[line[2]],
		}
	}
}

func init() {
	load_crowdsourcer()
	load_severity()
	load_vulnerability()
}

