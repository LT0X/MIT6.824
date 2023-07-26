package main

import "fmt"

type xx struct {
	Name string
	Age  int
}

func main() {

	x := make(map[string]bool)
	x["ss"] = true
	fmt.Print("fjk")
	v := xx{
		Name: "sddsf",
		Age:  12,
	}
	fmt.Println(v)
	fmt.Printf("%+v\n", v)

}
