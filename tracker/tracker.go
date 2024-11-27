package tracker

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/zeebo/bencode"
)

// GetPeers will attempt to contact the tracker and return a list of peers
func GetPeers(trackerURL string, infoHash, peerID [20]byte, port int) ([]net.TCPAddr, error) {
	u, err := url.Parse(trackerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tracker url: %w", err)
	}

	switch u.Scheme {
	case "http", "https":
		return getPeersFromHTTPTracker(u, infoHash, peerID, port)
	case "udp":
		return getPeersFromUDPTracker(u, infoHash, peerID, port)
	default:
		return nil, fmt.Errorf("unsupported tracker protocol: %s", u.Scheme)
	}
}

type compactHTTPTrackerResponse struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

// a verbose HTTP tracker response
type httpTrackerResponse struct {
	Peers []struct {
		ID   string `bencode:"peer id"`
		IP   string `bencode:"ip"`
		Port int    `bencode:"port"`
	} `bencode:"peers"`
	Interval   int    `bencode:"interval"`
	InfoHash   string `bencode:"info_hash"`
	Uploaded   int    `bencode:"uploaded"`
	Downloaded int    `bencode:"downloaded"`
	Left       int    `bencode:"left"`
	Event      string `bencode:"event"`
}

func getPeersFromHTTPTracker(u *url.URL, infoHash, peerID [20]byte, port int) ([]net.TCPAddr, error) {
	v := url.Values{}
	v.Add("info_hash", string(infoHash[:]))
	v.Add("peer_id", string(peerID[:]))
	v.Add("port", strconv.Itoa(port))
	v.Add("uploaded", "0")
	v.Add("downloaded", "0")
	v.Add("left", "0")
	v.Add("compact", "1")

	// set url query params
	u.RawQuery = v.Encode()

	// context for 3 seconds timeout of http request
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// make http request
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make http request: %w", err)
	}
	defer resp.Body.Close()

	// read all the bytes
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read http response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http response status code: %d", resp.StatusCode)
	}

	var compactResponse compactHTTPTrackerResponse
	err = bencode.DecodeBytes(raw, &compactResponse)

	if err == nil {
		var addrs []net.TCPAddr
		const peerSize = 6 // 4 bytes for IP, 2 for port
		if len(compactResponse.Peers)%peerSize != 0 {
			return nil, fmt.Errorf("invalid peers: %s", compactResponse.Peers)
		}

		for i := 0; i < len(compactResponse.Peers); i += peerSize {
			// convert port substring into byte slice to calculate via BigEndian
			portRaw := []byte(compactResponse.Peers[i+4 : i+6])
			port := binary.BigEndian.Uint16(portRaw)

			addrs = append(addrs, net.TCPAddr{
				IP:   []byte(compactResponse.Peers[i : i+4]),
				Port: int(port),
			})
		}

		return addrs, nil
	}

	// parse as original format
	var originalResponse httpTrackerResponse
	err = bencode.DecodeBytes(raw, &originalResponse)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling http response: %w", err)
	}

	var addrs []net.TCPAddr
	for _, peer := range originalResponse.Peers {
		// assume ipv4 and not domain names.
		addrs = append(addrs, net.TCPAddr{
			IP:   []byte(peer.IP),
			Port: peer.Port,
		})
	}

	return addrs, nil
}
