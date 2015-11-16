package snmp
import (
		"encoding/asn1"
		"errors"
		"fmt"
		"log"
		"net"
		"strconv"
		"strings"
	   )

const (
		Sequence			= 	0x30
		GetRequest			=	0xa0
		GetNextRequest		= 	0xa1
		GetResponse			= 	0xa2
		COUNTER				= 	0x41
		INTEGER				= 	0x02
	  )

var	 (
		DefaultPort	=	161
	  )

type Session struct {
	Addr		*net.UDPAddr
	conn		*net.UDPConn
	CurReqId	int
}

type Message struct {
	Version 		int
	Community		string
	RequestType		int
	RequestId		int
	ErrStatus		int
	ErrIndex		int
	RequestObjId	asn1.ObjectIdentifier
	Value			interface{}
}

type Varbind struct {
	ObjId		asn1.ObjectIdentifier	// need to be transformed to asn1.ObjectIdentifier. Underlying type is []int
	Value		asn1.RawValue			// It may be Null for or any other type
}

// SNMPv1
type packet1 struct {
	Version		int
	Community	[]byte
	Data		asn1.RawValue
}

type PDUv1 struct {
	RequestId		int
	ErrStatus		int
	ErrIndex		int
	VarbindList		[]Varbind
}

func NewSession() (ses *Session) {
	ses = new(Session)
	ses.CurReqId = 1
	return
}

func (ses *Session) SetAddr(addr string) (err error) {
	dot := strings.Index(addr, ":")
	if dot == -1 {
		addr = fmt.Sprintf("%s:%d", addr, DefaultPort)
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return
	}
	ses.Addr = udpAddr
	return
}

func (ses *Session) SetConn(conn *net.UDPConn) {
	ses.conn = conn
}
func (ses *Session) Get(m *Message) (err error) {
	log.Print("snmp: IN GET")
	defer func() {
		if err != nil {
			log.Print("Get: err: ", err)
		}
	}()

	if m == nil {	
		err = errors.New("Message is nil")
		return
	}
	m.RequestType = GetRequest
	err = ses.send(m)
	return
}


func newPacket(m *Message) (p *packet1, err error) {
	p = new(packet1)
	p.Version = m.Version
	p.Community = []byte(m.Community)
	bind := Varbind{ObjId: m.RequestObjId, Value: Null()}
	pdu := PDUv1{
		RequestId:		m.RequestId,
		VarbindList:	[]Varbind{bind},
	}	
	data, err := asn1.Marshal(pdu)
	if err != nil {
		fmt.Println("Marshal in newPacket: ", err)
		return nil, err
	}
	p.Data = RawValue(data, m.RequestType)
	return p, nil
}

// send the message
func (ses *Session) send(m *Message) (err error) {
	if ses.Addr == nil && ses.conn == nil {
		return errors.New("addr empty")
	}
	m.reset()
	m.RequestId = ses.CurReqId
	ses.CurReqId++
	p, err := newPacket(m)
	if err != nil {
		return
	}
	data, err := asn1.Marshal(*p)
	if err != nil {
		return errors.New("asn1.Marsha packet: " + err.Error())
	}	
	if ses.conn == nil {
		if ses.Addr == nil {
			log.Fatal("No address or connection assigned")
		}
		ses.conn, err = net.DialUDP("udp", nil, ses.Addr)
		if err != nil {
			ses.conn = nil
			return 
		}
	}
	conn := ses.conn
	_, err = conn.Write(data)
	if err != nil {
		return
	}
	data = make([]byte, 1500)		// MTU for ethernet		
	n, err := conn.Read(data)
	if err != nil {
		return
	}
	err = decodeMessage(m, data[:n])
	return 
}

