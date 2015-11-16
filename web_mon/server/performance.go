package main
import (
		_"github.com/Go-SQL-Driver/MySQL"
		"snmp"
		//"database/sql"
		"log"
		"net"
		//"reflect"
		"strings"
		"time"
	   )

type Performance struct {
	Id			int				// server id
	CPU			float32
	MemoryTtl	int
	MemoryAvail	int
	SwapTtl		int
	SwapAvail	int
	Disks		[]Disk
	IfOctets	[]IfOctet
	Community	string
	Err			string
	timeout		int				// timeout
	conn		*net.UDPConn
	ses			*snmp.Session
	addr		*net.UDPAddr	// format: domain:port
}

type Disk struct {
	Path	string
	Total	int64		// MB
	Avail	int64		// MB
}

type IfOctet struct {
	IfDescr		string
	IfInOctet	int64		// MB
	IfOutOctet	int64		// MB
}

func NewPerformance(id int) *Performance {
	p := new(Performance)
	p.Id = id
	p.ses = snmp.NewSession()
	p.timeout = 15				// 15s timeout for default
	p.conn = nil
	return p
}

func (p *Performance) SetAddr(addr string) {
	dot := strings.Index(addr, ":")
	if dot == -1 || len(addr[dot+1:]) == 0 {
		addr = addr + ":161"
	}
	a, err := net.ResolveUDPAddr("udp", addr)
	if err == nil {
		p.addr = a
	} else {
		p.addr = nil
	}
}

func (p *Performance) SetTimeout(n int) {
	if n > 100 {
		return
	}
	p.timeout = n
}

func (p *Performance) SetCommunity(s string) {
	p.Community = s
}

func (p *Performance) Run() {
	log.Print("performance.run: ")
	if p.Community == "" {
		log.Print("No community set for the client")
		return
	}
	if p.conn == nil {
		conn, err := net.DialUDP("udp", nil, p.addr)
		if err != nil {
			p.Err = err.Error()
			return
		}
		p.conn = conn
		p.ses.SetConn(conn)
	}

	p.conn.SetDeadline(time.Now().Add(time.Duration(p.timeout) * time.Second))
	var t time.Time
	defer p.conn.SetDeadline(t)

	p.retrieveCpuAndMem()		// ignore the error, however, this is a bug
	p.retrieveDisk()	
	p.retrieveOctets()
}

func (p *Performance) retrieveCpuAndMem() error {
	m := new(snmp.Message)
	m.Version = 1
	m.Community = p.Community

	var done = true
	// retrieve cpu
	objs := [][]int{snmp.CPU_RAW_IDLE, snmp.CPU_RAW_USER, snmp.CPU_RAW_NICE, 
						snmp.CPU_RAW_SYSTEM, snmp.CPU_RAW_WAIT, []int{},
						snmp.MEM_TOTAL, snmp.MEM_AVAIL, []int{},
						snmp.SWAP_TOTAL, snmp.SWAP_AVAIL} 
	vals := make([]int64, 9)				// 9 items
	func() {
		j := -1
		for _, v := range objs {
			if len(v) == 0 {
				done = true
				continue
			}
			j++
			if !done {
				continue
			}
			m.RequestObjId = v
			err := p.ses.Get(m)
			if err != nil {
				done = false
			}
			if m.ErrIndex == 0 {
				if ret, ok := m.Value.(int64); ok {
					vals[j] = ret
					done = true
				}
			}
		}
	}()
	if !done {
		p.CPU = 0.0
		p.MemoryTtl = 0
		p.MemoryAvail = 0
		p.SwapTtl = 0
		p.SwapAvail = 0
	} else {
		ttl := int64(0)
		for _, v := range vals[0:5] {
			ttl += v
		}
		p.CPU =  1.0 - float32(vals[0]) / float32(ttl)		
		p.MemoryTtl = int(vals[5]) / 1000		// KB -> MB
		p.MemoryAvail = int(vals[6]) / 1000
		p.SwapTtl = int(vals[7])	/ 1000		// KB -> MB
		p.SwapAvail = int(vals[8])	/ 1000
	}
	return nil
}

