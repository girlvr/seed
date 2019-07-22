package main

import (
	"encoding/json"
	"fmt"
	"github.com/yinhevr/seed"
	"log"
	"strings"

	"os"
)

func main() {
	args := os.Args
	dir, err := os.Getwd()
	if len(args) > 1 {
		err = nil
		dir = args[1]
	}
	if err != nil {
		log.Println("wd:", err)
		return
	}
	v := seed.NewVerify(dir)
	sfs := v.Check()
	file, err := os.OpenFile("check.json", os.O_CREATE|os.O_SYNC|os.O_RDWR, os.ModePerm)
	enc := json.NewEncoder(file)
	err = enc.Encode(sfs)
	if err != nil {
		log.Println("enc:", err)
		return
	}
	fmt.Println(strings.Join(v.FailedList(), " "))
}