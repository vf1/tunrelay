package iptool

import (
	"errors"
	"net"
)

var (
	ErrTooSmallIP  = errors.New("too small ip packet")
	ErrTooSmallUdp = errors.New("too small tcp packet")
	ErrTooSmallTcp = errors.New("too small udp packet")
)

func ReplaceIPs(packet []byte, newSrc, newDst net.IP) error {
	if len(packet) < IPHeaderMinSize {
		return ErrTooSmallIP
	}

	var src, dst [4]byte
	if newSrc != nil {
		src = Src(packet)
		PutSrc(packet, [4]byte(newSrc.To4()))
	}
	if newDst != nil {
		dst = Dst(packet)
		PutDst(packet, [4]byte(newDst.To4()))
	}
	if newSrc != nil || newDst != nil {
		cs := IPChecksum(packet)
		PutIPChecksum(packet, cs)
	}

	protocol := packet[ProtocolOffset]
	switch protocol {
	case ProtocolTcp:
		tcpPacket := packet[IPHeaderSize(packet):]
		if len(tcpPacket) < TCPHeaderMinSize {
			return ErrTooSmallTcp
		}
		checksum := TCPChecksum(tcpPacket)
		if newSrc != nil {
			checksum = RecalcTCPChecksumIP(checksum, src, [4]byte(newSrc.To4()))
		}
		if newDst != nil {
			checksum = RecalcTCPChecksumIP(checksum, dst, [4]byte(newDst.To4()))
		}
		PutTCPChecksum(tcpPacket, checksum)

	case ProtocolUdp:
		udpPacket := packet[IPHeaderSize(packet):]
		if len(udpPacket) < UdpChecksummOffset+2 {
			return ErrTooSmallUdp
		}
		checksum := UDPChecksum(udpPacket)
		if checksum != 0 {
			if newSrc != nil {
				checksum = RecalcTCPChecksumIP(checksum, src, [4]byte(newSrc.To4()))
			}
			if newDst != nil {
				checksum = RecalcTCPChecksumIP(checksum, dst, [4]byte(newDst.To4()))
			}
			PutUDPChecksum(udpPacket, checksum)
		}
	}

	return nil
}
