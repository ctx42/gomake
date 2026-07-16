// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// ExitStatus returns the exit status of the error if it's an instance of
// [exec.ExitError] or it has method: ExitStatus() int. It returns 0 if err is
// nil and 1 if err does not match the above criteria.
func ExitStatus(err error) int {
	if err == nil {
		return 0
	}

	type status interface{ ExitStatus() int }

	var es status
	if errors.As(err, &es) {
		return es.ExitStatus()
	}

	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		if ex, ok := ee.Sys().(status); ok {
			return ex.ExitStatus()
		}
	}
	return 1
}

// HasRun examines the error to determine if it was generated as a result of a
// command running via [exec.Command]. If the error is nil, or the command
// started (including a non-zero exit or a signal kill from a deadline), HasRun
// reports true. If the error is an unrecognized type, or it is an error from
// [exec.Command] that says the command failed to start (usually due to the
// command not existing or not being executable), it reports false.
func HasRun(err error) bool {
	if err == nil {
		return true
	}
	var ee *exec.ExitError
	return errors.As(err, &ee)
}

// GetGOOS returns the GOOS value from env; if unset, it returns
// [runtime.GOOS]. When the key appears more than once, the last value wins.
func GetGOOS(env []string) string {
	ret := runtime.GOOS
	if val, exists := LookupEnv(env, "GOOS"); exists {
		ret = val
	}
	return ret
}

// GetGOARCH returns the GOARCH value from env; if unset, it returns
// [runtime.GOARCH]. When the key appears more than once, the last value wins.
func GetGOARCH(env []string) string {
	ret := runtime.GOARCH
	if val, exists := LookupEnv(env, "GOARCH"); exists {
		ret = val
	}
	return ret
}

// LookupEnv retrieves the value of the environment variable named by the key.
// If the variable is present in the environment, the value (which may be empty)
// is returned and the boolean is true. Otherwise, the returned value will be
// empty and the boolean will be false.
func LookupEnv(env []string, key string) (string, bool) {
	var exists bool
	var value string
	for _, val := range env {
		if strings.HasPrefix(val, key+"=") {
			value = val[len(key)+1:]
			exists = true
		}
	}
	return value, exists
}

// Getenv retrieves the value of the environment variable named by the key.
// It returns the value, which will be empty if the variable is not present.
// To distinguish between an empty value and an unset value, use [LookupEnv].
func Getenv(env []string, key string) string {
	val, _ := LookupEnv(env, key)
	return val
}
