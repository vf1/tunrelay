package udpep

import (
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"
	"tunrelay/internal/iptool"
)

type Logger interface {
	Info(msg string, args ...any)
}

var (
	ErrNoPeer      = errors.New("peer not connected")
	ErrWrongPass   = errors.New("wrong password")
	ErrSmallPacket = errors.New("empty packet")
	ErrStalePacket = errors.New("stale packet")
	ErrNotAllowSrc = errors.New("not allow src")
)

const (
	HashSize    = md5.Size
	HeaderSize  = 4 + HashSize
	MaxTimeDiff = 4 // seconds
	UDPTimeout  = 10 * time.Second
)

func pack(b []byte, pass string) (net.Buffers, error) {
	header := make([]byte, HeaderSize)

	timestamp := uint32(time.Now().Unix())
	hash, err := calcHash(pass, b, timestamp)
	if err != nil {
		return nil, err
	}

	binary.BigEndian.PutUint32(header, timestamp)
	copy(header[4:], hash[:])

	return net.Buffers{header, b}, nil
}

func unpack(packet []byte, lookupPassword func(src [4]byte) (string, bool)) ([]byte, [4]byte, error) {
	var src [4]byte
	if len(packet) < HeaderSize {
		return nil, src, ErrSmallPacket
	}

	rtimestamp := binary.BigEndian.Uint32(packet[0:4])
	rhash := [HashSize]byte(packet[4 : 4+HashSize])
	payload := packet[HeaderSize:]

	timestamp := uint32(time.Now().Unix())
	if timestamp-rtimestamp > MaxTimeDiff && rtimestamp-timestamp > MaxTimeDiff {
		return nil, src, ErrStalePacket
	}

	src = iptool.Src(payload)
	pass, found := lookupPassword(src)
	if !found {
		return nil, src, fmt.Errorf("src %s: %w", netip.AddrFrom4(src), ErrNotAllowSrc)
	}

	hash, err := calcHash(pass, payload, rtimestamp)
	if err != nil {
		return nil, src, fmt.Errorf("calc hash: %w", err)
	}
	if rhash != hash {
		return nil, src, ErrWrongPass
	}

	return payload, src, nil
}

func calcHash(pass string, b []byte, timestamp uint32) (hash [16]byte, err error) {
	h := md5.New()
	_, err = h.Write([]byte(pass))
	if err != nil {
		return
	}
	err = binary.Write(h, binary.BigEndian, timestamp)
	if err != nil {
		return
	}
	if len(b) > 64 {
		b = b[:64]
	}
	_, err = h.Write(b)
	if err != nil {
		return
	}
	return [16]byte(h.Sum(nil)), nil
}
