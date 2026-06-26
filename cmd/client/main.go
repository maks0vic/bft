package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"net/http"
)

func main() {
	leader := flag.String("leader", "localhost:8001", "leader address")
	value := flag.String("value", "attack", "proposal value")
	flag.Parse()

	body, err := json.Marshal(map[string]string{
		"value": *value,
	})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.Post("http://"+*leader+"/start", "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	log.Printf("start response: %s", resp.Status)
}
