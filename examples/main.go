package main

import (
	"github.com/rwiteshbera/rapidgo"
)

func main() {
	rapidgo.LoadEnv()

	router := rapidgo.New()
	// 🔹 Static Routes
	router.Get("/user", func(c *rapidgo.Context) {
		c.JSON(200, map[string]string{"message": "/user"})
	})
	router.Get("/user/id", func(c *rapidgo.Context) {
		c.JSON(200, map[string]string{"message": "/user/id"})
	})
	router.Get("/user/:id", func(c *rapidgo.Context) {
		id := c.Param("id")
		c.JSON(200, map[string]string{"message": "/user/:id", "id": id})
	})
	router.Get("/user/:id/accounts", func(c *rapidgo.Context) {
		id := c.Param("id")
		c.JSON(200, map[string]string{"message": "/user/:id/accounts", "id": id})
	})
	router.Get("/user/:id/accounts/value", func(c *rapidgo.Context) {
		id := c.Param("id")
		c.JSON(200, map[string]string{"message": "/user/:id/accounts/value", "id": id})
	})
	router.Get("/user/:id/accounts/:value", func(c *rapidgo.Context) {
		id := c.Param("id")
		value := c.Param("value")
		c.JSON(200, map[string]string{"message": "/user/:id/accounts/:value", "id": id, "value": value})
	})

	// Start server
	router.ListenGracefully()
}
