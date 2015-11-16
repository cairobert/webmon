package services

import (
		"net/http"
		"strings"
		"time"
	   )

//The function issues a GET to the specified url. It returns 
// the time it costs.
func Get(url string) (time.Duration, error) {
	if strings.Index(url, "//") == -1 {
		url = "http://" + url
	}
	t1 := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	t2 := time.Now()
	return t2.Sub(t1), nil
}
