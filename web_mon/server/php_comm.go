package main
import (
		_"github.com/Go-SQL-Driver/MySQL"
		"database/sql"
		"fmt"
		"log"
		"net"
		"strings"
		"time"
	   )
var	(
		INTER_HOSTS = [2]string{"192.168.0.0/16", "127.0.0.1/24"}
	  )

// should be called in a go statement
func comm_php() {
	listener, err := net.Listen("tcp", PHP_COMM_ADDRESS)
	if err != nil {
		log.Print("error in comm_php(): ", err)
		return
	}
	running := true
	for running {
		select {
		case <- QUIT:
			running = false
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				log.Print("error in comm_php(): ", err)
				continue
			}
			log.Print(conn.RemoteAddr(), " connected to server.")
			
			if !isInternal(conn.RemoteAddr().String()) {
				conn.Close()
				continue
			}
			
			go serve_php_comm(conn)
		}
	}
}

func serve_php_comm(conn net.Conn) {
	defer conn.Close()
	var rets []byte
	for {
		ret := make([]byte, 1500)
		conn.SetDeadline(time.Now().Add(10 * time.Second))
		n, err := conn.Read(ret)
		if err != nil {
			break
		}
		rets = append(rets, ret[:n]...)
	}
	go parse_php_comm(rets)	// called in a go statement, free php connection
}

func isInternal(addr string) bool {
	dot := strings.Index(addr, ":")
	if dot != -1 {
		addr = addr[:dot]
	}
	ip := net.ParseIP(addr)
	for _, v := range INTER_HOSTS {
		_, ipNet, err := net.ParseCIDR(v)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

func parse_php_comm(data []byte) {
	log.Print("receive: ", string(data))
	contents := strings.Split(string(data), ";")
	head := strings.Fields(contents[0])
	if head[0] != "UPDATE" && len(head) < 2{
		return
	}
	switch head[1] {

/** REVISON *** /		
/*
	case "HOST":
		HOST_UPDATE <- struct{}{}
*/
	case "WEB":
		log.Print("PHP: UPDATE")		// UPDATE WEB; INSERT URL FREQ
		update_node_urls(contents[1:])
	default:
		log.Print("Unknown command from the php end")
	}
}

func update_node_urls(contents []string) {
	sends := strings.Join(contents, ";")
	db, err := sql.Open("mysql", db_user)
	if err != nil {
		log.Print("error in updates: ", err)
		return
	}
	defer db.Close()
	rows, err := db.Query("SELECT ip FROM isp")
	if err != nil {
		log.Print("error in updates: ", err)
		return
	}
	for rows.Next() {
		var ip string
		err = rows.Scan(&ip)
		if err != nil {
			continue
		}
		go func() {
			nodeAddr := fmt.Sprintf("%s:%d", ip, NODE_PORT)
			conn, err := net.Dial("tcp", nodeAddr)
			if err != nil {
				log.Print("error in update_urls: ", err, "nodeAddr: ", nodeAddr)
				return
			}
			conn.SetDeadline(time.Now().Add(10 * time.Second))
			conn.Write([]byte(sends))
			conn.Close()
		}()
	}
}
