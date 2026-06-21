package main

import (
	"ginchat/router"
)

func main() {
	r := router.Router()
	r.Run() // listens on 0.0.0.0:8080 by default
}
