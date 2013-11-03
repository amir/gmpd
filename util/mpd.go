// Package util provides string utilities required by gmpd
package util

import (
	"bytes"
	"strconv"

	"github.com/amir/gpm"
)

// Track is a gpm.Track type alias.
type Track gpm.Track

var supportedCommands = []string{
	"addid", "list", "play", "playid", "playlistfind", "notcommands",
	"urlhandlers", "tagtypes", "playlistid", "list", "playlist", "stop", "pause",
	"currentsong",
}

var notSupportedCommands = []string{
	"idle", "noidle",
}

// String returns MPD-response-formatted representation of a track.
func (t Track) String() string {
	var buffer bytes.Buffer

	duration, err := strconv.Atoi(t.DurationMillis)
	if err != nil {
		duration = 0
	}
	if t.ID == "" {
		buffer.WriteString("file: " + t.Nid + "\n")
	} else {
		buffer.WriteString("file: " + t.ID + "\n")
	}
	buffer.WriteString("Time: " + strconv.Itoa(duration/1000) + "\n")
	buffer.WriteString("Artist: " + t.Artist + "\n")
	buffer.WriteString("Title: " + t.Title + "\n")
	buffer.WriteString("Album: " + t.Album + "\n")

	return buffer.String()
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
