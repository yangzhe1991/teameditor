package controllers

import (
	"log"
)

func handle(json string){
	log.Println("handling:"+json)
	var res string
	res=json
	h.broadcast<-[]byte(res)
}
