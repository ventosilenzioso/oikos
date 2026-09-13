package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("oikos: no command")
		os.Exit(1)
	}
	fmt.Println("oikos: unknown command:", os.Args[1])
	os.Exit(1)
}
