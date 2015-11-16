package main
import (
		"log"
		"time"
	   )

func master(sites Sites, STOP chan struct{}, DONE chan []interface{}) {
	done := make(chan []interface{}, 50)
	num := 0
	for _, url := range sites.Urls {
		go worker(url, done)
		num++
	}
	if num == 0 {
		return
	}
	var results []interface{}
	results = append(results, time.Now().Unix())
	for num > 0 {
		select {
		case res := <- done:
			results = append(results, res...)
			num--
		case <- STOP:
			return
		}
	}
	log.Print("results: ", results)
	DONE <- results
}

func worker(url string, done chan []interface{}) {
	cli := NewClient(url)
	cli.Run()
	done <- cli.Results
}
