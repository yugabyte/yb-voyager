package migrationutil

import "log"

func CheckError(err error, executedCommand string, possibleReason string, stop bool) {
	if err != nil {
		if executedCommand != "" {
			log.Printf("Error Command: %s\n", executedCommand)
		}

		if possibleReason != "" {
			log.Printf("%s\n", possibleReason)
		}
		if stop {
			log.Fatalf("%s \n", err)
		} else {
			log.Printf("%s \n", err)
		}
	}
}

func CheckErrorSimple(err error, printStatement string, stop bool) {
	if err != nil {
		if stop {
			log.Fatalf("%s: %s\n", printStatement, err)
		} else {
			log.Printf("%s: %s\n", printStatement, err)
		}
	}
}
