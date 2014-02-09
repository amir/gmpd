// gmpd is a MPD-esque daemon which can play music from
// Google Play Music and Google Play Music All Access.
//
// You need a registered device to use this daemon.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	cp "github.com/amir/gmpd/contentprovider"
	"github.com/amir/gmpd/util"
	"github.com/amir/gst"
	"github.com/ziutek/glib"
)

const mpdVersion = "0.17.0"

// MPD ACK_ERRORs
const (
	AckErrorNotList    = 1
	AckErrorArg        = 2
	AckErrorPassword   = 3
	AckErrorPermission = 4
	AckErrorUnknown    = 5

	AckErrorNoExist       = 50
	AckErrorPlaylistMax   = 51
	AckErrorSystem        = 52
	AckErrorPlaylistLoad  = 53
	AckErrorUpdateAlready = 54
	AckErrorPlayerSync    = 55
	AckErrorExist         = 56
)

// MPD client list modes
const (
	ClientListModeBegin   = "command_list_begin"
	ClientListOkModeBegin = "command_list_ok_begin"
	ClientListModeEnd     = "command_list_end"
)

// Playlist represents player's current playlist.
type Playlist struct {
	tracks   []string // track IDs
	position int      // current position
}

// commandList represents daemon's commands queue.
type commandList struct {
	commands []string // list of commands
	active   bool     // are we in command list mode?
	okMode   bool     // should print a list_OK after each commands output
}

// gmpd represents a google MPD.
type gmpd struct {
	cp          *cp.ContentProvider // Proxies (and caches) Google Play Music WS calls
	playlist    *Playlist           // daemon's playlist
	startTime   int64               // when daemon started
	commandList *commandList        // daemon's queued commands list
}

// Player represents a GStreamer playbin for playing music.
type Player struct {
	pipe *gst.Element
	bus  *gst.Bus
}

var (
	daemon *gmpd
	player *Player

	serviceAddress = flag.String("address", ":6600", "gMPD service address")
	email          = flag.String("email", "email", "Google account email")
	password       = flag.String("password", "password", "Google account password")
	cacheDir       = flag.String("cache-dir", "", "Cache directory")
)

// onMessage is GStreamer's playbin bus message callback.
func (p *Player) onMessage(bus *gst.Bus, msg *gst.Message) {
	switch msg.GetType() {
	case gst.MESSAGE_EOS:
		p.pipe.SetState(gst.STATE_NULL)
		daemon.playlist.playNext()
	case gst.MESSAGE_ERROR:
		p.pipe.SetState(gst.STATE_NULL)
		err, debug := msg.ParseError()
		fmt.Printf("Error: %s (debug: %s)\n", err, debug)
	}
}

// onSyncMessage is GStreamer's playbin bus sync element callback.
func (p *Player) onSyncMessage(bus *gst.Bus, msg *gst.Message) {
}

// play sets GStreamer pipe's URI prorperty, and set the state to play.
func (p *Player) play(url string) {
	p.pipe.SetProperty("uri", url)
	p.pipe.SetState(gst.STATE_PLAYING)
}

// pause pauses player if its playing.
func (p *Player) pause() {
	state, _, _ := p.pipe.GetState(gst.CLOCK_TIME_NONE)
	if state == gst.STATE_PLAYING {
		p.pipe.SetState(gst.STATE_PAUSED)
	}
}

// stop stops player.
func (p *Player) stop() {
	p.pipe.SetState(gst.STATE_NULL)
}

// state reports player's state.
func (p *Player) state() string {
	state, _, _ := p.pipe.GetState(gst.CLOCK_TIME_NONE)
	switch state {
	case gst.STATE_PLAYING:
		return "play"
	default:
		return "stop"
	}
}

// trackPosition returns track's position in playlist
func (p *Playlist) trackPosition(track string) int {
	index := -1
	for i, s := range p.tracks {
		if s == track {
			index = i
		}
	}

	return index
}

