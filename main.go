/*
 * Copyright (c) 2008-2021, Hazelcast, Inc. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License")
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package main

import (
	"os"

	"github.com/hazelcast/hazelcast-commandline-client/runner"
)

const (
	exitOK    = 0
	exitError = 1
)

func main() {
	programArgs := os.Args[1:]
	config, err := runner.CLC(programArgs, os.Stdin, os.Stdout, os.Stderr)
	defer func() {
		if config != nil {
			config.LogFile.Close()
		}
	}()
	if err == nil {
		return
	}
	errStr := runner.HandleError(err)
	config.Logger.Println(errStr)
	os.Exit(exitError)
}
