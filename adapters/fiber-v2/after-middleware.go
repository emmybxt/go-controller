package fiberv2adapter

import "github.com/gofiber/fiber/v2"

type continuation struct{ entered bool }

func withAfter(handler fiber.Handler, middleware []fiber.Handler) []fiber.Handler {
	if len(middleware) == 0 {
		return []fiber.Handler{handler}
	}
	key := new(int)
	wrapped := func(c *fiber.Ctx) error {
		previous := c.Locals(key)
		state := &continuation{}
		c.Locals(key, state)
		defer c.Locals(key, previous)
		err := handler(c)
		if err != nil || state.entered {
			return err
		}
		return c.Next()
	}
	// Detect a handler's explicit Next call so downstream middleware runs once.
	gate := func(c *fiber.Ctx) error {
		c.Locals(key).(*continuation).entered = true
		return c.Next()
	}
	handlers := append([]fiber.Handler{wrapped, gate}, middleware...)
	// End this route even when the last after middleware calls Next.
	return append(handlers, func(*fiber.Ctx) error { return nil })
}