// String returns MPD-response-formatted representation of the playlist
func (p *Playlist) String() string {
	buffer := bytes.NewBufferString("")

	for p, t := range p.tracks {
		fmt.Fprintf(buffer, "%d:file: %s\n", p, t)
	}

	return buffer.String()
}

// trackAtPosition returns track ID at provided position in playlist
func (p *Playlist) trackAtPosition(pos int) (track string, err error) {
	if pos >= 0 && pos < p.length() {
		return p.tracks[pos], nil
	}
	return "", errors.New("track does not exist")
}

// currentTrack returns current track in playlist
func (p *Playlist) currentTrack() (tack string, err error) {
	if p.length() > 0 {
		return p.tracks[p.position], nil
	}

	return "", errors.New("playlist is empty")
}

// playNext plays the next track in playlist
func (p *Playlist) playNext() {
	if p.position < p.length() {
		track, err := p.trackAtPosition(p.position + 1)
		if err == nil {
			url, err := daemon.cp.TrackStreamURL(track)
			if err == nil {
				player.play(url)
				p.position = p.position + 1
			}
		}
	}
}

// addTrack adds a new track to playlist
func (p *Playlist) addTrack(track string) int {
	p.tracks = append(p.tracks, track)
	return p.length() - 1
}

// length returns number of tracks in playlist
func (p *Playlist) length() int {
	return len(p.tracks)
}

// being begins consuming, and populating commands
func (c *commandList) begin(okMode bool) {
	var commands []string
	c.commands = commands
	c.active = true
	c.okMode = okMode
}

// add adds a new command to queue
func (c *commandList) add(command string) {
	c.commands = append(c.commands, command)
}

// process process all commands in command list
func (c *commandList) process() []byte {
	var response []byte
	for _, s := range c.commands {
		r, ackError := processCommand(s)
		if ackError > 0 {
			break
		}
		if c.okMode {
			r = append(r, []byte("list_OK\n")...)
		}
		response = append(response, r...)
	}
	return response
}

// reset clear the queue
func (c *commandList) reset() {
	c.commands = c.commands[:0]
	c.active = false
}

