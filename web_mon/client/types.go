package main

//const WEB_ADDR = ":7080"
const SERVER_ADDR = "localhost:7070"
const NODE_ADDR = ":7080"
var UPDATE chan struct{}

func init() {
	UPDATE = make(chan struct{})
}
