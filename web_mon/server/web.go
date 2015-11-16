package main
import (
		_"github.com/Go-SQL-Driver/MySQL"
		"database/sql"
		"encoding/json"
		"fmt"
		"log"
		"net"
		"strings"
		"time"
	   )

// typically called in a go statement
func serveWeb() {
	listener, err := net.Listen("tcp", SERVER_LISTEN_ADDR)
	if err != nil {
		log.Fatal(err)
	}
	db, err := sql.Open("mysql", db_user)
	if err != nil {
		log.Fatal("sql.Open(): ", err)
	}
	defer db.Close()
	stmt, err := db.Prepare("SELECT id FROM isp WHERE ip=?")
   	if err != nil {
		log.Fatal("db.Prepare(): ", err)
	}
	running := true
	go comm_php()
	for running {
		select {
		case <- QUIT:
			running = false
			break
		default:
			conn, err := listener.Accept()		// new node connected
			if err != nil {
				log.Print("err in accepting: ", err)
				continue
			}
			go serveConn(conn, stmt)
		}
	}
}

// called in a go statement
func serveConn(conn net.Conn, stmt *sql.Stmt) {
	defer conn.Close()
	log.Print(conn.RemoteAddr(), " connected to server...")
	ret := make([]byte, 1500)
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	n, err := conn.Read(ret)
	if err != nil {
		log.Print("error in reading: ", err)
		return
	}
	var cmds []interface{}
	err = json.Unmarshal(ret[:n], &cmds)
	if err != nil {
		log.Print("Fail to parse results")
		return
	}
	if _, ok := cmds[0].(string); !ok {
		log.Print("error in serveConn, data format that clients send is wrong")
		return
	}
	cmd := cmds[0].(string)
	switch cmd {
	case "INIT":
		initNode(conn, stmt) 
	case "MONITOR":
		parseResults(cmds)
	default:
		log.Print("Unknown command: ", cmds)
	}
}

func initNode(conn net.Conn, stmt *sql.Stmt) {
	log.Print("Init client: ", conn.RemoteAddr())
	address := conn.RemoteAddr().String()
	dot := strings.Index(address, ":")
	addr := address[0:dot]
	var id int
	err := stmt.QueryRow(addr).Scan(&id)		// if no such id, QueryRow return ErrNoRows
	if err != nil {
		log.Print("stmt.QueryRow(): ", err)
		return
	}
	data, err := json.Marshal(id)
	if err != nil {
		log.Print("error in initNode: ", err)
		return
	}
	conn.Write(data)
	go initNodeUrls(addr)		// send address lists to this node
}

func parseResults(cmds []interface{}) {
	log.Print("In parseResults:", cmds)
	if len(cmds) < 4 {			// MONITOR id time ip;lossRate,rtt;dns	...
		log.Print("error in parseResults, wrong command format", cmds)
		return
	}
	idf, ok := cmds[1].(float64)
	if !ok {
		log.Print("error in parseResults: cannot convert ", cmds[1], " to int")
		return
	}
	ispid := int(idf)
	tf, ok := cmds[2].(float64)
	if !ok {
		log.Print("error in parseResults: cannot convert ", cmds[2], " to type int64")
		return
	}
	ti := int64(tf)
	t := time.Unix(ti, 0)
	cmds = cmds[3:]
	if len(cmds) % 4 != 0 {
		return
	}
	db, err := sql.Open("mysql", db_user)
	if err != nil {
		log.Fatal("error in parseResults: ", err)
		return
	}
	defer db.Close()
	queryStmt, err := db.Prepare("SELECT id FROM web WHERE domain=?")
	if err != nil {
		log.Fatal("error in parseResults: ", err)
	}
	stmt, err := db.Prepare("INSERT INTO web_history(time, ispid, webid, http_time," +
						"ping_rtt, ping_loss_rate, dns_time) VALUES(?,?,?,?,?,?,?) ")
   	if err != nil {
		log.Fatal("db.Prepare(): ", err)
	}
	existStmt, err := db.Prepare("SELECT id FROM web_info WHERE webid = ? AND ispid = ?")
	if err != nil {
		log.Fatal("db.Prepare(): ", err)
	}
	insStmt, err := db.Prepare("INSERT INTO web_info(time, ispid, webid, http_time," +
						"ping_rtt, ping_loss_rate, dns_time) VALUES(?,?,?,?,?,?,?) ")

	if err != nil {
		log.Fatal("db.Prepare(): ", err)
	}
	
	updateStmt, err := db.Prepare("UPDATE web_info SET http_time = ?, ping_rtt = ?, ping_loss_rate = ?, dns_time = ?, time = ? WHERE id = ?")
	if err != nil {
		log.Fatal("db.Prepare(): ", err)
	}

	for i := 0; i < len(cmds); i += 4 {
		if url, lossRate, rtt, dns, http, ok := isDataValid(cmds[i:i+4]); ok {
			var webid int
			rows, err := queryStmt.Query(url)
			for rows.Next() {
				err = rows.Scan(&webid)
				if err != nil {
					log.Print("error in parseResults, rows.Scan(): ", err)
					continue
				}
			/*****
			err := queryStmt.QueryRow(url).Scan(&webid)
			if err != nil {
				log.Print("error in parseResults: ", err)
				continue
			}
			*****/
				_, err = stmt.Exec(t, ispid, webid, http, rtt, lossRate, dns)
				if err != nil {
					log.Print("error in parseResults: sql error: ", err)
				} else {
					log.Print("successfully insert a record.")
				}
				var curWebId int
				err = existStmt.QueryRow(webid, ispid).Scan(&curWebId)
				if err != nil {
					_, err = insStmt.Exec(t, ispid, webid, http, rtt, lossRate, dns)
					if err != nil {
						log.Print("error occured while inserting records to web_info", err)
					}
				} else {
					_, err = updateStmt.Exec(http, rtt, lossRate, dns, t, webid)
					if err != nil {
						log.Print("error occured while updating records in web_info", err)
					}
				}	
			}
		}
	}
}

