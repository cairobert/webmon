package main
import (
		"encoding/json"
		"log"
		"net"
		"strconv"
		"strings"
		"sync"
		"time"
	   )

type Controller struct {
	Groups		[]Sites				// a group consits of urls sharing same mode
	Mu			sync.RWMutex		// protects Groups
	Results		[]string			// holding results to be sent
	QUIT		chan struct{}
	UPDATE		chan struct{}
	id			int
	serverAddr	string
	sending 	sync.Mutex			// protects write operation on conn
}

type Sites struct {
	Urls			[]string	// sites
	Freq			int			// units: min
}

func NewController() *Controller {
	ctr := new(Controller)
	ctr.serverAddr = SERVER_ADDR
	ctr.QUIT = make(chan struct{})
	ctr.UPDATE= make(chan struct{})
	return ctr
}

func (ctr *Controller) SetServerAddr(addr string) {
	ctr.serverAddr = addr
}

func (ctr *Controller) Init() (err error) {
	conn, err := net.Dial("tcp", ctr.serverAddr)
	if err != nil {
		return err
	}
	err = ctr.request(conn)
	conn.Close()
	return
}
	
func (ctr *Controller) request(conn net.Conn) (err error) {
	log.Print("ctr.request:")
	var iniStr = []interface{}{"INIT"}
	sends, err := json.Marshal(iniStr)
	if err != nil {
		log.Print("error in request: ", err)
		return
	}
	_, err = conn.Write(sends)				// fisrt send the INIT and local addr to it
	if err != nil {
		log.Print("conn.Write(): ", err)
		return
	}
	conn.SetDeadline(time.Now().Add(time.Duration(10) * time.Second))
	defer func() {
		var zero time.Time
		conn.SetDeadline(zero)
	}()

	ret := make([]byte, 1500)
	n, err := conn.Read(ret)
	if err != nil {
		log.Print("conn.Read(): ", err)
		return
	}
	id := -1	
	err = json.Unmarshal(ret[:n], &id)		// fetch id
	if err != nil {
		log.Print("Fail to communicate with server: ", err)
		return
	}
	ctr.id = id
	go ctr.Listen(ctr.QUIT)
	return nil
}

// the run function. should be called in a go statement
func (ctr *Controller) Run() {
	log.Print("ctr.Run...")
	var STOP chan struct{}
	DONE := make(chan []interface{}, 20)

	tms := []int{1, 3, 5, 10}
	ticker1 := time.NewTicker(1 * time.Minute)
	ticker2 := time.NewTicker(3 * time.Minute)
	ticker3 := time.NewTicker(5 * time.Minute)
	ticker4 := time.NewTicker(10 * time.Minute)
	tickers := []*time.Ticker{ticker1, ticker2, ticker3, ticker4}
	mFn := func(tm int) {
		log.Print("mode: ", tm, " running...")
		ctr.Mu.RLock()
		log.Print("ctr.Groups")
		for _, sites := range ctr.Groups {
			if sites.Freq == tm {
				go master(sites, STOP, DONE)
				break
			}
		}
		ctr.Mu.RUnlock()
	}
	time.Sleep(5 * 1e9)
	go mFn(tms[0])			// instant update
	go mFn(tms[1])
	go mFn(tms[2])
	go mFn(tms[3])
	for {
		select {
		case <- tickers[0].C:
			mFn(tms[0])
		case <- tickers[1].C:
			mFn(tms[1])
		case <- tickers[2].C:
			mFn(tms[2])
		case <- tickers[3].C:
			mFn(tms[3])
		case <- ctr.QUIT:
			if STOP != nil {
				close(STOP)
			}
			return
		case res := <- DONE:
			ctr.SendResults(res)
		case <- UPDATE:
			//close(STOP)
			time.Sleep(5 * 1e9)
			//STOP = make(chan struct{})
		}
	}

}

// the entry that the server connects to client
// should be called in a go statement
func (ctr *Controller) Listen(QUIT chan struct{}) {
	log.Print("client listenning on: ", NODE_ADDR)
	listener, err := net.Listen("tcp", NODE_ADDR)
	if err != nil {
		log.Fatal(err)
	}
	for {
		select {
		case <- QUIT:
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				log.Print("error in acceping: ", err)
				continue
			}
			log.Print("server ", conn.RemoteAddr().String(), " connected")
			go ctr.serveConn(conn)
		}
	}
}

func (ctr *Controller) serveConn(conn net.Conn) {
	defer conn.Close()
	log.Print("in serveConn")
	var rets []byte
	for {
		ret := make([]byte, 2500)
		n, err := conn.Read(ret)		// has only one read entry, no race conndition
		if err != nil {
			break
		}
		rets = append(rets, ret[:n]...)
	}
	ctr.parseCmd(rets)
}

