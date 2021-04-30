package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/hekmon/transmissionrpc"
)

var (
	host           string
	port           int
	useHTTPS       bool
	username       string
	password       string
	debug          bool
	trackersSource string
)

const (
	ngoSang               = "https://raw.githubusercontent.com/ngosang/trackerslist/master/trackers_all.txt"
	ngoSangJsDelivrMirror = "https://cdn.jsdelivr.net/gh/ngosang/trackerslist/trackers_all.txt"
)

func parseFlag(fs *flag.FlagSet, args []string) error {
	fs.StringVar(&host, "host", "127.0.0.1", "")
	fs.IntVar(&port, "port", 9091, "")
	fs.BoolVar(&useHTTPS, "use-https", false, "")
	fs.StringVar(&username, "username", "rpcuser", "")
	fs.StringVar(&password, "password", "rpcpass", "")
	fs.BoolVar(&debug, "debug", false, "")
	fs.StringVar(&trackersSource, "trackers-source", ngoSangJsDelivrMirror, "")
	return fs.Parse(args)
}

func main() {
	if err := parseFlag(flag.CommandLine, os.Args[1:]); err != nil {
		log.Println("parse command line args failed", err)
		return
	}

	client, err := connect(host, username, password, transmissionrpc.AdvancedConfig{
		Port:  uint16(port),
		Debug: debug,
	})
	if err != nil {
		log.Println("connect transmission failed:", err)
		return
	}

	torrents, err := client.TorrentGetAll()
	if err != nil {
		log.Printf("get all torrents failed: %s", err)
		return
	}
	if len(torrents) == 0 {
		log.Println("no torrents found.")
		return
	}

	trackers, err := getTrackers()
	if err != nil {
		log.Println("load trackers failed: ", err)
		return
	}

	for _, t := range torrents {
		log.Printf("torrend %d \"%s\"", *t.ID, *t.Name)
		if err := addTrackers(client, t, trackers); err != nil {
			log.Printf("add trackers to %d '%s' failed: %s\n", *t.ID, *t.Name, err)
			return
		}
	}
}

func connect(
	host, username, password string, conf transmissionrpc.AdvancedConfig,
) (*transmissionrpc.Client, error) {

	client, err := transmissionrpc.New(host, username, password, &conf)
	if err != nil {
		return nil, fmt.Errorf("new transmission rpc failed: %w", err)
	}

	ok, serverVersion, serverMinimumVersion, err := client.RPCVersion()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf(
			"remote transmission RPC version (v%d) is incompatible with the transmission library (v%d): remote needs at least v%d",
			serverVersion, transmissionrpc.RPCVersion, serverMinimumVersion)
	}
	log.Printf("remote transmission RPC version (v%d)", serverVersion)
	return client, nil
}

func addTrackers(
	client *transmissionrpc.Client, torrent *transmissionrpc.Torrent, newTrackers []string) error {
	oldTrackers := make(map[string]bool)
	for _, tracker := range torrent.Trackers {
		oldTrackers[tracker.Announce] = true
	}

	var toAddTrackers []string
	for _, nt := range newTrackers {
		if !oldTrackers[nt] {
			toAddTrackers = append(toAddTrackers, nt)
		}
	}

	if len(toAddTrackers) == 0 {
		log.Printf("torrent %d \"%s\" has all trackers",
			*torrent.ID, *torrent.Name)
		return nil
	}

	payload := transmissionrpc.TorrentSetPayload{
		TrackerAdd: toAddTrackers,
		IDs:        []int64{*torrent.ID},
	}
	if err := client.TorrentSet(&payload); err != nil {
		return fmt.Errorf("update torrent %d \"%s\" failed: %w",
			*torrent.ID, *torrent.Name, err)
	}
	log.Printf("torrent %d \"%s\" added %d new trackers",
		*torrent.ID, *torrent.Name, len(toAddTrackers))
	return nil
}

func getTrackers() ([]string, error) {
	log.Println("download trackers from ", trackersSource)

	var trackers []string

	resp, err := http.Get(trackersSource)
	if err != nil {
		return nil, fmt.Errorf("get %s failed: %w", trackersSource, err)
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
		return nil, err
	}

	log.Printf("trackers: %+v", trackers)
	return trackers, nil
}
