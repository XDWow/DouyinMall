package main

import (
	searchv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/search/v1/searchservice"
	"log"
)

func main() {
	svr := searchv1.NewServer(new(SearchServiceImpl))

	err := svr.Run()

	if err != nil {
		log.Println(err.Error())
	}
}
