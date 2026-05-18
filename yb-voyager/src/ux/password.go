/*
Copyright (c) YugabyteDB, Inc.

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

package ux

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/term"
)

func AskPassword(destination string, user string, envVar string) (string, error) {
	if os.Getenv(envVar) != "" {
		return os.Getenv(envVar), nil
	}

	if user == "" {
		fmt.Printf("Password to connect to %s (In addition, you can also set the password using the environment variable '%s'): ",
			destination, envVar)
	} else {
		fmt.Printf("Password to connect to '%s' user of %s (In addition, you can also set the password using the environment variable '%s'): ",
			user, destination, envVar)
	}
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	fmt.Print("\n")
	return string(bytePassword), nil
}
