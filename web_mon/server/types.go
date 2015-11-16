package main

const SQL_CONN_TIMEOUT = 10		// 100s

var STOP, QUIT, HOST_UPDATE, WEB_UPDATE chan struct{}
func init() {
	STOP = make(chan struct{})		// used to control if master works
	QUIT = make(chan struct{})		// terminates the program
	//HOST_UPDATE = make(chan struct{})		// signal  host update
	WEB_UPDATE = make(chan struct{})		// signal web update
}

const SERVER_LISTEN_ADDR = ":7070"
const NODE_PORT = 7080	
const PHP_COMM_ADDRESS = ":7090"
