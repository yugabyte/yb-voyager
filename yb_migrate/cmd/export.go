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
package cmd

import (
	"fmt"
	"time"
	"github.com/spf13/cobra"
)

type Source struct {
	sourceDBType string
	sourceHost string
	sourcePort string
}

type Destination struct {
	sourceDBType string
	sourceHost string
	sourcePort string
}

type Format interface {
	PrintFormat(cnt int)
}

func (s *Source) PrintFormat(cnt int) {
	fmt.Printf("On type Source\n")
}

func (s *Destination) PrintFormat(cnt int) {
	fmt.Printf("On type Destination\n")
}

//func Wait(c chan* int) {
//	fmt.Print("\033[?25l") // Hide the cursor
//	chars := [4]byte{'|', '/', '-', '\\'}
//	var i = 0
//	for true {
//		i++
//		select {
//		case <-c:
//			fmt.Printf("\nGot Data on channel. Export Done\n")
//			return
//		default:
//			fmt.Print("\b" + string(chars[i%4]))
//			time.Sleep(100 * time.Millisecond)
//		}
//	}
//}

func JustPrintStringForNTimesAndSleepInBetween(s string, secInterval time.Duration, numTimes int, c chan* int) {
	for i := 0; i < 5; i++ {
		// fmt.Printf(s)
		time.Sleep(secInterval * time.Second)
	}
	l :=0
	c <- &l
}

var sourceStruct Source
//var sourceDBType string
//var sourceHost string
//var sourcePort string
var sourceUser string
var sourceDBPasswd string
var sourceDatabase string
var sourceSchema string
var sourceSSLCert string

// exportCmd represents the export command
var exportCmd = &cobra.Command {
	Use:   "export",
	Short: "Export has various sub-commands to extract schema, data and generate migration report",
	Long: `Export has various sub-commands to extract schema, data and generate migration report.
`,

	Run: func(cmd *cobra.Command, args []string) {
		//fmt.Printf("export called with command use = %s and args[0] = %s\n", cmd.Use, args[0])
		//exportGenerateReportCmd.Run(cmd, args)
		//exportSchemaCmd.Run(cmd, args)
		//exportDataCmd.Run(cmd, args)
		//time.Sleep(3 * time.Second)
		var s Source
		s.sourceDBType = sourceStruct.sourceDBType
		fmt.Printf("export called with source data type = %s\n", s.sourceDBType)
		s.PrintFormat(1)
		var d Destination
		d.PrintFormat(10)
		var f Format = &s
		f.PrintFormat(0)
		f = &d
		f.PrintFormat(0)
		ci := make(chan *int)
		fmt.Printf("Starting to Export...")
		go JustPrintStringForNTimesAndSleepInBetween("exporting...", 1, 20, ci)
		fmt.Printf("Started wait for export to finish\n\n")
		Wait(ci)
		var timeZone = map[string]int{
			"UTC":  60,
			"EST":  70,
			"CST":  80,
			"MST":  90,
			"PST":  90,
		}
		x := len(timeZone)
		fmt.Printf("%d\n", x)
		fmt.Printf("%d\n", timeZone["UTC"])
		timeZone["NEW"] = 100
		fmt.Printf("For NEW map value = %d\n", timeZone["NEW"])
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.PersistentFlags().StringVar(&sourceStruct.sourceDBType, "source-db-type", "",
		"source database type (Oracle/PostgreSQL/MySQL)")
	exportCmd.PersistentFlags().StringVar(&sourceStruct.sourceHost, "source-host", "localhost",
		"The host on which the source database is running")
	// TODO How to change defaults with the db type
	exportCmd.PersistentFlags().StringVar(&sourceStruct.sourcePort, "source-port", "",
		"The port on which the source database is running")
	exportCmd.PersistentFlags().StringVar(&sourceUser, "source-user", "",
		"The user with which the connection will be made to the source database")
	// TODO All sensitive parameters can be taken from the environment variable
	exportCmd.PersistentFlags().StringVar(&sourceDBPasswd, "source-db-password", "",
		"The user with which the connection will be made to the source database")
	exportCmd.PersistentFlags().StringVar(&sourceDatabase, "source-database", "",
		"The source database which needs to be migrated to YugabyteDB")
	exportCmd.PersistentFlags().StringVar(&sourceSchema, "source-schema", "",
		"The source schema which needs to be migrated to YugabyteDB")
	// TODO SSL related more args will come. Explore them later.
	exportCmd.PersistentFlags().StringVar(&sourceSSLCert, "source-ssl-cert", "",
		"source database type (Oracle/PostgreSQL/MySQL")


}
