/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package utils

import (
	"bufio"
	"fmt"
	"time"
)

func Wait(c chan* int) {
	fmt.Print("\033[?25l") // Hide the cursor
	chars := [4]byte{'|', '/', '-', '\\'}
	var i = 0
	for true {
		i++
		select {
		case <-c:
			//fmt.Printf("\nGot Data on channel. Export Done\n")
			fmt.Println("\r")
			return
		default:
			fmt.Print("\b" + string(chars[i%4]))
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func Readline(r *bufio.Reader) (string, error) {
	var (isPrefix bool = true
		err error = nil
		line, ln []byte
	)
	for isPrefix && err == nil {
		line, isPrefix, err = r.ReadLine()
		ln = append(ln, line...)
	}
	return string(ln), err
}