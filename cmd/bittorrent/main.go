package main

import (
	"flag"
	"syscall"
)

func main() {
	flag.Parse()
	syscall.Exit(0)
}
