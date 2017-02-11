package coaxer

import (
	"context"
	"fmt"
)

func Example() {

	manifest := func() (interface{}, error) {
		return "Hi", nil
	}

	c := NewCoaxer(manifest, func(c *Coaxer) {
		c.Attempts = 10
	})

	context := context.Background()

	result := <-c.Coax(context)

	if result.Error != nil {
		fmt.Println(result.Error)
		return
	}
	fmt.Println(result.Value)
}
