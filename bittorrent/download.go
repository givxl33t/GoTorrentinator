package bittorrent

import (
	"crypto/rand"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/givxl33t/bittorrent-client-go/peer"
	"github.com/givxl33t/bittorrent-client-go/torrentparser"
	"github.com/givxl33t/bittorrent-client-go/tracker"
)

type Download struct {
	Torrent     torrentparser.TorrentFile
	PeerId      [20]byte
	PeerClients []*peer.Client
}

// sets up worker threads to download the torrent
// whether it be a torrentfile or magnet links
// parse off the infohash and tracker urls
func NewDownload(source string) (*Download, error) {
	torrent, err := torrentparser.New(source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse torrent file: %w", err)
	}

	var peerID [20]byte
	rand.Read(peerID[:])
	// random port
	port := 6881

	var peerAddrs []net.TCPAddr
	var wg sync.WaitGroup
	var mut sync.Mutex

	// get peer addresses from trackers
	wg.Add(len(torrent.TrackerURLs))
	for _, trackerURL := range torrent.TrackerURLs {
		trackerURL := trackerURL
		go func() {
			defer wg.Done()
			addrs, err := tracker.GetPeers(trackerURL, torrent.InfoHash, peerID, port)
			if err != nil {
				fmt.Printf("failed to get peers from tracker %s: %s\n", trackerURL, err.Error())
				return
			}

			mut.Lock()
			fmt.Printf("peers from %s: %d\n", trackerURL, len(addrs))
			peerAddrs = append(peerAddrs, addrs...)
			mut.Unlock()
		}()
	}
	wg.Wait()

	// dedupe peer addresses
	peerAddrs = dedupeAddrs(peerAddrs)

	// create all peer clients
	var peerClients []*peer.Client
	wg.Add(len(peerAddrs))
	for _, addr := range peerAddrs {
		addr := addr
		go func() {
			defer wg.Done()
			client, err := peer.NewClient(addr, torrent.InfoHash, peerID)
			if err != nil {
				fmt.Printf("failed connecting to peer at %s: %s\n", addr.String(), err.Error())
				return
			}

			mut.Lock()
			peerClients = append(peerClients, client)
			mut.Unlock()
		}()
	}
	wg.Wait()

	fmt.Println("total peer count:", len(peerClients))

	if len(peerClients) == 0 {
		return nil, fmt.Errorf("no peers found")
	}

	// get metadata if it was a magnet link
	if strings.HasPrefix(source, "magnet") {
		var err error
		var metadataBytes []byte
		for _, client := range peerClients {
			metadataBytes, err = client.GetMetadata(torrent.InfoHash)
			if err != nil {
				fmt.Printf("failed to get metadata from peer %s: %s\n", client.Addr().String(), err.Error())
			}
			if err == nil {
				break
			}
		}

		if len(metadataBytes) == 0 {
			return nil, fmt.Errorf("failed to get metadata from any peer")
		}

		err = torrent.AppendMetadata(metadataBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to append metadata: %w", err)
		}
	}

	return &Download{
		Torrent:     torrent,
		PeerClients: peerClients,
		PeerId:      peerID,
	}, nil
}

// helper function to dedupe all the addresses from multiple tracker responses
func dedupeAddrs(addrs []net.TCPAddr) []net.TCPAddr {
	deduped := []net.TCPAddr{}
	set := map[string]bool{}
	for _, a := range addrs {
		if !set[a.String()] {
			deduped = append(deduped, a)
			set[a.String()] = true
		}
	}

	return deduped
}
