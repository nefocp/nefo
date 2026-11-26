//go:build !linux

package siteimport

import "errors"

func mkfifo(string) error { return errors.New("no FIFOs here") }
