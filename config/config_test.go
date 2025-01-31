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
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/alecthomas/assert"
	"github.com/charmbracelet/lipgloss"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	"github.com/hazelcast/hazelcast-go-client/logger"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	clcerrors "github.com/hazelcast/hazelcast-commandline-client/errors"
	"github.com/hazelcast/hazelcast-commandline-client/internal/tuiutil"
)

func TestDefaultConfig(t *testing.T) {
	conf := DefaultConfig()
	assert.Equal(t, DefaultClusterName, conf.Hazelcast.Cluster.Name)
	assert.Equal(t, logger.ErrorLevel, conf.Hazelcast.Logger.Level)
	assert.Equal(t, true, conf.Hazelcast.Cluster.Unisocket)
}

func TestReadConfig(t *testing.T) {
	const clientName = "test-client"
	tempDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Hazelcast.ClientName = clientName
	emptyPath := uniquePathWithContent(tempDir, nil)
	b, err := yaml.Marshal(cfg)
	require.Nil(t, err)
	customPath := uniquePathWithContent(tempDir, b)
	require.Nil(t, err)
	nonExistentPath := uniquePath(tempDir)
	testCases := []struct {
		name              string
		defaultConfigPath string
		path              string
		// workaround !!
		// not comparing hazelcast.Config objects since config != unmarshal(marshal(config)) because of nil map and slices
		expectedClientName string
		errMsg             string
	}{
		{
			name:   "Path: custom path, File: does not exist, Expect: error",
			path:   nonExistentPath,
			errMsg: fmt.Sprintf("configuration not found: %s", nonExistentPath),
		},
		{
			name:               "Path: custom path, File: is empty, Expect: default configuration",
			path:               emptyPath,
			expectedClientName: "",
		},
		{
			name:               "Path: custom path, File: custom config, Expect: custom configuration",
			path:               customPath,
			expectedClientName: clientName,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			err := readConfig(tc.path, &cfg, tc.defaultConfigPath)
			if tc.errMsg != "" {
				require.NotNil(t, err)
				require.Equal(t, err.Error(), tc.errMsg)
				return
			}
			require.Nil(t, err)
			cfg.Hazelcast.Clone()
			require.Equal(t, cfg.Hazelcast.ClientName, tc.expectedClientName)
		})
	}
}