// processCommand process MPD commands, and responds to them
func processCommand(commandString string) ([]byte, int) {
	ackError := 0
	var responseBuffer bytes.Buffer
	response := bufio.NewWriter(&responseBuffer)

	tok := util.NewTokenizer(commandString)
	command := tok.NextParam()
	switch command {
	case "add":
		songID := tok.NextParam()
		daemon.playlist.addTrack(songID)
	case "addid":
		songID := tok.NextParam()
		fmt.Fprintf(response, "Id: %d\n", daemon.playlist.addTrack(songID))

	case "playlistfind":
		tok.NextParam()
		filename := tok.NextParam()
		pos := daemon.playlist.trackPosition(filename)
		if pos > -1 {
			track, err := daemon.cp.FindTrack(filename)
			if err != nil {
				ackError = AckErrorNoExist
			} else {
				fmt.Fprintf(response, "%s", track)
				fmt.Fprintf(response, "Pos: %d\n", pos)
				fmt.Fprintf(response, "Id: %d\n", pos)
			}
		} else {
			ackError = AckErrorNoExist
		}

	case "commands":
		fmt.Fprintf(response, "%s", util.MPDSupportedCommands())

	case "notcommands":
		fmt.Fprintf(response, "%s", util.MPDNotSupportedCommands())

	case "playid":
		pos, err := strconv.Atoi(tok.NextParam())
		if err == nil && pos <= daemon.playlist.length() {
			filename, err := daemon.playlist.trackAtPosition(pos)
			if err == nil {
				url, err := daemon.cp.TrackStreamURL(filename)
				if err != nil {
					ackError = AckErrorNoExist
				} else {
					player.play(url)
				}
			} else {
				ackError = AckErrorNoExist
			}
		}

	case "stop":
		player.stop()

	case "pause":
		player.pause()

	case "playlist":
		fmt.Fprintf(response, "%s", daemon.playlist)

	case "playlistinfo":
		fallthrough
	case "playlistid":
		id, err := strconv.Atoi(tok.NextParam())
		if err != nil {
			ackError = AckErrorNoExist
			break
		}

		filename, err := daemon.playlist.trackAtPosition(id)
		if err != nil {
			ackError = AckErrorNoExist
			break
		}

		track, err := daemon.cp.FindTrack(filename)
		if err != nil {
			ackError = AckErrorNoExist
		} else {
			fmt.Fprintf(response, "%s", track)
			fmt.Fprintf(response, "Pos: %d\n", id)
			fmt.Fprintf(response, "Id: %d\n", id)
		}

	case "status":
		response.Write([]byte("playlist: 0\n"))
		fmt.Fprintf(response, "playlistlength: %d\n", daemon.playlist.length())
		state := player.state()
		if state == "play" {
			response.Write([]byte("state: " + state + "\n"))
			fmt.Fprintf(response, "song: %d\n", daemon.playlist.position)
			fmt.Fprintf(response, "songid: %d\n", daemon.playlist.position)
			ok, pos := player.pipe.GetPosition()
			if ok {
				pos = pos / 1000000000
				fmt.Fprintf(response, "elapsed: %d.00\n", pos)
				fmt.Fprintf(response, "time: %d:00\n", pos)
			}
		} else {
			response.Write([]byte("state: " + state + "\n"))
		}

	case "search":
		var queryBuffer bytes.Buffer
		query := tok.NextParam()
		for i := 0; query != ""; i++ {
			query = tok.NextParam()
			if i%2 == 0 {
				queryBuffer.WriteString(query + " ")
			}
		}
		query = queryBuffer.String()

		tracks, err := daemon.cp.FindTracks(query)
		if err != nil {
			break
		}
		for _, track := range tracks {
			fmt.Fprintf(response, "%s", track)
		}

	case "find":
		queryType := tok.NextParam()
		query := tok.NextParam()
		switch queryType {
		case "album":
			album, err := daemon.cp.FindAlbum(query, true)
			if err != nil {
				for _, track := range album.Tracks {
					fmt.Fprintf(response, "%s", track)
				}
			}
		case "artist":
			tok.NextParam()
			album := tok.NextParam()
			tracks := daemon.cp.FindTracksByArtist(query, album)
			for _, track := range tracks {
				fmt.Fprintf(response, "%s", track)
			}
		}

	case "outputs":
		response.Write([]byte("outputid: 0\n"))
		response.Write([]byte("outputname: My Pulse Output\no"))
		response.Write([]byte("outputenabled: 1\n"))

	case "stats":
		now := time.Now()
		response.Write([]byte("uptime: " +
			strconv.FormatInt(now.Unix()-daemon.startTime, 10) + "\n"))

	case "lsinfo":
		tracks, err := daemon.cp.UserTracks()
		if err != nil {
			log.Fatal(err)
		}
		for _, track := range tracks {
			fmt.Fprintf(response, "%s", track)
		}

	case "list":
		tagType := tok.NextParam()
		query := tok.NextParam()
		switch tagType {
		case "album":
			var albums []cp.Album
			if query == "artist" {
				artist := tok.NextParam()
				albums = daemon.cp.FindAlbumsByArtistName(artist)
			} else {
				albums, _ = daemon.cp.FindAlbums(query)
			}
			for _, album := range albums {
				fmt.Fprintf(response, "Album: %s\n", album.Name)
			}
		case "artist":
			artists := daemon.cp.ListArtists(query)
			for _, artist := range artists {
				fmt.Fprintf(response, "Artist: %s\n", artist.Name)
			}
		case "date":
			if query == "artist" {
				artist := tok.NextParam()
				tok.NextParam() // should be tag name, ignore it.
				album := tok.NextParam()
				log.Printf("Album is: %v", album)
				albums := daemon.cp.FindAlbumsByArtistName(artist)
				years := make(map[uint16]bool)
				for _, a := range albums {
					if !years[a.Year] {
						if album != "" {
							if a.Name == album {
								years[a.Year] = true
							}
						} else {
							years[a.Year] = true
						}
					}
				}
				for k, _ := range years {
					fmt.Fprintf(response, "Date: %d\n", k)
				}
			}
		}

	case "listplaylists":
		playlists, err := daemon.cp.Playlists()
		if err != nil {
			break
		}
		for _, playlist := range playlists {
			fmt.Fprintf(response, "playlist: %s\n", playlist.Name)
		}

	case "currentsong":
		state := player.state()
		if state != "play" {
			break
		}
		filename, err := daemon.playlist.currentTrack()
		track, err := daemon.cp.FindTrack(filename)
		if err != nil {
			ackError = AckErrorNoExist
			break
		}
		fmt.Fprintf(response, "%s", track)

	case "urlhandlers":
		fallthrough
	case "tagtypes":

	default:
		ackError = AckErrorUnknown
	}

	response.Flush()
	return responseBuffer.Bytes(), ackError
}

