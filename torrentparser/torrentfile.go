package torrentparser

import (
	"fmt"
	"strings"

	// bencode
	"github.com/zeebo/bencode"
)

// TorrentFile represents a torrent file contents, parsed from a torrent file.
// To ease the downloading process
//
// 20-bytes SHA1 hashes are formatted as 20-byte arrays for easy
// comparison of piece hashes
//
// The Infohash is the SHA1 hash of the info dictionary
type TorrentFile struct {
	TrackerURLs []string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Files       []File
	Length      int
	Name        string
}

// File contains metadata about the downloaded files, such as length and path
type File struct {
	Length   int
	Path     string
	SHA1Hash string
	MD5Hash  string
}

type bencodeTorrent struct {
	// URL of tracker server to get peers from
	Announce     string     `bencode:"announce"`
	AnnounceList [][]string `bencode:"announce-list"`
	// info is parsed as a RawMessage to ensure that the final info_hash is
	// correct even in the case of the info being a dictionary
	Info bencode.RawMessage `bencode:"info"`
}

// Only Length OR Files will be present per BEP0003
// spec: http://bittorrent.org/beps/bep_0003.html#info-dictionary
type bencodeInfo struct {
	Pieces      []byte `bencode:"pieces"`           // binary blob of all SHA1 hashes of each piece
	PieceLength int    `bencode:"piece length"`     // length in bytes of each piece (must be a power of 2)
	Name        string `bencode:"name"`             // Name of file (or folder if there are multiple files)
	Length      int    `bencode:"length,omitempty"` // Total length of the file (only for single-file torrents)
	Files       []struct {
		Length int      `bencode:"length"` // Length of this file in bytes
		Path   []string `bencode:"path"`   // List of subdirectories; last element is file name
		// Optional fields (custom, not part of BEP0003)
		SHA1Hash string `bencode:"sha1,omitempty"` // Optional: SHA1 hash for this file
		MD5Hash  string `bencode:"md5,omitempty"`  // Optional: MD5 hash for this file
	} `bencode:"files,omitempty"` // Present only for multi-file torrents
}

// New returns a new TorrentFile
//
// If the source is a .torrent file, it will be parse.
//
// If the source is a magnet link, metadata will be extracted from peers
// already in the swarm, then added using TorrentFile.AppendMetadata()
func New(source string) (TorrentFile, error) {
	if strings.HasSuffix(source, ".torrent") {
		return ParseTorrentFile(source)
	} else if strings.HasPrefix(source, "magnet") {
		return ParseMagnetLink(source)
	} else {
		return TorrentFile{}, fmt.Errorf("invalid source: %s", source)
	}
}
