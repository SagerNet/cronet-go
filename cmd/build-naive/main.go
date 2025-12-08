package main

import "log"

func main() {
	err := mainCommand.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
