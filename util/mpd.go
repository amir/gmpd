// Package util provides string utilities required by gmpd
package util

import (
	"bytes"
)

var supportedCommands = []string{
	"addid", "list", "play", "playid", "playlistfind", "notcommands",
	"urlhandlers", "tagtypes", "playlistid", "list", "playlist", "stop", "pause",
	"currentsong",
}

var notSupportedCommands = []string{
	"idle", "noidle",
}

// MPDSupportedCommands returns list of supported MPD commands.
func MPDSupportedCommands() string {
	var buffer bytes.Buffer

	for _, c := range supportedCommands {
		buffer.WriteString("command: " + c + "\n")
	}

	return buffer.String()
}

// MPDNotSupportedCommands returns list of not supported MPD commands.
func MPDNotSupportedCommands() string {
	var buffer bytes.Buffer

	for _, c := range notSupportedCommands {
		buffer.WriteString("command: " + c + "\n")
	}

	return buffer.String()
}