// handleMessage handles incoming messages from clients
func handleMessage(client net.Conn) {
	b := bufio.NewReader(client)
	for {
		line, err := b.ReadBytes('\n')
		if err != nil {
			break
		}
		ackError := 0
		response := []byte("")
		commandString := strings.TrimSpace(string(line))
		tok := util.NewTokenizer(commandString)
		command := tok.NextParam()

		if daemon.commandList.active == true {
			if command == ClientListModeEnd {
				response = daemon.commandList.process()
				daemon.commandList.reset()
			} else {
				daemon.commandList.add(commandString)
			}
		} else {
			if command == ClientListModeBegin {
				daemon.commandList.begin(false)
			} else if command == ClientListOkModeBegin {
				daemon.commandList.begin(true)
			} else {
				response, ackError = processCommand(commandString)
			}
		}

		if daemon.commandList.active == false {
			if ackError > 0 {
				switch ackError {
				case AckErrorUnknown:
					fmt.Fprintf(client, "ACK [%d@%d] {} unknown command \"%s\"\n", ackError, 0, command)
				default:
					fmt.Fprintf(client, "ACK [%d@%d] {%s}\n", ackError, 0, command)
				}
			} else {
				client.Write(response)
				client.Write([]byte("OK\n"))
			}
		}
	}
}

// NewPlayer allocates a new Player.
func NewPlayer() *Player {
	p := new(Player)

	p.pipe = gst.ElementFactoryMake("playbin2", "autoplay")
	p.bus = p.pipe.GetBus()
	p.bus.AddSignalWatch()
	p.bus.Connect("message", (*Player).onMessage, p)
	p.bus.EnableSyncMessageEmission()
	p.bus.Connect("sync-message::element", (*Player).onSyncMessage, p)

	return p
}

// NewGmpd allocates a new gmpd.
func NewGmpd() *gmpd {
	contentProvider, err := cp.New(*email, *password, *cacheDir)
	if err != nil {
		log.Fatal(err)
	}

	return &gmpd{
		cp:          contentProvider,
		playlist:    new(Playlist),
		commandList: new(commandList),
	}
}

func mpdListener() {
	listener, err := net.Listen("tcp", *serviceAddress)
	if err != nil {
		log.Fatalf("ListenAndServe: %s", err.Error())
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		conn.Write([]byte("OK MPD " + mpdVersion + "\n"))
		go handleMessage(conn)
	}
	listener.Close()
}

func init() {
	flag.Parse()
	if *cacheDir == "" {
		*cacheDir = util.CacheDir()
	}
	daemon = NewGmpd()
	player = NewPlayer()

	now := time.Now()
	daemon.startTime = now.Unix()
}

func main() {
	go mpdListener()
	glib.NewMainLoop(nil).Run()
}
