package peer

import (
	"bytes"
	"fmt"
	"io"
)

// handshake completes the entire handshake process with the underlying peer
func (p *Client) handshake(infoHash, peerId [20]byte) error {
	const protocol = "BitTorrent protocol"
	var buf bytes.Buffer
	buf.WriteByte(byte(len(protocol)))
	buf.WriteString(protocol)

	extensionBytes := make([]byte, 8)
	// support BEP0010 Extension Protocol
	// "20th bit from the right" = reserved_byte[5] & 0x10 (00010000 in binary)
	extensionBytes[5] |= 0x10
	extensionBytes[7] |= 1 // support BEP0005 DHT
	buf.Write(extensionBytes)

	// write info hash and peer id
	buf.Write(infoHash[:])
	buf.Write(peerId[:])

	// send handshake
	_, err := p.Conn.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("writing handshake: %w", err)
	}

	// read the protocol length
	lengthBuf := make([]byte, 1)
	_, err = io.ReadFull(p.Conn, lengthBuf)
	if err != nil {
		return err
	}
	lengthProtocol := int(lengthBuf[0])
	if lengthProtocol != 19 {
		return fmt.Errorf("invalid protocol length: %d", lengthProtocol)
	}

	// read the handshake buffer
	handShakeBuf := make([]byte, lengthProtocol+48)
	_, err = io.ReadFull(p.Conn, handShakeBuf)
	if err != nil {
		return fmt.Errorf("reading handshake: %w", err)
	}

	// parse handshake details into handshake
	responseProtocol := string(handShakeBuf[:lengthProtocol])
	if responseProtocol != protocol {
		return fmt.Errorf("invalid protocol: %s", responseProtocol)
	}

	// check reserved bytes for feature support
	read := lengthProtocol
	var responseExtensionBytes [8]byte
	read += copy(responseExtensionBytes[:], handShakeBuf[read:read+8])
	if responseExtensionBytes[7]|1 != 0 {
		p.DHTSupport = true
	}

	// check for extension protocol support
	if responseExtensionBytes[5]|0x10 != 0 {
		p.ExtensionSupport = true
	}

	var responseInfoHash [20]byte
	read += copy(responseInfoHash[:], handShakeBuf[read:read+20])
	copy(p.PeerID[:], handShakeBuf[read:])

	if !bytes.Equal(responseInfoHash[:], infoHash[:]) {
		return fmt.Errorf("invalid info hash: %x", responseInfoHash)
	}

	return nil
}
