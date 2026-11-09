package main

import "fmt"

var verbose = false

func main() {
	for i := range 3 {
		fmt.Println(i)
	}
	switch {
	case verbose:
	}
	go func() {}()
	help()
}

func help() {
	fmt.Println("a string literal that is far longer than forty runes")
}
