package peer

import (
	"encoding/binary"
	"fmt"
	"io"
)

// messageID are the types of messages a peer can send
type messageID uint8

type message struct {
	ID      messageID
	Payload []byte
}

// use iota for incrementing messageID
const (
	msgChoke messageID = iota
	msgUnchoke
	msgInterested
	msgNotInterested
	msgHave
	msgBitfield
	msgRequest
	msgPiece
	msgCancel
	msgPort // 0x09 for DHT (BEP0005)

	msgExtended messageID = 20 // BEP0010 extension messages

	//additional ids
	msgKeepAlive messageID = 254
	msgUnknown   messageID = 255
)

var messageIDStrings = map[messageID]string{
	msgChoke:         "choke",
	msgUnchoke:       "unchoke",
	msgInterested:    "interested",
	msgNotInterested: "not interested",
	msgHave:          "have",
	msgBitfield:      "bitfield",
	msgRequest:       "request",
	msgPiece:         "piece",
	msgCancel:        "cancel",
	msgPort:          "port",
	msgExtended:      "extended",
	msgKeepAlive:     "keep alive",
	msgUnknown:       "unknown",
}

func (m messageID) String() string {
	return messageIDStrings[m]
}

// sendMessage serializes and sends a message id and payload to the peer
func (p *Client) sendMessage(id messageID, payload []byte) error {
	length := uint32(len(payload) + 1) // +1 for ID
	message := make([]byte, length+4)  // +4 to fit <length> at start of message
	binary.BigEndian.PutUint32(message[0:4], length)
	message[4] = byte(id)

	// add in payload if not a keep alive message
	if id != msgKeepAlive {
		copy(message[5:], payload)
	}

	_, err := p.Conn.Write(message)
	if err != nil {
		return fmt.Errorf("writing message: %w", err)
	}

	return nil
}

// receiveMessage reads a message from the peer
func (p *Client) receiveMessage() (message, error) {
	// receive and parse message length
	lengthBuf := make([]byte, 4)
	_, err := io.ReadFull(p.Conn, lengthBuf)
	if err != nil {
		return message{ID: msgUnknown}, fmt.Errorf("reading message length: %w", err)
	}

	messageLength := binary.BigEndian.Uint32(lengthBuf)
	if messageLength == 0 {
		// keep-alive message
		return message{ID: msgKeepAlive}, nil
	}

	// buffer to contain the rest of the message, 1 byte for the id, rest for payload
	messageBuf := make([]byte, messageLength)
	_, err = io.ReadFull(p.Conn, messageBuf)
	if err != nil {
		return message{ID: msgUnknown}, fmt.Errorf("reading message: %w", err)
	}
	msgID := messageID(messageBuf[0])
	msgPayload := messageBuf[1:]

	// apply sidde effects
	switch msgID {
	case msgHave:
		index := binary.BigEndian.Uint32(msgPayload)
		p.Bitfield.SetPiece(int(index))
	case msgChoke:
		p.Choked = true
	case msgUnchoke:
		p.Choked = false
	case msgBitfield:
		p.Bitfield = bitfield(msgPayload)
	case msgPort:
		p.DHTPort = int(binary.BigEndian.Uint16(msgPayload))
	}

	return message{
		ID:      msgID,
		Payload: msgPayload,
	}, nil
}
