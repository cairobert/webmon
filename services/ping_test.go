package services 
import "testing"

var LocalAddr = "127.0.0.1"
var NTimeout = 3

func TestCheckSum(t *testing.T) {
	buf := []byte{0, 0, 0, 1}
	sum := 1
	ans := uint16(^sum)
	if s := CheckSum(buf); s != ans {
		t.Fatal("CheckSum of 0001 wrong")
	}
	buf = []byte{1, 1, 1, 1, 1}
	sum = 0x203
	ans = uint16(^sum)
	if CheckSum(buf) != ans {
		t.Fatal("CheckSum of 11111 wrong")
	}
}

func TestPing(t *testing.T) {
	if _, e := Ping(LocalAddr, NTimeout); e != nil {
		t.Fatal(e)
	}
}