func isDataValid(cmds []interface{}) (url string, lossRate float64, rtt, dns, http int64, ok bool) {
	url, ok = cmds[0].(string)
	if !ok {
		log.Print("error in parseResults: cannot convert ", cmds[0], " to type string")
		return
	}
	ping, ok := cmds[1].([]interface{})
	if !ok {
		log.Print("error in parseResults: cannot convert ", cmds[1], " to []interface{}")
		return
	}
	lossRate, ok = ping[0].(float64)
	if !ok {
		log.Print("error in parseResults: cannot convert ", ping[0], " to float64")
		return
	}
	rttf, ok := ping[1].(float64)
	if !ok {
		log.Print("error in parseResults: cannot convert time.Duration", ping[1], " to float64")
		return
	}
	rtt = int64(rttf)/1000		// units: microseconds
	dnsf, ok := cmds[2].(float64)
	if !ok {
		log.Print("error in parseResults: cannot convert ", cmds[2], " to time.Duration")
		return
	}
	dns = int64(dnsf)/1000		// units: microseconds
	httpf, ok := cmds[3].(float64)
	if !ok {
		log.Print("error in parseResults: cannot convert ", cmds[3], "to time.Duration")
		return
	}
	http = int64(httpf)/1000		// units: microseconds
	return
}

// send urls to node specified by addr
func initNodeUrls(addr string) { 
	dot := strings.Index(addr, ":")
	if dot == -1 {
		addr = fmt.Sprintf("%s:%d", addr, NODE_PORT)
	}
	db, err := sql.Open("mysql", db_user)
	if err != nil {
		log.Print("sql.Open(): ", err)
		return
	}
	defer db.Close()
	rows, err := db.Query("SELECT domain, frequent FROM web")
	if err != nil {
		log.Print("error in initNodeUrls: ", err)
		return
	}
	var urls []string
	var freq []int
	for rows.Next() {
		var url string
		var i int
		err = rows.Scan(&url, &i)
		if err != nil {
			continue
		}
		urls = append(urls, url)
		freq = append(freq, i)
	}
	sends := "INIT;"
	for i, url := range urls {
		sends += fmt.Sprintf("%s %d;", url, freq[i])
	}
	if sends == "INIT;" {
		log.Print("no urls to send")
		return
	}
	sends = sends[:len(sends)-1]
	time.Sleep(1e9)		// prevent localaddr
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Print("cannot connect to client ", err)
		return
	}
	conn.Write([]byte(sends))
	conn.Close()
		
}

/*
// if isSame is true, apply methods[0] to all urls
func updateNode(addr string, urls, methods []string, freq []int, isSame bool) {
	if len(methods) == 0 {
		log.Print("error in updateNode(): methods length cannot be zero")
		return
	}
	sends := ""
	for i := range urls {
		if isSame {
			sends += fmt.Sprintf("%s %s %d;", methods[0], urls[i], freq[i])
		} else {
			sends += fmt.Sprintf("%s %s %d;", methods[i], urls[i], freq[i])
		}
	}
	if sends != "" {
		sends = sends[0:len(sends)-1]
	}

	dot := strings.Index(addr, ":")
	if dot == -1 {
		addr = fmt.Sprintf("%s:%d", addr, NODE_PORT)
	} else {
		addr = fmt.Sprintf("%s:%d", addr[:dot], NODE_PORT)
	}
	time.Sleep(1e9)		// prevent localaddr
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Print("error in initNodeUrls: ", err)
		return
	}	
	log.Print("send urls: ", sends, " to ", addr)
	conn.Write([]byte(sends))
	conn.Close()
}
*/
