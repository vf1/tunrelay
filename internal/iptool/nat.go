package iptool

import (
	"errors"
	"net"
	"net/netip"
)

var (
	ErrTooSmallIP  = errors.New("too small ip packet")
	ErrTooSmallUdp = errors.New("too small tcp packet")
	ErrTooSmallTcp = errors.New("too small udp packet")
)

func ReplaceAddrs(packet []byte, newSrcPrefix, newDstPrefix netip.Prefix) error {
	if len(packet) < IPHeaderMinSize {
		return ErrTooSmallIP
	}

	var (
		src, dst, newSrc, newDst   [4]byte
		isReplaceSrc, isReplaceDst bool
	)
	if newSrcPrefix.IsValid() {
		src = Src(packet)
		newSrc = calcNewAddr(src, newSrcPrefix)
		isReplaceSrc = true
	}
	if newDstPrefix.IsValid() {
		dst = Dst(packet)
		newDst = calcNewAddr(dst, newDstPrefix)
		isReplaceDst = true
	}

	if isReplaceSrc {
		PutSrc(packet, newSrc)
	}
	if isReplaceDst {
		PutDst(packet, newDst)
	}
	if isReplaceSrc || isReplaceDst {
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
		if isReplaceSrc {
			checksum = RecalcTCPChecksumIP(checksum, src, newSrc)
		}
		if isReplaceDst {
			checksum = RecalcTCPChecksumIP(checksum, dst, newDst)
		}
		PutTCPChecksum(tcpPacket, checksum)

	case ProtocolUdp:
		udpPacket := packet[IPHeaderSize(packet):]
		if len(udpPacket) < UdpChecksummOffset+2 {
			return ErrTooSmallUdp
		}
		checksum := UDPChecksum(udpPacket)
		if checksum != 0 {
			if isReplaceSrc {
				checksum = RecalcTCPChecksumIP(checksum, src, newSrc)
			}
			if isReplaceDst {
				checksum = RecalcTCPChecksumIP(checksum, dst, newDst)
			}
			PutUDPChecksum(udpPacket, checksum)
		}
	}

	return nil
}

func calcNewAddr(oldAddr [4]byte, newAddr netip.Prefix) [4]byte {
	maskPrefix, _ := netip.AddrFrom4([4]byte{255, 255, 255, 255}).Prefix(newAddr.Bits())
	mask := maskPrefix.Masked().Addr().As4()
	addr := newAddr.Masked().Addr().As4()
	for i := range 4 {
		addr[i] |= (oldAddr[i] & (^mask[i]))
	}
	return addr
}

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
