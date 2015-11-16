package main
import (
		"log"
		"math/rand"
		"net"
		"net/http"
		"strings"
		"time"
	   )

type Client struct {
	url			string
	Results		[]interface{}
}

const (
		DefaultTimeout		= 			time.Duration(20 * time.Second) // exceeding 20s will be timeout
		NPING				=			10
		icmpv4EchoRequest = 8
		icmpv4EchoReply = 0
		icmpv6EchoRequest = 128
		icmpv6EchoReply = 129
	  )

func NewClient(url string) *Client {
	cli := new(Client)
	cli.url = url
	return cli
}

func (cli *Client) Run() {
	pingCh := make(chan interface{}, 1)
	dnsCh := make(chan interface{}, 1)
	httpCh := make(chan interface{}, 1)
	go cli.http(httpCh)
	go cli.ping(pingCh)
	go cli.dns(dnsCh)
	ret := make([]interface{}, 4)
	ret[0] =  cli.url
	for i := 0; i < 3; i++ {
		select {
		case s := <- pingCh:		// []interface{}{lossRate, rtt}
			ret[1] = s
		case s := <- dnsCh:
			ret[2] = s
		case s := <- httpCh:
			ret[3] = s
		}
	}
	cli.Results = ret
}

func dialTimeout(network, addr string) (conn net.Conn, err error) {
	return net.DialTimeout(network, addr, DefaultTimeout)
}

func (cli *Client) http(ch chan interface{}) {
	log.Print("in client http")
	trans := http.Transport{Dial: dialTimeout}
	c := http.Client{Transport: &trans}
	url := cli.url
	
	if strings.Index(url, "//") == -1 {
		url = "http://" + url
	}
	log.Print("In client http:, url: ", url)
	t1 := time.Now()
	resp, err := c.Get(url)
	if err != nil {	
		log.Print("http failed: ", err)
		ch <- time.Duration(0)
	} else {
		resp.Body.Close()
		ch <- time.Now().Sub(t1)
	}
}

func (cli *Client) ping(ch chan interface{}) {
	conn, err := net.DialTimeout("ip4:icmp", cli.url, DefaultTimeout)
	if err != nil {
		log.Print("ping: ", err)
		ch <- []interface{}{1, 0}
		return
	}
	defer conn.Close()
	lossRate, rtt := pinger(conn, DefaultTimeout)
	ch <- []interface{}{lossRate, rtt}
}

//Send and read datagrams from conn
func pinger(conn net.Conn, timeout time.Duration) (lossRate float64, rtt time.Duration) {
	defer func() {
		var zero time.Time
		conn.SetDeadline(zero)
	}()

	len := 8		// icmp echo request datagram length
	var ttlTime time.Duration = 0
	rand.Seed(time.Now().Unix())
	pid := rand.Intn(100000)
	var id1 = byte((pid & 0xff00) >> 8)
	var id2 = byte(pid & 0xff)
	var ts [NPING]time.Time
	go func() {
		for i := 0; i < NPING; i++ {
			var msg [100]byte
			msg[0] = icmpv4EchoRequest		// type: echo request, 8
			msg[1] = 0				// code: 0
			msg[2] = 0				// checksum[0],	fix later
			msg[3] = 0 				// checksum[1] 
			msg[4] = id1 			// id[0], use the last two bytes of pid
			msg[5] = id2			// id[1]
			msg[6] = 0				// sequence[0]
			msg[7] = byte(i+1)		// sequence[1]	
			check := CheckSum(msg[0:len])
			msg[2] = byte(check >> 8)
			msg[3] = byte(check & 0xff)

			t1 := time.Now()
			ts[i] = t1
			conn.SetDeadline(t1.Add(timeout))
			if _, err := conn.Write(msg[0:len]); err != nil {
				log.Print("Fail: ", err)
				continue
			}
			if i != NPING-1 {
				time.Sleep(1e9)
			}
		}
	}()
	timer := time.NewTimer(timeout)
	var running = true
	var done [NPING]int
	iDone := 0
	rcv := make([]byte, 512)
	for running {
		select {
		case <- timer.C:
			running = false
			break
		default:
			_, err := conn.Read(rcv)
			if err == nil {
				data := rcv[20:28]
				//log.Print("rcv: ", rcv[0:n])
				if data[0] != byte(0) {
					continue
				}
				if data[0] == byte(0) && data[1] == byte(0) && data[4] == id1 && data[5] == id2 && data[7] <= byte(NPING) {
					i := int(data[7])
					if done[i-1] == 0 {
						ttlTime += time.Now().Sub(ts[i-1])
						done[i-1] = 1
						iDone++
						if iDone == NPING {
							running = false
							timer.Stop()
							break
						}
					}
				}
			}
		}
	}
	
	if iDone == 0 {
		lossRate = float64(0.0)
		rtt = time.Duration(0)
	} else {
		lossRate = 1 - float64(iDone) / float64(NPING)
		rtt = time.Duration(ttlTime.Nanoseconds() / int64(iDone))
	}
	return
}
		
// Caculates the checksum of buf
func CheckSum(buf []byte) uint16 {
	var sum int32
	n := len(buf)
	if len(buf) % 2 != 0 {
		n--
	}
	for i := 0; i < n; i += 2 {
		sum += int32(buf[i]) << 8 + int32(buf[i+1])
	}
	if len(buf) % 2 != 0 {
		sum += int32(buf[n])
	}
	sum = (sum >> 16) + (sum & 0xffff)
	sum += (sum >> 16)
	var ans uint16 = uint16(^sum)
	return ans
}

func (cli *Client) dns(ch chan interface{}) {
	log.Print("in client dns")
	t1 := time.Now()
	_, err := net.LookupIP(cli.url)
	if err != nil {
		log.Print("dns failed: ", err)
		ch <- time.Duration(0)
	} else {
		ch <- time.Now().Sub(t1)
	}
}
