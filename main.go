package main

import (
	"github.com/labstack/echo/v4"

	"github.com/woonglife62/woongkie-talkie/middleware"
	"github.com/woonglife62/woongkie-talkie/router"
)

func main() {
	e := echo.New()

	middleware.Middleware(e)

	e.Static("/", "/home/wclee/go/src/server") // echo root Path

	router.Router(e)

	e.Logger.Fatal(e.Start("192.168.219.151:8080"))
}
