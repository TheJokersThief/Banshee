package example

import "fmt"

// Hello formats a string with a Hello, string greeting
func Hello(msg string) string {
	return fmt.Sprintf("Hello, %s", msg)
}
