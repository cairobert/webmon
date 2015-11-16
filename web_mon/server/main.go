package main
import (
		_"github.com/Go-SQL-Driver/MySQL"
		"log"
		"os"
		"os/signal"
		"runtime"
		"time"
	   )

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	log.Print("starting...")
	go serveWeb()
	go hostMonitor()
	s := <- c
	log.Print("Got signal: ", s)
	log.Print("closing")
	quit()
	time.Sleep(1e9)
	log.Print("finished")
}