func (ctr *Controller) parseCmd(rets []byte) {
	log.Print("in parseCmd, rets = ", string(rets))
	cmds := strings.Split(string(rets), ";")
	log.Print("cmds: ", cmds)
	if len(cmds) == 0 {
		log.Print("error: no command")
		return
	}
	ctr.Mu.Lock()
	defer ctr.Mu.Unlock()
	for _, cmd := range cmds {
		
		conts := strings.Fields(cmd)
		log.Print("conts: ", conts)
		if len(conts) == 0 {
			continue
		}
		if len(conts) != 3 && conts[0] != "INIT" {
			//fmt.Printf("len(conts) = %d, len(conts[0]) = %d\n", len(conts), len(conts[0]))
			log.Print("wrong command format: ", conts, " len(conts) = ", len(conts))
			continue
		}
		switch conts[0] {
		case "INIT": 				// format: INIT; IP FREQ1;...
			ctr.initUrls(cmds)		// init operation.
			return 					// all urls inserted, break the loop
		case "INSERT":				// format: INSERT IP FREQ;...
			ctr.insert(conts[1:3])	// not init operation
			//go ctr.instantUpdate(conts[1])
		case "UPDATE":
			ctr.update(conts[1:3])
		case "DELETE":
			ctr.delete(conts[1:3])
		default:
			log.Print("unknown command")
		}
	}
	log.Print("ctr.Groups: ", ctr.Groups)
}

func (ctr *Controller) initUrls(cmds []string) {
	cmds = cmds[1:]			// trim the "INIT"
	for _, cmd := range cmds {
		conts := strings.Fields(cmd)
		if len(conts) != 2 {
			continue
		}
		ctr.insert(conts)
	}
}

// conts format: {IP FREQ}
func (ctr *Controller) insert(conts []string) {
	log.Print("ctr.insert(): ", conts)
	freq, err := strconv.ParseInt(conts[1], 10, 32)
	if err != nil {
		log.Print("Fail to parse command: ", err)
		return
	}
	done := false
	for i := range ctr.Groups {
		if ctr.Groups[i].Freq == int(freq) {
			for _, v := range ctr.Groups[i].Urls {		// check if it is already in the list
				if v == conts[0] {
					return
				}
			}
			ctr.Groups[i].Urls = append(ctr.Groups[i].Urls, conts[0])
			done = true
			break
		}	
	}
	if !done {
		sites := Sites{Urls: []string{conts[0]}, Freq: int(freq)}
		ctr.Groups = append(ctr.Groups, sites)
	}
}

// conts format: {IP FREQ}
func (ctr *Controller) delete(conts []string) {
	log.Print("ctr.delete(): ", conts)
	freq, err := strconv.ParseInt(conts[1], 10, 32)
	if err != nil {
		log.Print("Fail to parse command: ", err)
		return
	}
	for i := range ctr.Groups {
		if ctr.Groups[i].Freq == int(freq) {
			for j, url := range ctr.Groups[i].Urls {
				if url == conts[0] {
					if j < len(ctr.Groups[i].Urls) - 1 {		// not the last one
						ctr.Groups[i].Urls = append(ctr.Groups[i].Urls[0:j], ctr.Groups[i].Urls[j+1:]...)
					} else {
						ctr.Groups[i].Urls = ctr.Groups[i].Urls[0:j]
					}
					if len(ctr.Groups[i].Urls) == 0 {		// if this group is empty, remove it
						if i < len(ctr.Groups) - 1 {		// not the last one
							ctr.Groups = append(ctr.Groups[0:i], ctr.Groups[i+1:]...)
						} else {
							ctr.Groups = ctr.Groups[0:i]
						}
					}
				}
			}
		}
	}
}

// conts format: {IP FREQ}
func (ctr *Controller) update(conts []string) {
	log.Print("ctr.update(): ", conts)
	ctr.delete(conts)
	ctr.insert(conts)
}

func (ctr *Controller) Quit() {
	close(ctr.QUIT)
}

// format: [id, time, ip1;lossRate,rtt;dns;http, ip2;lossRate,rtt;dns;http ...] json list
func (ctr *Controller) SendResults(res []interface{}) {
	log.Print("In SendResults")
	conn, err := net.Dial("tcp", ctr.serverAddr)
	if err != nil {
		log.Print("error in SendResults: ", err)
		return
	}
	defer conn.Close()
	sends := make([]interface{}, 2)
	sends[0] = "MONITOR"
	sends[1] = ctr.id
	sends = append(sends, res...)
	bs, err := json.Marshal(sends)
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))	// timeout: 10s
	_, err = conn.Write(bs)
	if err != nil {
		log.Print("error in ctr.SendResults(): ", err)
	}
	log.Print("send results: ", sends)
}

func (ctr *Controller) instantUpdate(url string) {
	done := make(chan []interface{}, 1)
	go worker(url, done)
	results := <- done
	ctr.SendResults(results)
}
