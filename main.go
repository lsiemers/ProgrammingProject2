package main

import (
	"directoryProgram/filesystem"
)

func main() {
	filesystem.Initialize()
	filesystem.Open("open", "test.txt", 1)
	filesystem.Open("open", "Lukas.png", 1)
	filesystem.Open("read", "test.txt", 1)
	filesystem.Open("write", "test.txt", 1)
	filesystem.Open("append", "test.txt", 1)


}
