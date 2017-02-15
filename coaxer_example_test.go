package coaxer

import (
	"context"
	"fmt"
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

	result := promise.Result()

	if result.Error != nil {
		fmt.Println(result.Error)
		return
	}
	fmt.Println(result.Value)
}
