package main

import (
	"fmt"
	"github.com/fredyk/westack-go/westack"
	"github.com/gofiber/fiber/v2"
	"log"
)

func main() {

	app := westack.New(westack.WeStackOptions{
		Debug:        false,
		RestApiRoot:  "/api/v1",
		Port:         8023,
		JwtSecretKey: []byte(""),
	})

	app.Boot(ServerBoot)

	// For the ones pending to be done
	app.Server.Get("/*", func(c *fiber.Ctx) error {
		log.Println("GET: " + c.Path())
		return c.Status(404).JSON(fiber.Map{"error": fiber.Map{"status": 404, "message": fmt.Sprintf("Unknown method %v %v", c.Method(), c.Path())}})
	})
	app.Server.Post("/*", func(c *fiber.Ctx) error {
		log.Println("POST: " + c.Path())
		return c.Status(404).JSON(fiber.Map{"error": fiber.Map{"status": 404, "message": fmt.Sprintf("Unknown method %v %v", c.Method(), c.Path())}})
	})

	log.Fatal(app.Start(fmt.Sprintf(":%v", app.Port)))

}