func (p *Performance) retrieveDisk() error {
	m := new(snmp.Message)
	m.Version = 1
	m.Community = p.Community

	// retrieve device paths
	m.RequestObjId = snmp.DISK_DEVICE
	rets, err := p.ses.Walk(m)
	if err != nil {
		return err
	}
	// retrieve device mount point
	m.RequestObjId = snmp.DISK_PATH
	rets2, err := p.ses.Walk(m)
	if err != nil {
		return err
	}

	var paths []string
	var done = make([]int, len(rets))
	for i, v := range rets {
		var dev string
		switch v.(type) {
		case []byte:
			d := v.([]byte)
			dev = string(d)
		default:
			log.Fatal("Uanble to decode value: ", v)		// to expose error
		}
		if dev != "" && strings.HasPrefix(dev, "/dev/") {
			var path string
			switch rets2[i].(type) {
			case []byte:
				p := rets2[i].([]byte)
				path = string(p)
			default:
				log.Fatal("Uanble to decode value: ", v)		// to expose error
			}
			if path != "" {
				paths = append(paths, path)
				done[i] = 1
			}
		}
	}

	var objs = [][]int{snmp.DISK_TOTAL, snmp.DISK_AVAIL}
	var vals = make([][]int64, len(objs))		// each element of vals is []int, relating to the paths
	for i, v := range objs {
		m.RequestObjId = v
		vs, err := p.ses.Walk(m)		// walk the obj
		if err != nil {
			return err
		}
		var tmp = make([]int64, len(paths))
		if m.ErrIndex == 0 {
			var j = 0
			for i2 := range done {
				if done[i2] == 1 {			// corresponding values to /dev/sd*	
					if val, ok := vs[i2].(int64); ok {
						tmp[j] = val
					} else {
						tmp[j] = 0			// failed to transfer to int
					}
					j++
				}
			}
		}
		vals[i] = tmp
	}
	
	p.Disks = make([]Disk, len(paths))
	for i := range paths {
		p.Disks[i].Path = paths[i]
		p.Disks[i].Total = vals[0][i] / 1024		// file: KB -> MB
		p.Disks[i].Avail = vals[1][i] / 1024		// file: KB -> MB
	}
	return nil
}
		
// retrieve ifOctets except "lo"
func (p *Performance) retrieveOctets() error {
	m := new(snmp.Message)
	m.Version = 1
	m.Community = p.Community

	m.RequestObjId = snmp.IF_DESCR
	rets, err := p.ses.Walk(m)
	if err != nil {
		return err
	}
	var devs []string
	var lo int
	for i, v := range rets {
		var dev string
		switch v.(type) {
		case []byte:
			d := v.([]byte)
			dev = string(d)
		default:
			log.Fatal("Uanble to decode value: ", v)
		}
		if dev != "" {
			if dev == "lo" {
				lo = i
			} else {
				devs = append(devs, dev)
			}
		}
	}
	
	var objs = [][]int{snmp.IF_INOCTETS, snmp.IF_OUTOCTETS}
	var vals = make([][]int64, len(objs))
	for i, v := range objs {
		m.RequestObjId = v
		vs, err := p.ses.Walk(m)
		if err != nil {
			return err
		}
		var tmp = make([]int64, len(devs))
		if m.ErrIndex == 0 {
			var j = 0
			for i2, v2 := range vs {
				if i2 != lo {
					// doesn't check if j will be out of range here to expose potential error in advance
					/*						
					if j >= len(devs) {			
						break
					}
					*/
					if val, ok := v2.(int64); ok {
						tmp[j] = val
					} else {
						tmp[j] = 0
					}
					j++
				}
			}
		}
		vals[i] = tmp
	}
	p.IfOctets = make([]IfOctet, len(devs))
	for i, v := range devs {
		p.IfOctets[i].IfDescr = v
		p.IfOctets[i].IfInOctet = vals[0][i] / 1000000		// B -> MB
		p.IfOctets[i].IfOutOctet =  vals[1][i] / 1000000
	}
	return nil
}

func (p *Performance) Quit() {
	p.ses.Quit()
}
