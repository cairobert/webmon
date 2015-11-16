//This package need to be re-factoried.
package services 
import (
		//"errors"
		"os"
		"net"
		"time"
	   )

const (
		icmpv4EchoRequest = 8
		icmpv4EchoReply = 0
		icmpv6EchoRequest = 128
		icmpv6EchoReply = 129
	  )

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

//Ping pings an address addr with timeout. It tries 4 times and returns the average time for response.
func Ping(addr string, timeout int) (retTime time.Duration, err error) {
	conn, err := net.Dial("ip4:icmp", addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	retTime, err = pinger(conn, timeout)
	return
}

//Send and read datagrams from conn
func pinger(conn net.Conn, timeout int) (retTime time.Duration, err error) {
	defer func() {
		var zero time.Time
		conn.SetDeadline(zero)
	}()

	pid := os.Getpid()
	var id1 = byte(pid & 0xff00 >> 8)
	var id2 = byte(pid & 0xff)
	len := 8
	var iFail = 0
	var ttlTime time.Duration = 0

	for i := 0; i < NTimes; i++ {
		var msg [512]byte
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
		conn.SetDeadline(t1.Add( time.Second * time.Duration(timeout) ))
		if _, err = conn.Write(msg[0:len]); err != nil {
			iFail++
			continue
		}
		if _, err = conn.Read(msg[0:]); err != nil {
			iFail++
			continue
		}
		t2 := time.Now()
		reply := msg[20:]	
		var dis = t2.Sub(t1)
		if reply[4] == id1 && reply[5] == id2 && reply[6] == 0 && reply[7] == byte(i+1) {
			ttlTime += dis
		} else {
			iFail++
		}
		if i != NTimes {
			time.Sleep(1e9)
		}
	}

	if iFail != NTimes {
		return ttlTime/NTimes, nil 
	} else {
		return 0, err
	}
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
