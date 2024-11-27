package peer

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

type Client struct {
	Conn              net.Conn // connection to the peer
	Choked            bool
	Bitfield          bitfield    // tracks which pieces the peer has
	PeerID            [20]byte    // id received from the tracker in the original protocol
	DHTSupport        bool        // DHT support (BEP0005)
	DHTPort           int         // port for peer's DHT node
	Address           net.TCPAddr // storedd for easy access to iP address for DHT
	ExtensionSupport  bool
	ExtensionMetadata struct { // essenstial magnet link properties in handshake
		messageID    int
		metadataSize int
	}
}

func NewClient(addr net.TCPAddr, infoHash, peerID [20]byte) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr.String(), 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dialing peer: %w", err)
	}

	client := &Client{
		Conn:    conn,
		Address: addr,
		Choked:  true,
	}

	client.Conn.SetDeadline(time.Now().Add(3 * time.Second))
	defer client.Conn.SetDeadline(time.Time{})

	err = client.handshake(infoHash, peerID)
	if err != nil {
		return nil, fmt.Errorf("sending handshake: %w", err)
	}

	if client.ExtensionSupport {
		client.Conn.SetDeadline(time.Now().Add(3 * time.Second))
		// receive extension message
		err = client.receiveExtendedHandshake()
		if err != nil {
			return nil, fmt.Errorf("receiving extension handshake: %w", err)
		}
	}

	// receive bitfield message IF it hasn't been received already
	if len(client.Bitfield) == 0 {
		client.Conn.SetDeadline(time.Now().Add(3 * time.Second))
		_, err = client.receiveMessage()
		if err != nil {
			return nil, fmt.Errorf("receiving bitfield: %w", err)
		}
		if len(client.Bitfield) == 0 {
			return nil, fmt.Errorf("received empty bitfield")
		}
	}

	if client.DHTSupport {
		client.Conn.SetDeadline(time.Now().Add(5 * time.Second))
		// allow 50 retries to account for client that sends other messages first
		// loop will likely exit successfullyy or an i/o timeout on receiveMessage()
		for i := 0; i < 50 && client.DHTPort == 0; i++ {
			_, err := client.receiveMessage()
			if err != nil {
				// this shouldn't invalidate the peer connection, just break
				break
			}
		}
	}

	client.Conn.SetDeadline(time.Now().Add(3 * time.Second))
	// send unchoke and interested message so the peer is ready for requests
	err = client.sendMessage(msgUnchoke, nil)
	if err != nil {
		return nil, fmt.Errorf("sending unchoke: %w", err)
	}
	err = client.sendMessage(msgInterested, nil)
	if err != nil {
		return nil, fmt.Errorf("sending interested: %w", err)
	}

	return client, nil
}

// Address returns the address of the peer
func (p *Client) Addr() net.Addr {
	return p.Conn.RemoteAddr()
}

func (p *Client) Close() error {
	return p.Conn.Close()
}

var ErrNotInBitfield = errors.New("client does not have piece")

func (p *Client) GetPiece(index, length int, hash [20]byte) ([]byte, error) {
	if !p.Bitfield.HasPiece(index) {
		return nil, ErrNotInBitfield
	}

	// set deadline to handle stuck peer
	p.Conn.SetDeadline(time.Now().Add(15 * time.Second))
	defer p.Conn.SetDeadline(time.Time{})

	const maxBlockSize = 16384 // 16KiB
	const maxBacklog = 10

	var requested, received, backlog int
	pieceBuf := make([]byte, length)
	for received < length {
		for !p.Choked && backlog < maxBacklog && requested < length {
			payload := make([]byte, 12)
			binary.BigEndian.PutUint32(payload[0:4], uint32(index))
			binary.BigEndian.PutUint32(payload[4:8], uint32(requested))
			blockSize := maxBlockSize
			if requested+blockSize > length {
				blockSize = length - requested
			}
			binary.BigEndian.PutUint32(payload[8:12], uint32(blockSize))

			err := p.sendMessage(msgRequest, payload)
			if err != nil {
				return nil, fmt.Errorf("sending request: %w", err)
			}

			requested += blockSize
			backlog++
		}

		if p.Choked {
			err := p.sendMessage(msgUnchoke, nil)
			if err != nil {
				return nil, fmt.Errorf("sending unchoke: %w", err)
			}
		}

		// receiving blocks
		msg, err := p.receiveMessage()
		if err != nil {
			return nil, fmt.Errorf("receiving message: %w", err)
		}
		if msg.ID != msgPiece {
			continue
		}

		// piece format: <index, uint32><begin offset, uint32><data []byte>
		responseIndex := binary.BigEndian.Uint32(msg.Payload[0:4])
		if responseIndex != uint32(index) {
			continue
		}

		begin := binary.BigEndian.Uint32(msg.Payload[4:8])
		blockData := msg.Payload[8:]

		// write block data to piece buffer
		write := copy(pieceBuf[begin:], blockData[:])

		received += write
		if write != 0 {
			backlog--
		}
	}

	// check integrity
	pieceHash := sha1.Sum(pieceBuf)
	if !bytes.Equal(pieceHash[:], hash[:]) {
		// disconnect from peer if hash doesn't match
		return nil, fmt.Errorf("failed integrity check from %s", p.Conn.RemoteAddr())
	}

	// inform peer we have received the piece
	havePayload := make([]byte, 4)
	binary.BigEndian.PutUint32(havePayload, uint32(index))
	p.sendMessage(msgHave, havePayload)

	return pieceBuf, nil
}
