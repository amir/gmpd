package contentprovider

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/amir/gpm"
	_ "github.com/mattn/go-sqlite3"
)

type ContentProvider struct {
	gpm      *gpm.Client
	db       *sql.DB
	deviceID string
}

// Track is a gpm.Track type alias.
type Track gpm.Track

// Track is a gpm.Album type alias.
type Album gpm.Album

// Artist is a gpm.Artist type alias.
type Artist gpm.Artist

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

var schemaVersion = 1

var sqlCreateTables []string = []string{
	`CREATE TABLE tracks (
    id VARCHAR(255) NOT NULL PRIMARY KEY,
    nid VARCHAR(255),
    title VARCHAR(255) NOT NULL,
    album VARCHAR(255) NOT NULL,
    artist VARCHAR(255) NOT NULL,
    albumId VARCHAR(255) NOT NULL,
    duration INTEGER)`,
	`CREATE TABLE albums (
    id VARCHAR(255) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    artist VARCHAR(255) NOT NULL,
    year CHAR(4))`,
}

func New(email, password, cacheDir string) (*ContentProvider, error) {
	gpmClient, deviceID, err := newGPMClient(email, password)
	if err != nil {
		return nil, err
	}

	db, err := newDb(cacheDir)
	if err != nil {
		return nil, err
	}

	return &ContentProvider{
		gpm:      gpmClient,
		db:       db,
		deviceID: deviceID,
	}, nil
}

func newGPMClient(username, password string) (*gpm.Client, string, error) {
	gpmc := gpm.New(username, password)
	err := gpmc.Login()
	if err != nil {
		return nil, "", err
	}

	settings, err := gpmc.Settings()
	if err != nil {
		return nil, "", err
	}

	var deviceID string
	for _, d := range settings.Settings.Devices {
		if d["type"] == "PHONE" {
			deviceID = d["id"].(string)
			deviceID = deviceID[2:len(deviceID)]
		}
	}
	if deviceID == "" {
		return nil, "", errors.New("No registered device found")
	}

	return gpmc, deviceID, nil
}

func newDb(cacheDir string) (*sql.DB, error) {
	if fi, err := os.Stat(cacheDir); os.IsNotExist(err) || !fi.IsDir() {
		err = os.MkdirAll(cacheDir, 0700)
		if err != nil {
			return nil, err
		}
	}
	path := filepath.Join(cacheDir, "content-provider.db")
	if fi, err := os.Stat(path); os.IsNotExist(err) || (fi != nil && fi.Size() == 0) {
		err = initSqliteDatabase(path)
		if err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite3", path)
	return db, err
}

func initSqliteDatabase(path string) error {
	_, err := os.Create(path)
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer db.Close()
	for _, createTableSql := range sqlCreateTables {
		_, err = db.Exec(createTableSql)
		if err != nil {
			return err
		}
		_, err = db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion))
		if err != nil {
			return err
		}
	}

	return nil
}

func (cp *ContentProvider) persistTrack(track Track) {
	stmt, _ := cp.db.Prepare(`
	  INSERT INTO tracks(id, nid, title, album, artist, albumId, duration)
	  VALUES (?, ?, ?, ?, ?, ?, ?)`)

	stmt.Exec(track.ID, track.Nid, track.Title, track.Album, track.Artist,
		track.AlbumID, track.DurationMillis)
}

func (cp *ContentProvider) retrieveTrack(trackID string) Track {
	var track Track
	cp.db.QueryRow(`SELECT
      id, nid, title, album, albumId, artist, duration
      FROM tracks WHERE id = ?`,
		trackID).Scan(&track.ID, &track.Nid, &track.Title, &track.Album,
		&track.AlbumID, &track.Artist, &track.DurationMillis)

	return track
}

func (cp *ContentProvider) persistAlbum(album Album) {
	stmt, _ := cp.db.Prepare("INSERT INTO albums(id, name, artist, year) VALUES (?, ?, ?, ?)")

	stmt.Exec(album.ID, album.Name, album.Artist, album.Year)
}

func (cp *ContentProvider) TrackStreamURL(track string) (string, error) {
	url, err := cp.gpm.MP3StreamURL(track, cp.deviceID)
	if err != nil {
		return "", err
	}

	return url, nil
}

