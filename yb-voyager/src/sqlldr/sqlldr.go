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
package sqlldr

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func CreateSqlldrDir(exportDir string) error {
	if _, err := os.Stat(fmt.Sprintf("%s/sqlldr", exportDir)); os.IsNotExist(err) {
		err = os.Mkdir(fmt.Sprintf("%s/sqlldr", exportDir), 0755)
		if err != nil {
			return fmt.Errorf("create sqlldr directory %q: %w", fmt.Sprintf("%s/sqlldr", exportDir), err)
		}
	}
	return nil
}

func CreateSqlldrControlFile(exportDir string, tableName string, sqlldrConfig string, fileName string) (sqlldrControlFilePath string, err error) {
	sqlldrControlFileName := fmt.Sprintf("%s-%s.ctl", tableName, fileName)
	sqlldrControlFilePath = fmt.Sprintf("%s/sqlldr/%s", exportDir, sqlldrControlFileName)
	sqlldrControlFile, err := os.Create(sqlldrControlFilePath)
	if err != nil {
		return "", fmt.Errorf("create sqlldr control file %q: %w", sqlldrControlFilePath, err)
	}
	defer sqlldrControlFile.Close()
	_, err = sqlldrControlFile.WriteString(sqlldrConfig)
	if err != nil {
		return "", fmt.Errorf("write sqlldr control file %q: %w", sqlldrControlFilePath, err)
	}
	return sqlldrControlFilePath, nil
}

func CreateSqlldrLogFile(exportDir string, tableName string) (sqlldrLogFilePath string, sqlldrLogFile *os.File, err error) {
	sqlldrLogFileName := fmt.Sprintf("%s.log", tableName)
	sqlldrLogFilePath = fmt.Sprintf("%s/sqlldr/%s", exportDir, sqlldrLogFileName)
	sqlldrLogFile, err = os.Create(sqlldrLogFilePath)
	if err != nil {
		return "", nil, fmt.Errorf("create sqlldr log file %q: %w", sqlldrLogFilePath, err)
	}
	return sqlldrLogFilePath, sqlldrLogFile, nil
}

func RunSqlldr(sqlldrArgs string, password string) (outbufStr string, errbufStr string, err error) {
	var outbuf bytes.Buffer
	var errbuf bytes.Buffer
	cmd := exec.Command("sqlldr", sqlldrArgs)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return "", "", fmt.Errorf("get stdin pipe for sqlldr: %w", err)
	}
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err = cmd.Start()
	if err != nil {
		return "", "", fmt.Errorf("start sqlldr: %w", err)
	}
	_, err = stdinPipe.Write([]byte(fmt.Sprintf("%s\n", password)))
	if err != nil {
		return "", "", fmt.Errorf("write password to sqlldr: %w", err)
	}
	err = stdinPipe.Close()
	if err != nil {
		return "", "", fmt.Errorf("close stdin pipe for sqlldr: %w", err)
	}
	err = cmd.Wait()

	return outbuf.String(), errbuf.String(), err
}
