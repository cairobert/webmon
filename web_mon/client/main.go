package main
import (
		"fmt"
		"log"
		"os"
		"os/signal"
		"time"
	   )

func main() {
	log.Print("client starting...")
	ctr := NewController()
	err := ctr.Init()
	if err != nil {
		log.Print(err)
		return
	}
	go ctr.Run()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	s := <- c
	fmt.Println("received ", s)
	ctr.Quit()
	time.Sleep(1e9)
	log.Print("done.")
}