func (cp *ContentProvider) Playlists() ([]gpm.Playlist, error) {
	return cp.Playlists()
}

func (cp *ContentProvider) FindTrack(trackID string) (Track, error) {
	track := cp.retrieveTrack(trackID)
	if track.ID == "" {
		gpmTrack, err := cp.gpm.TrackInfo(trackID)
		if err != nil {
			return track, err
		}
		track = Track(gpmTrack)
		cp.persistTrack(track)
	}

	return track, nil
}

func (cp *ContentProvider) FindTracks(query string) ([]Track, error) {
	gpmTracks, err := cp.gpm.SearchAllAccessTracks(query, 200)
	if err != nil {
		return nil, err
	}
	var tracks = make([]Track, len(gpmTracks))
	for i, track := range gpmTracks {
		t := Track(track)
		cp.persistTrack(t)
		tracks[i] = t
	}
	return tracks, err
}

func (cp *ContentProvider) FindAlbum(albumID string, includeTracks bool) (Album, error) {
	gpmAlbum, err := cp.gpm.AlbumInfo(albumID, includeTracks)
	var album Album
	if err != nil {
		return album, err
	}
	album = Album(gpmAlbum)
	cp.persistAlbum(album)

	return album, nil
}

func (cp *ContentProvider) FindAlbums(query string) ([]Album, error) {
	gpmAlbums, err := cp.gpm.SearchAllAccessAlbums(query, 200)
	if err != nil {
		return nil, err
	}
	var albums = make([]Album, len(gpmAlbums))
	for i, album := range gpmAlbums {
		a := Album(album)
		cp.persistAlbum(a)
		albums[i] = a
	}

	return albums, err
}

func (cp *ContentProvider) UserTracks() ([]Track, error) {
	gpmTrackList, err := cp.gpm.TrackList()
	if err != nil {
		return nil, err
	}
	var tracks = make([]Track, len(gpmTrackList.Data.Items))
	for i, track := range gpmTrackList.Data.Items {
		t := Track(track)
		cp.persistTrack(t)
		tracks[i] = t
	}

	return tracks, nil
}

func (cp *ContentProvider) ListArtists(query string) []Artist {
	var artists []Artist
	stmt, err := cp.db.Prepare(`SELECT DISTINCT(artist) FROM albums WHERE
	  artist LIKE ? AND artist <> ""`)
	if err != nil {
		return artists
	}
	defer stmt.Close()
	rows, err := stmt.Query(fmt.Sprintf("%%%s%%", query))
	if err != nil {
		return artists
	}
	defer rows.Close()
	for rows.Next() {
		var artist Artist
		rows.Scan(&artist.Name)
		artists = append(artists, artist)
	}

	return artists
}

func (cp *ContentProvider) FindAlbumsByArtistName(artist string) []Album {
	var albums []Album
	stmt, err := cp.db.Prepare(`SELECT id, name, artist, year FROM albums WHERE artist = ?`)
	if err != nil {
		return albums
	}
	defer stmt.Close()
	rows, err := stmt.Query(artist)
	if err != nil {
		return albums
	}
	defer rows.Close()
	for rows.Next() {
		var album Album
		rows.Scan(&album.ID, &album.Name, &album.Artist, &album.Year)
		albums = append(albums, album)
	}

	return albums
}

func (cp *ContentProvider) FindTracksByArtist(artist, album string) []Track {
	var tracks []Track
	stmt, err := cp.db.Prepare(`SELECT id, nid, title, album, albumId, artist, duration FROM tracks WHERE artist = ? AND album LIKE ?`)
	if err != nil {
		return tracks
	}
	defer stmt.Close()
	rows, err := stmt.Query(artist, fmt.Sprintf("%%%s%%", album))
	if err != nil {
		return tracks
	}
	defer rows.Close()
	for rows.Next() {
		var track Track
		rows.Scan(&track.ID, &track.Nid, &track.Title, &track.Album, &track.AlbumID, &track.Artist, &track.DurationMillis)
		tracks = append(tracks, track)
	}

	return tracks
}
