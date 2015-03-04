package main

import (
	_ "teameditor/routers"
	"github.com/astaxie/beego"
	"net/http"
	"teameditor/controllers"
)

func main() {
	go func() {
		http.HandleFunc("/echo", controllers.EchoServer)
		err := http.ListenAndServe(":18001", nil)
		if err != nil {
			panic("ListenAndServe: " + err.Error())
		}
	}()
	beego.Run()
}

