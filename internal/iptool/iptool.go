package iptool

import (
	"encoding/binary"
	"net"
)

const (
	SrcOffset       = 12
	DstOffset       = 16
	ChecksummOffset = 10
	ProtocolOffset  = 9

	IPHeaderMinSize  = 20
	TCPHeaderMinSize = 20

	ProtocolTcp = 6
	ProtocolUdp = 17

	TcpChecksummOffset = 16
	UdpChecksummOffset = 6
)

func PutSrc(ip []byte, addr [4]byte) {
	copy(ip[SrcOffset:], addr[:])
}

func Src(ip []byte) [4]byte {
	var addr [4]byte
	copy(addr[:], ip[SrcOffset:])
	return addr
}

func SrcIPv4(ip []byte) net.IP {
	ipb := Src(ip)
	return net.IPv4(ipb[0], ipb[1], ipb[2], ipb[3])
}

func PutDst(ip []byte, addr [4]byte) {
	copy(ip[DstOffset:], addr[:])
}

func Dst(ip []byte) [4]byte {
	var addr [4]byte
	copy(addr[:], ip[DstOffset:])
	return addr
}

func DstIPv4(ip []byte) net.IP {
	ipb := Dst(ip)
	return net.IPv4(ipb[0], ipb[1], ipb[2], ipb[3])
}

func PutIPChecksum(ip []byte, cs uint16) {
	binary.BigEndian.PutUint16(ip[ChecksummOffset:], cs)
}

func RecalcTCPChecksumIP(checksum uint16, oldIP, newIP [4]byte) uint16 {
	old1 := uint16(oldIP[0])<<8 | uint16(oldIP[1])
	old2 := uint16(oldIP[2])<<8 | uint16(oldIP[3])

	new1 := uint16(newIP[0])<<8 | uint16(newIP[1])
	new2 := uint16(newIP[2])<<8 | uint16(newIP[3])

	checksum = RecalcTCPChecksum16(checksum, old1, new1)
	checksum = RecalcTCPChecksum16(checksum, old2, new2)

	return checksum
}

func RecalcTCPChecksum16(checksum uint16, oldVal uint16, newVal uint16) uint16 {
	sum := uint32(^checksum) + uint32(^oldVal) + uint32(newVal)

	sum = (sum & 0xffff) + (sum >> 16)
	sum = (sum & 0xffff) + (sum >> 16)

	return ^uint16(sum)
}

func IPHeaderSize(ip []byte) int {
	return int(ip[0]&0x0f) * 4
}

func IPChecksum(ip []byte) uint16 {
	size := IPHeaderSize(ip)

	sum := 0
	for i := 0; i < size-1; i += 2 {
		if i == ChecksummOffset {
			continue
		}
		sum += int(binary.BigEndian.Uint16(ip[i:]))
	}
	if size%2 == 1 {
		sum += int(ip[len(ip)-1]) << 8
	}
	for (sum >> 16) > 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

func PutTCPChecksum(tcp []byte, value uint16) {
	binary.BigEndian.PutUint16(tcp[TcpChecksummOffset:], value)
}

func TCPChecksum(tcp []byte) uint16 {
	return binary.BigEndian.Uint16(tcp[TcpChecksummOffset:])
}

func PutUDPChecksum(udp []byte, value uint16) {
	binary.BigEndian.PutUint16(udp[UdpChecksummOffset:], value)
}

func UDPChecksum(udp []byte) uint16 {
	return binary.BigEndian.Uint16(udp[UdpChecksummOffset:])
}
