package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/hekmon/transmissionrpc"
)

var (
	host     = "127.0.0.1"
	port     = 9091
	https    = false
	username = "rpcuser"
	password = "rpcpass"
	debug    = false
)

// const trackersSource = "https://raw.githubusercontent.com/ngosang/trackerslist/master/trackers_all.txt"
const trackersSource = "https://cdn.jsdelivr.net/gh/ngosang/trackerslist/trackers_all.txt"

var trackers []string

func main() {
	if err := loadTrackers(); err != nil {
		panic(fmt.Errorf("load trackers failed: %w", err))
	}

	conf := transmissionrpc.AdvancedConfig{
		Port:  uint16(port),
		Debug: debug,
	}

	client, err := transmissionrpc.New(host, username, password, &conf)
	if err != nil {
		panic(fmt.Errorf("new transmission rpc failed: %w", err))
	}

	ok, serverVersion, serverMinimumVersion, err := client.RPCVersion()
	if err != nil {
		panic(err)
	}
	if !ok {
		panic(fmt.Sprintf("remote transmission RPC version (v%d) is incompatible with the transmission library (v%d): remote needs at least v%d",
			serverVersion, transmissionrpc.RPCVersion, serverMinimumVersion))
	}
	log.Printf("remote transmission RPC version (v%d)", serverVersion)

	torrents, err := client.TorrentGetAll()
	if err != nil {
		log.Printf("get all torrents failed: %s", err)
		return
	}
	for _, t := range torrents {
		log.Printf("torrend %d \"%s\"", *t.ID, *t.Name)
		addTrackers(client, t)
	}
}

func addTrackers(client *transmissionrpc.Client, torrent *transmissionrpc.Torrent) error {
	oldTrackers := make(map[string]bool)
	for _, tracker := range torrent.Trackers {
		oldTrackers[tracker.Announce] = true
	}

	newTrackers := getTrackers()
	var toAddTrackers []string
	for _, nt := range newTrackers {
		if !oldTrackers[nt] {
			toAddTrackers = append(toAddTrackers, nt)
		}
	}

	if len(toAddTrackers) == 0 {
		log.Printf("torrent %d \"%s\" has all trackers, skip add", *torrent.ID, *torrent.Name)
		return nil
	}

	payload := transmissionrpc.TorrentSetPayload{
		TrackerAdd: toAddTrackers,
		IDs:        []int64{*torrent.ID},
	}
	if err := client.TorrentSet(&payload); err != nil {
		return fmt.Errorf("update torrent %d \"%s\" failed: %w", *torrent.ID, *torrent.Name, err)
	}
	log.Printf("torrent %d \"%s\" added %d new trackers",
		*torrent.ID, *torrent.Name, len(toAddTrackers))
	return nil
}

func getTrackers() []string {
	return trackers
}

func loadTrackers() error {
	resp, err := http.Get(trackersSource)
	if err != nil {
		return fmt.Errorf("get %s failed: %w", trackersSource, err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		trackers = append(trackers, line)
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return err
	}

	log.Printf("trackers: %+v", trackers)
	return nil
}
