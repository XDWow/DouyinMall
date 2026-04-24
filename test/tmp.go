package main

import "sync"

func main() {
	wg := sync.WaitGroup{}
	wg.Go()
}
