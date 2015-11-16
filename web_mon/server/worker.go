package main
import (
		_"github.com/Go-SQL-Driver/MySQL"
		"database/sql"
		"fmt"
		"log"
		"time"
	   )


// should be called in a go statement
func hostMonitor() {
	db, err := sql.Open("mysql", db_user)
	if err != nil {
		log.Fatal("sql.Open(): ", err)
	}
	defer db.Close()
/*		
	rows, err := db.Query("SELECT * FROM snmp_template")
	if err != nil {
		log.Fatal("db.Exec(): ", err)
	}
	run := func() {
		close(STOP)
		time.Sleep(5 * 1e9)		// terminates running masters 
		STOP = make(chan struct{})
		for rows.Next() {
			var id, t int
			err = rows.Scan(&id, &t)
			if err != nil {
				log.Print(err)
				continue
			}
			log.Print("start master ", id)
			go master(id, t)
		}
	}
	run()
*/
	go master(1)
	go master(3)
	go master(5)
	go master(10)
	ticker1 := time.NewTicker(time.Duration(1) * time.Minute)
	ticker3 := time.NewTicker(time.Duration(3) * time.Minute)
	ticker5 := time.NewTicker(time.Duration(5) * time.Minute)
	ticker10 := time.NewTicker(time.Duration(10) * time.Minute)
	for {
		select {
		case <- ticker1.C:
			go master(1)
		case <- ticker3.C:
			go master(3)
		case <- ticker5.C:
			go master(5)
		case <- ticker10.C:
			go master(10)
		case <- QUIT:		// program quits
			return			
		}
	}
}

// each master consits of many workers sharing same smnp template 
func master(tm int) {
	log.Print("hostMonitor master running, tm: ", tm)
	db, err := sql.Open("mysql", db_user)
	if err != nil {
		log.Print("sql.Open(): ", err)
		return
	}		
	defer db.Close()

	stmt, err := db.Prepare("SELECT id, domain, port, snmp_passwd FROM server" + 
										" WHERE frequent = ?")
	if err != nil {
		log.Print("db.Prepare(): ", err)
		return
	}
	rows, err := stmt.Query(tm)
	if err != nil {
		log.Print("stmt.Query(): ", err)
		return
	}
	done := make(chan *Performance, 50)
	var num int
	for rows.Next() {
		var domain, snmp_passwd string
		var id, port int
		log.Print("before scan")
		err = rows.Scan(&id, &domain, &port, &snmp_passwd)
		log.Print("after scan")
		log.Print("in master: ", id, port, domain, snmp_passwd)
		if err != nil {
			log.Print("rows.Scan(): ", err)
			continue
		}
		go worker(id, port, domain, snmp_passwd, done)
		num++
	}
	log.Print("after rows.Next()")
	for num > 0 {
		select {
		case p := <- done:
			go recordPerformance(p)
			num--
		case <- STOP:		// finish
			return
		}
	}
}

func worker(id, port int, domain, community string, done chan *Performance) {
	log.Print("worker: ", id, port, domain, community, done)
	p := NewPerformance(id)
	p.Community = community
	addr := fmt.Sprintf("%s:%d", domain, port)
	p.SetAddr(addr)
	p.Run()
	p.Quit()
	done <- p
}

func recordPerformance( p *Performance) {
	log.Print("recordPerformance: ", p)
	db, err := sql.Open("mysql", db_user)
	if err != nil {
		log.Fatal("sql.Open(): ", err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		log.Print("db.Begin(): ", err)
		return
	}
	now := time.Now()
	res, err := tx.Exec("INSERT INTO host_history(serverid, time, cpu, memTtl, memAvail, swapTtl, swapAvail) VALUES(?,?,?,?,?,?,?)", 
							p.Id, now, p.CPU, p.MemoryTtl, p.MemoryAvail, p.SwapTtl, p.SwapAvail) 
	if err != nil {
		tx.Rollback()
		log.Print("tx.Exec(): ", err)
		return
	}
	tx.Commit()
	lastId, err := res.LastInsertId()
	if err != nil {
		log.Print("res.LastInsertId(): ", err)
		return
	}

	exec := func(query string, args []interface{}) {
		if len(args) == 0 {
			return
		}
		if query[len(query)-1] == ',' {
			query = query[0:len(query)-1]
		}
		tx, _ := db.Begin()
		_, err := tx.Exec(query, args...)
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
			log.Print("error while execute: ", query, ", ", err)
		}
	}
	
	diskQuery := "INSERT INTO disk_history(host_infoid, path, total, avail, util) VALUES "
	var diskArgs []interface{}
	for _, d := range p.Disks {
		util := float64(d.Avail)/float64(d.Total)
		diskQuery += " (?,?,?,?,?),"
		diskArgs = append(diskArgs, lastId, d.Path, d.Total, d.Avail, util)
	}
	
	nicQuery := "INSERT INTO nic_history(host_infoid, name, inMB, outMB) VALUES "
	var nicArgs []interface{}
	for _, nic := range p.IfOctets {
		nicQuery += " (?,?,?,?)," 
		nicArgs = append(nicArgs, lastId, nic.IfDescr, nic.IfInOctet, nic.IfOutOctet)
	}
	exec(diskQuery, diskArgs)
	exec(nicQuery, nicArgs)
	log.Print("one record added to history: ", p)
	
	tx, _ = db.Begin()
	tx.Exec("DELETE FROM host_info WHERE serverid = ?", p.Id)
	tx.Commit()

	tx, _ = db.Begin()
	tx.Exec("INSERT INTO host_info(serverid, time, cpu, memTtl, memAvail, swapTtl, swapAvail) VALUES (?,?,?,?,?,?,?) ", 
			p.Id, time.Now(), p.CPU, p.MemoryTtl, p.MemoryAvail, p.SwapTtl, p.SwapAvail)
	tx.Commit()

	tx, _ = db.Begin()
	tx.Exec("DELETE FROM disk_info WHERE serverid = ?", p.Id)
	tx.Commit()
	diskUpdateQuery := "INSERT INTO disk_info(serverid, path, total, avail, util) VALUES(?,?,?,?,?) "


	for _, d := range p.Disks {
		util := float64(d.Avail)/float64(d.Total)
		tx, _ := db.Begin()
		_, err  = tx.Exec(diskUpdateQuery, p.Id, d.Path, d.Total, d.Avail, util)
		if err == nil {
			tx.Commit()
			log.Print("\n")
			log.Print("successfully inserted a record to disk_info")
		} else {
			tx.Rollback()
			log.Print("error in updating disk information: ", err)
		}
	}

	tx, _ = db.Begin()
	tx.Exec("DELETE FROM nic_info WHERE serverid = ?", p.Id)
	tx.Commit()
	nicUpdateQuery := "INSERT INTO nic_info(serverid, name, inMB, outMB) VALUES(?,?,?,?) "
	for _, nic := range p.IfOctets {
		tx, _ := db.Begin()
		_, err = tx.Exec(nicUpdateQuery, p.Id, nic.IfDescr, nic.IfInOctet, nic.IfOutOctet)
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
			log.Print("error in updating nic information: ", err)
		}
	}
	log.Print("updated host_info")	
}

func quit() {
	close(QUIT)
}
