package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	static "staticdata"
	"time"
)

func main() {
	csJson, _ := json.Marshal(static.Cs)
	fmt.Println(string(csJson))

	sevJson, _ := json.Marshal(static.Sev)
	fmt.Println(string(sevJson))

	vulJson, _ := json.Marshal(static.Vul)
	fmt.Println(string(vulJson))

	file, err := os.Open("input/finding.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(bufio.NewReader(file))

	for {
		line, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				fmt.Println("sleeeeeeeeping")
				time.Sleep(1 * time.Second)
			} else {
				break
			}
		} else {
			layout := "2006-01-02 15:04:05"  // January 2nd :roll_eyes:
			time, _ := time.Parse(layout, line[1])
			finding := static.Finding{
				Id:            line[0],
				Time:          time,
				Vulnerability: static.Vul[line[2]],
			}

			findingJson, _ := json.Marshal(finding)
			fmt.Println(string(findingJson))
		}
	}
}
