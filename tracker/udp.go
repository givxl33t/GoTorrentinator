package tracker

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"time"
)

func getPeersFromUDPTracker(u *url.URL, infoHash, peerID [20]byte, port int) ([]net.TCPAddr, error) {
	udpClient, err := NewUDPClient(u, infoHash, peerID, port)
	if err != nil {
		return nil, fmt.Errorf("failed to create udp client: %w", err)
	}
	return udpClient.GetPeers()
}

// udpMessageAction is sent in BigEndian
type udpMessageAction uint32

const (
	ConnectAction udpMessageAction = iota
	AnnounceAction
	ScrapeAction
	ErrorAction
)

var actionStrings = map[udpMessageAction]string{
	ConnectAction:  "connect",
	AnnounceAction: "announce",
	ScrapeAction:   "scrape",
	ErrorAction:    "error",
}

func (m udpMessageAction) String() string {
	return actionStrings[m]
}

// UDPClient is an implementation of BEP0015 to locate peers without DHT
type UDPClient struct {
	Conn         *net.UDPConn
	PeerID       [20]byte
	InfoHash     [20]byte
	Port         int
	Peers        []net.TCPAddr
	ConnectionID uint64
}

// NewUDPClient generates a client to a UDP tracker
func NewUDPClient(trackerURL *url.URL, infoHash, peerID [20]byte, port int) (*UDPClient, error) {
	udpAddr, err := net.ResolveUDPAddr(trackerURL.Scheme, trackerURL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve udp address: %w", err)
	}

	udpConn, err := net.DialUDP(trackerURL.Scheme, nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial udp connection: %w", err)
	}

	err = udpConn.SetReadBuffer(4096)
	if err != nil {
		return nil, fmt.Errorf("failed to set read buffer: %w", err)
	}

	return &UDPClient{
		Conn:     udpConn,
		PeerID:   peerID,
		InfoHash: infoHash,
		Port:     port,
	}, nil
}

func (u *UDPClient) GetPeers() ([]net.TCPAddr, error) {
	err := u.connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	err = u.announce()
	if err != nil {
		return nil, fmt.Errorf("failed to announce: %w", err)
	}

	return u.Peers, nil
}

// to initiate message to the UDP Tracker Server to acquire
// Connection ID to use for announce
func (u *UDPClient) connect() error {
	const protocolID = 0x41727101980 // magic number for protocol identification
	transactionID := rand.Uint32()

	connectMsg := make([]byte, 16)
	binary.BigEndian.PutUint64(connectMsg[0:8], uint64(protocolID))
	binary.BigEndian.PutUint32(connectMsg[8:12], uint32(ConnectAction))
	binary.BigEndian.PutUint32(connectMsg[12:16], transactionID)

	u.Conn.SetDeadline(time.Now().Add(3 * time.Second))
	defer u.Conn.SetDeadline(time.Time{}) // clear any deadlines

	_, err := u.Conn.Write(connectMsg)
	if err != nil {
		return fmt.Errorf("failed to write connect message: %w", err)
	}

	// read connect response from server
	resp := make([]byte, 16)
	n, _, _, _, err := u.Conn.ReadMsgUDP(resp, nil)
	if err != nil {
		return fmt.Errorf("failed to read connect response: %w", err)
	}

	if n != 16 {
		return fmt.Errorf("want connect message to be 16 bytes, got %d bytes", err)
	}
	connectResp, err := u.parseUDPResponse(transactionID, ConnectAction, resp)
	if err != nil {
		return fmt.Errorf("failed to parse connect response: %w", err)
	}

	u.ConnectionID = binary.BigEndian.Uint64(connectResp)

	return nil
}

func (u *UDPClient) announce() error {
	announceMsg := make([]byte, 98)

	binary.BigEndian.PutUint64(announceMsg[0:8], u.ConnectionID)
	binary.BigEndian.PutUint32(announceMsg[8:12], uint32(AnnounceAction))
	transactionID := rand.Uint32()
	binary.BigEndian.PutUint32(announceMsg[12:16], transactionID)
	copy(announceMsg[16:36], u.InfoHash[:])
	copy(announceMsg[36:56], u.PeerID[:])

	binary.BigEndian.PutUint64(announceMsg[56:64], 0) // downloaded
	binary.BigEndian.PutUint64(announceMsg[64:72], 0) // left, unknown to magnet links
	binary.BigEndian.PutUint64(announceMsg[72:80], 0) // uploaded

	binary.BigEndian.PutUint32(announceMsg[80:84], 0) // event 0:none; 1:completed; 2:started; 3:stopped
	binary.BigEndian.PutUint32(announceMsg[84:88], 0) // IP address, default

	binary.BigEndian.PutUint32(announceMsg[88:92], rand.Uint32()) // key - for tracker statistics

	neg1 := -1
	binary.BigEndian.PutUint32(announceMsg[92:96], uint32(neg1))   // num_want
	binary.BigEndian.PutUint16(announceMsg[96:98], uint16(u.Port)) // port

	u.Conn.SetDeadline(time.Now().Add(5 * time.Second))
	defer u.Conn.SetDeadline(time.Time{}) // clear any deadlines

	_, err := u.Conn.Write(announceMsg)
	if err != nil {
		return fmt.Errorf("failed to write announce message: %w", err)
	}

	// allocate enough buffer for UDP response
	resp := make([]byte, 4096)
	n, err := u.Conn.Read(resp)
	if err != nil {
		return fmt.Errorf("failed to read announce response: %w", err)
	}
	resp = resp[:n]
	announceResp, err := u.parseUDPResponse(transactionID, AnnounceAction, resp)
	if err != nil {
		return fmt.Errorf("failed to parse announce response: %w", err)
	}

	var peers []net.TCPAddr
	for i := 12; i < len(announceResp); i += 6 {
		// parse 6 byes for peer's ip (4 bytes) and port (2 bytes)
		peers = append(peers, net.TCPAddr{
			IP:   net.IP(announceResp[i : i+4]),
			Port: int(binary.BigEndian.Uint16(announceResp[i+4 : i+6])),
		})
	}

	if len(peers) == 0 {
		return fmt.Errorf("no peers found in announce response")
	}

	u.Peers = peers

	return nil
}

func (u *UDPClient) parseUDPResponse(wantTransactionID uint32, wantAction udpMessageAction, resp []byte) ([]byte, error) {
	if len(resp) < 8 {
		return nil, fmt.Errorf("want response to be at least 8 bytes, got %d bytes", len(resp))
	}

	respTransactionID := binary.BigEndian.Uint32(resp[4:8])
	if respTransactionID != wantTransactionID {
		return nil, fmt.Errorf("want transaction id %d, got %d", wantTransactionID, respTransactionID)
	}

	action := binary.BigEndian.Uint32(resp[0:4])
	if udpMessageAction(action) == ErrorAction {
		// return an error than includes the message
		errorText := string(resp[8:])
		return nil, fmt.Errorf("error response: %s", errorText)
	}
	if udpMessageAction(action) != wantAction {
		return nil, fmt.Errorf("want action %s, got %s", wantAction, udpMessageAction(action))
	}

	return resp[8:], nil
}
