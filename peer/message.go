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
	// Receive and parse the message <length><id><payload>
	// 4 bytes that represent the length of the rest of the message
	lengthBuf := make([]byte, 4)
	_, err := io.ReadFull(p.Conn, lengthBuf)
	if err != nil {
		return message{ID: msgUnknown}, fmt.Errorf("reading message length: %w", err)
	}

	msgLength := binary.BigEndian.Uint32(lengthBuf)
	if msgLength == 0 {
		// keep-alive message
		return message{ID: msgKeepAlive}, nil
	}

	// buffer to contain the rest of the message, 1 byte for the messageID, the
	// rest for the payload
	messageBuf := make([]byte, msgLength)
	_, err = io.ReadFull(p.Conn, messageBuf)
	if err != nil {
		return message{ID: msgUnknown}, fmt.Errorf("reading message payload: %w", err)
	}
	msgID := messageID(messageBuf[0])
	messagePayload := messageBuf[1:]

	// apply side effects to client if applicable
	switch msgID {
	case msgChoke:
		p.Choked = true
	case msgUnchoke:
		p.Choked = false
	case msgHave:
		index := binary.BigEndian.Uint32(messagePayload)
		p.Bitfield.SetPiece(int(index))
	case msgBitfield:
		p.Bitfield = bitfield(messagePayload)
	case msgPort:
		p.DHTPort = int(binary.BigEndian.Uint16(messagePayload))
	}

	return message{
		ID:      msgID,
		Payload: messagePayload,
	}, nil
}
