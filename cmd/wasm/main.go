package main

import (
	"syscall/js"
)

func printMessage(i []js.Value) {
	js.Global().Set("output", js.ValueOf(i[0].Int()+i[1].Int()))
	println(js.ValueOf(i[0].Int() + i[1].Int()).String())
}

func main() {
	c := make(chan struct{}, 0)

	println("Hello World")
	js.Global().Set("printMessage", js.FuncOf(printMessage))

	<-c
}
