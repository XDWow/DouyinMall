package main

import "fmt"

func main() {
	const workNum = 1000
	done := make(chan struct{})
	token := make(chan int, 1)
	token <- 0

	for i := 1; i <= workNum; i++ {
		go func(id int) {
			for {
				v := <-token
				if v > 1000 {
					close(done)
					return
				}
				c := v % 26
				fmt.Println(id, 'a'+rune(c))
				token <- v + 1
			}
		}(i)
	}

	<-done
}
