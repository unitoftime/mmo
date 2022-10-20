package serdes

import (
	"fmt"
	"testing"
)

func TestChatFilter(t *testing.T) {
	fmt.Println(FilterChat("Hello World"))

	fmt.Println(FilterChat("^*(!@)$^!(*"))

	fmt.Println(FilterChat(`hello world!@#$%^&*() [{]}'";:<>,./+=?~-_,.`))
	fmt.Println(FilterChat(`☀hello world!@#$%^&*() [{]}'";:<>,./+=?~-_,.`))
}