func decodeMessage(m *Message, data []byte) (err error) {
	pckt := new(packet1)
	_, err = asn1.Unmarshal(data, pckt)
	if err != nil {
		return err
	}
	if pckt.Data.FullBytes[0] != GetResponse {
		return errors.New("wrong return pdu type")
	}
	pckt.Data.FullBytes[0] = Sequence		// change GetResponse to Sequence to marshal
	pdu := new(PDUv1)
	_, err = asn1.Unmarshal(pckt.Data.FullBytes, pdu)
	if err != nil {
		return err
	}
	var em interface{}
	if len(pdu.VarbindList) == 0 {
		log.Fatal("wrong return type")
	}
	value := pdu.VarbindList[0].Value
	if value.FullBytes[0] == byte(COUNTER) {	
		value.FullBytes[0] = INTEGER			// change type counter to type int
	} else if value.Class != 0 {				// non-universal type other than counter
		return errors.New("Unsupported type")
	}
	_, err = asn1.Unmarshal(value.FullBytes, &em)
	if err != nil {
		return
	}

	m.ErrStatus = pdu.ErrStatus
	m.ErrIndex = pdu.ErrIndex
	m.RequestObjId = pdu.VarbindList[0].ObjId
	m.Value = em
	return
}

func (ses *Session) GetNext(m *Message) (err error) {
	if m == nil {
		return errors.New("Message is nil")
	}
	m.RequestType = GetNextRequest
	return ses.send(m)
}

func StrToObjId(oid string) (objId []int, err error) {
	ids := strings.Split(oid, ".")
	if len(ids) < 3 {
		err = errors.New("wrong object identifier format")
		return
	}
	objId = make([]int, len(ids))
	for i, v := range ids {
		num, e := strconv.ParseInt(v, 10, 0)			// e.g.: "1" -> 1
		if e != nil {
			err = errors.New("wrong object identifier format")
			return
		}
		objId[i] = int(num)
	}
	return
}

func (ses *Session) Walk(m *Message) (values []interface{}, err error) {
	root := m.RequestObjId
	lroot := len(root)
	if lroot < 3 {
		return values, errors.New("wrong objectIdentifier")
	}
	for {
		err = ses.GetNext(m)
		if err != nil {
			return
		}
		if objId := m.RequestObjId; isChild(root, objId) {	// original objId is not a leaf
			values = append(values, m.Value)				// a child of oid
		} else {														// objId is a leaf
			return
		}	
	}
	return
}

func isChild(obj1, obj2 asn1.ObjectIdentifier) bool {
	if len(obj1) > len(obj2) {
		return false
	}
	var bChild = true
	for i := range obj1 {
		if obj1[i] != obj2[i] {
			bChild = false
		}
	}
	return bChild
}		
	
func (ses *Session) Quit() {
	ses.conn.Close()
}

// clear error status of message
func (m *Message) reset() {
	m.ErrStatus = 0
	m.ErrIndex = 0
}

// data is already been marshaled. For useage of Bytes and FullBytes, please read the source code of package asn1
func RawValue(data []byte, typ int) asn1.RawValue {
	typ = typ & 0xff				// Get the value of last 8 bits
// Acutally cls, tag, isPound can be omitted here.		
	cls := typ >> 6				// Get the value of bit 7 and 8
	//isPound := bool((typ >> 5) & 0x1)		// Get the value of bit 6
	var isPound bool
	if (typ >> 5 & 0x1) != 0 {
		isPound = true
	}
	tag := typ & 0xf				// Get the value of last 4 bits
	full := data
	full[0] = byte(typ)				// for length, it is already been stored in data
	return asn1.RawValue{
			Class:			cls,
			Tag:			tag,
			IsCompound:		isPound,
			Bytes:			[]byte{},
			FullBytes:		full,
	}
}

func Null() asn1.RawValue {
	return asn1.RawValue{
			Class:		0, 
			Tag: 		5, 
			IsCompound:	false, 
			Bytes:		[]byte{}, 
			FullBytes:	[]byte{0x5, 0x0},
	}
}

