package main

import (
	"flag"
	"fmt"
	//"goweb"
)

// Command line options
var (
	PORT     = flag.Int("port", 8000, "the port to listen on")
	DATAROOT = "/Users/jared/projects/GoShockData"
)

func init() {}

func main() {
	flag.Parse()

	n, err := CreateNode("/Users/jared/ANL/Apr_Day_pf.fas", "test.json")
	if err != nil {
		fmt.Println("hells bells: " + err.String())
	}
	err = n.Save()
	if err != nil {
		fmt.Println("hells bells: " + err.String())
	}
	fmt.Println(n.ToJson())
	fmt.Println(n.Path())

	/*
	n, err = LoadNode("bf6d2f5b9611cb4ebe28d79f25cd65f4")
	if err != nil {
		fmt.Println("hells bells: " + err.String())
	}
	fmt.Println(n.ToJson())
	*/
	
	//goweb.MapRest("/node", new(NodeController))
	//goweb.ListenAndServe(":"+fmt.Sprintf("%d", *PORT))  
}
