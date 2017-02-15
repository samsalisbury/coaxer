package coaxer

import (
	"context"
	"fmt"
	"log"
)

func Example() {

	makeHiString := func() (interface{}, error) {
		return "hi", nil
	}

	ctx := context.Background()

	c := NewCoaxer(func(c *Coaxer) {
		c.Attempts = 10
	})

	promise := c.Coax(ctx, makeHiString, "hi string")

	value, err := promise.Result()

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(value)
}