func TestMergeFlagsWithConfig(t *testing.T) {
	tests := []struct {
		flags          GlobalFlagValues
		expectedConfig Config
		wantErr        bool
	}{
		{
			// Flags: none, Expect: Default config
			expectedConfig: DefaultConfig(),
		},
		{
			flags: GlobalFlagValues{
				Token: "test-token",
			},
			expectedConfig: func() Config {
				c := DefaultConfig()
				c.Hazelcast.Cluster.Cloud.Token = "test-token"
				c.Hazelcast.Cluster.Cloud.Enabled = true
				return c
			}(),
		},
		{
			flags: GlobalFlagValues{
				Cluster: "test-cluster",
			},
			expectedConfig: func() Config {
				c := DefaultConfig()
				c.Hazelcast.Cluster.Name = "test-cluster"
				return c
			}(),
		},
		{
			flags: GlobalFlagValues{
				Address: "localhost:8904,myserver:4343",
			},
			expectedConfig: func() Config {
				c := DefaultConfig()
				c.Hazelcast.Cluster.Network.Addresses = []string{"localhost:8904", "myserver:4343"}
				return c
			}(),
		},
		{
			flags: GlobalFlagValues{
				LogLevel: "unknownLogLevel",
			},
			wantErr: true,
		},
		{
			flags: GlobalFlagValues{
				LogLevel: "trace",
			},
			expectedConfig: func() Config {
				c := DefaultConfig()
				c.Hazelcast.Logger.Level = logger.TraceLevel
				return c
			}(),
		},
		{
			flags: GlobalFlagValues{
				LogLevel: "fatal",
				Verbose:  true,
			},
			expectedConfig: func() Config {
				c := DefaultConfig()
				c.Hazelcast.Logger.Level = logger.DebugLevel
				return c
			}(),
		},
		{
			flags: GlobalFlagValues{
				LogLevel: "trace",
				Verbose:  true,
			},
			expectedConfig: func() Config {
				c := DefaultConfig()
				c.Hazelcast.Logger.Level = logger.TraceLevel
				return c
			}(),
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("testcase-%d", i+1), func(t *testing.T) {
			c := DefaultConfig()
			err := mergeFlagsWithConfig(&tt.flags, &c)
			isErr := err != nil
			if isErr != tt.wantErr {
				t.Fatalf("mergeFlagsWithConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			assert.Equal(t, c, tt.expectedConfig)
		})
	}
}

func TestSetStyling(t *testing.T) {
	c := Config{Styling: Styling{
		Theme:        "solarized",
		ColorPalette: tuiutil.ColorPalette{ResultText: tuiutil.NewColor(lipgloss.Color("#ffffaa"))},
	}}
	setStyling(false, &c)
	selectedTheme := tuiutil.GetTheme()
	require.Equal(t, lipgloss.Color("#ffffaa"), selectedTheme.ResultText.TerminalColor)
}

func TestDefaultConfigWritten(t *testing.T) {
	path := uniquePath(t.TempDir())
	cfg := DefaultConfig()
	err := readConfig(path, &cfg, path)
	if err != nil {
		var le clcerrors.LoggableError
		if errors.As(err, &le) {
			t.Fatal(le.VerboseError())
		}
		t.Fatal(err.Error())
	}
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	cfg = Config{}
	require.NoError(t, yaml.Unmarshal(b, &cfg))
	cc := cfg.Hazelcast.Cluster
	require.Equal(t, "dev", cc.Name)
	require.Equal(t, true, cc.Unisocket)
	require.Equal(t, cluster.NetworkConfig{Addresses: []string{"localhost:5701"}}, cc.Network)
	require.Equal(t, cluster.CloudConfig{}, cc.Cloud)
	require.Equal(t, cluster.SecurityConfig{}, cc.Security)
	require.Equal(t, cluster.DiscoveryConfig{}, cc.Discovery)
	require.Equal(t, logger.ErrorLevel, cfg.Hazelcast.Logger.Level)
	require.Equal(t, SSLConfig{}, cfg.SSL)
	require.Equal(t, false, cfg.NoAutocompletion)
	require.Equal(t, "default", cfg.Styling.Theme)
	require.Empty(t, cfg.Logger.LogFile)
}

func TestSetupLogger(t *testing.T) {
	logFileDir := t.TempDir()
	tcs := []struct {
		name                 string
		logFile              string
		gfv                  GlobalFlagValues
		shouldLogToFile      bool // if false, should log to out
		logShouldOnlyContain []string
		isErr                bool
	}{
		{
			name: "No file, no log-level specified. Should log to out.",
			gfv:  GlobalFlagValues{},
		},
		{
			name:            "Log file specified. Should log to file.",
			gfv:             GlobalFlagValues{},
			logFile:         path.Join(logFileDir, "log1.txt"),
			shouldLogToFile: true,
		},
		{
			name:            "Log file specified via flag. Should log to file.",
			gfv:             GlobalFlagValues{LogFile: path.Join(logFileDir, "log2.txt")},
			shouldLogToFile: true,
		},
		{
			name: "Log file specified with both flag and config. Flag should take precedence.It should log to file.",
			gfv: GlobalFlagValues{
				LogFile: path.Join(logFileDir, "log3.txt"),
			},
			logFile:         "/path/that/dont/exist",
			shouldLogToFile: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var bb bytes.Buffer
			c := DefaultConfig()
			c.Logger.LogFile = tc.logFile
			l, err := SetupLogger(&c, &tc.gfv, &bb)
			if tc.isErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			defer l.Close()
			// log level must set to empty, otherwise go client throws an error
			require.Empty(t, c.Hazelcast.Logger.Level)
			customL := c.Hazelcast.Logger.CustomLogger
			// a custom logger must be set in all cases
			require.NotNil(t, customL)
			customL.Log(logger.WeightFatal, func() string {
				return "fatal"
			})
			customL.Log(logger.WeightError, func() string {
				return "error"
			})
			customL.Log(logger.WeightWarn, func() string {
				return "warn"
			})
			l.Println("clc")
			require.NoError(t, l.Close())
			var logs string
			if tc.shouldLogToFile {
				lf := tc.logFile
				if tc.gfv.LogFile != "" {
					lf = tc.gfv.LogFile
				}
				content, err := ioutil.ReadFile(lf)
				require.NoError(t, err)
				logs = string(content)
				// buffer must be untouched if logs are written to the file
				require.Zero(t, bb.Len())
			} else {
				logs = bb.String()
				require.NoFileExists(t, tc.logFile)
			}
			contains := make(map[string]struct{})
			for _, e := range tc.logShouldOnlyContain {
				contains[e] = struct{}{}
			}
			// should contain
			for _, keyword := range []string{"error", "fatal", "clc"} {
				require.Contains(t, logs, keyword)
			}
			require.NotContains(t, logs, "warn")
		})
	}
}

var pathID int32

func uniquePath(parentDir string) string {
	id := atomic.AddInt32(&pathID, 1)
	fn := fmt.Sprintf("config-%05d.yaml", id)
	return filepath.Join(parentDir, fn)
}

func uniquePathWithContent(parentDir string, content []byte) string {
	path := uniquePath(parentDir)
	if err := os.WriteFile(path, content, 0666); err != nil {
		panic(err)
	}
	return path
}
