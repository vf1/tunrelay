package iptool

import (
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"testing"
)

func makeIPv4Header(src, dst [4]byte, protocol byte, totalLen uint16) []byte {
	h := make([]byte, 20)
	h[0] = 0x45
	binary.BigEndian.PutUint16(h[2:], totalLen)
	h[8] = 64
	h[9] = protocol
	copy(h[12:16], src[:])
	copy(h[16:20], dst[:])
	return h
}

func makeTCPPacket(src, dst [4]byte, srcPort, dstPort uint16, payload []byte) []byte {
	ipHdr := makeIPv4Header(src, dst, ProtocolTcp, uint16(20+20+len(payload)))
	tcpHdr := make([]byte, 20)
	binary.BigEndian.PutUint16(tcpHdr[0:], srcPort)
	binary.BigEndian.PutUint16(tcpHdr[2:], dstPort)
	tcpHdr[12] = 0x50
	packet := append(ipHdr, tcpHdr...)
	packet = append(packet, payload...)
	cs := IPChecksum(packet)
	PutIPChecksum(packet, cs)
	tcpCs := transportChecksumFull(packet, ProtocolTcp)
	PutTCPChecksum(packet[20:], tcpCs)
	return packet
}

func makeUDPPacket(src, dst [4]byte, srcPort, dstPort uint16, payload []byte) []byte {
	ipHdr := makeIPv4Header(src, dst, ProtocolUdp, uint16(20+8+len(payload)))
	udpHdr := make([]byte, 8)
	binary.BigEndian.PutUint16(udpHdr[0:], srcPort)
	binary.BigEndian.PutUint16(udpHdr[2:], dstPort)
	binary.BigEndian.PutUint16(udpHdr[4:], uint16(8+len(payload)))
	packet := append(ipHdr, udpHdr...)
	packet = append(packet, payload...)
	cs := IPChecksum(packet)
	PutIPChecksum(packet, cs)
	udpCs := transportChecksumFull(packet, ProtocolUdp)
	PutUDPChecksum(packet[20:], udpCs)
	return packet
}

func transportChecksumFull(packet []byte, protocol byte) uint16 {
	ipHdrLen := IPHeaderSize(packet)
	src := Src(packet)
	dst := Dst(packet)
	seg := packet[ipHdrLen:]
	segLen := len(seg)

	sum := uint32(0)
	sum += uint32(src[0])<<8 | uint32(src[1])
	sum += uint32(src[2])<<8 | uint32(src[3])
	sum += uint32(dst[0])<<8 | uint32(dst[1])
	sum += uint32(dst[2])<<8 | uint32(dst[3])
	sum += uint32(protocol)
	sum += uint32(segLen)

	var csOff int
	if protocol == ProtocolTcp {
		csOff = TcpChecksummOffset
	} else {
		csOff = UdpChecksummOffset
	}

	for i := 0; i < segLen-1; i += 2 {
		if i == csOff {
			continue
		}
		sum += uint32(binary.BigEndian.Uint16(seg[i:]))
	}
	if segLen%2 == 1 {
		sum += uint32(seg[segLen-1]) << 8
	}

	for sum>>16 > 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

func TestCanGetVersion(t *testing.T) {
	if CanGetVersion(nil) {
		t.Fatal("expected false for nil")
	}
	if CanGetVersion([]byte{}) {
		t.Fatal("expected false for empty")
	}
	if !CanGetVersion([]byte{0x45}) {
		t.Fatal("expected true for non-empty")
	}
}

func TestVersion(t *testing.T) {
	if v := Version([]byte{0x45}); v != 4 {
		t.Fatalf("IPv4: expected 4, got %d", v)
	}
	if v := Version([]byte{0x60}); v != 6 {
		t.Fatalf("IPv6: expected 6, got %d", v)
	}
}

func TestSrc(t *testing.T) {
	hdr := makeIPv4Header([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, ProtocolTcp, 40)
	if src := Src(hdr); src != [4]byte{10, 0, 0, 1} {
		t.Fatalf("expected 10.0.0.1, got %v", src)
	}
}

func TestDst(t *testing.T) {
	hdr := makeIPv4Header([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, ProtocolTcp, 40)
	if dst := Dst(hdr); dst != [4]byte{10, 0, 0, 2} {
		t.Fatalf("expected 10.0.0.2, got %v", dst)
	}
}

func TestPutSrc(t *testing.T) {
	hdr := makeIPv4Header([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, ProtocolTcp, 40)
	PutSrc(hdr, [4]byte{192, 168, 1, 1})
	if src := Src(hdr); src != [4]byte{192, 168, 1, 1} {
		t.Fatalf("expected 192.168.1.1, got %v", src)
	}
}

func TestPutDst(t *testing.T) {
	hdr := makeIPv4Header([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, ProtocolTcp, 40)
	PutDst(hdr, [4]byte{192, 168, 1, 2})
	if dst := Dst(hdr); dst != [4]byte{192, 168, 1, 2} {
		t.Fatalf("expected 192.168.1.2, got %v", dst)
	}
}

func TestSrcIPv4(t *testing.T) {
	hdr := makeIPv4Header([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, ProtocolTcp, 40)
	if ip := SrcIPv4(hdr); !ip.Equal(net.IPv4(10, 0, 0, 1)) {
		t.Fatalf("expected 10.0.0.1, got %v", ip)
	}
}

func TestDstIPv4(t *testing.T) {
	hdr := makeIPv4Header([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, ProtocolTcp, 40)
	if ip := DstIPv4(hdr); !ip.Equal(net.IPv4(10, 0, 0, 2)) {
		t.Fatalf("expected 10.0.0.2, got %v", ip)
	}
}

func TestIPHeaderSize(t *testing.T) {
	hdr := makeIPv4Header([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, ProtocolTcp, 40)
	if s := IPHeaderSize(hdr); s != 20 {
		t.Fatalf("IHL=5: expected 20, got %d", s)
	}
	hdr[0] = 0x46
	if s := IPHeaderSize(hdr); s != 24 {
		t.Fatalf("IHL=6: expected 24, got %d", s)
	}
}

func TestIPChecksum(t *testing.T) {
	hdr := makeIPv4Header([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, ProtocolTcp, 40)
	cs := IPChecksum(hdr)
	PutIPChecksum(hdr, cs)
	if IPChecksum(hdr) != cs {
		t.Fatalf("checksum self-verify failed: computed=0x%04X, stored=0x%04X", IPChecksum(hdr), cs)
	}
}

func TestIPChecksumKnownValue(t *testing.T) {
	hdr := makeIPv4Header([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, ProtocolTcp, 40)
	cs := IPChecksum(hdr)
	if cs != 0x66CE {
		t.Fatalf("expected 0x66CE, got 0x%04X", cs)
	}
}

func TestPutIPChecksum(t *testing.T) {
	hdr := makeIPv4Header([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, ProtocolTcp, 40)
	PutIPChecksum(hdr, 0x1234)
	if v := binary.BigEndian.Uint16(hdr[ChecksummOffset:]); v != 0x1234 {
		t.Fatalf("expected 0x1234, got 0x%04X", v)
	}
}

func TestTCPChecksum(t *testing.T) {
	tcp := make([]byte, 20)
	binary.BigEndian.PutUint16(tcp[TcpChecksummOffset:], 0xABCD)
	if v := TCPChecksum(tcp); v != 0xABCD {
		t.Fatalf("expected 0xABCD, got 0x%04X", v)
	}
}

func TestPutTCPChecksum(t *testing.T) {
	tcp := make([]byte, 20)
	PutTCPChecksum(tcp, 0xBEEF)
	if v := binary.BigEndian.Uint16(tcp[TcpChecksummOffset:]); v != 0xBEEF {
		t.Fatalf("expected 0xBEEF, got 0x%04X", v)
	}
}

func TestUDPChecksum(t *testing.T) {
	udp := make([]byte, 8)
	binary.BigEndian.PutUint16(udp[UdpChecksummOffset:], 0xABCD)
	if v := UDPChecksum(udp); v != 0xABCD {
		t.Fatalf("expected 0xABCD, got 0x%04X", v)
	}
}

func TestPutUDPChecksum(t *testing.T) {
	udp := make([]byte, 8)
	PutUDPChecksum(udp, 0xBEEF)
	if v := binary.BigEndian.Uint16(udp[UdpChecksummOffset:]); v != 0xBEEF {
		t.Fatalf("expected 0xBEEF, got 0x%04X", v)
	}
}

func TestRecalcTCPChecksum16(t *testing.T) {
	tests := []struct {
		name     string
		checksum uint16
		oldVal   uint16
		newVal   uint16
		expected uint16
	}{
		{"basic", 0x1234, 0x5678, 0x9ABC, 0xCDEF},
		{"noChange", 0x1234, 0x5678, 0x5678, 0x1234},
		{"zeroOld", 0x1234, 0x0000, 0x5678, 0xBBBB},
		{"allMax", 0xFFFF, 0xFFFF, 0x0000, 0xFFFF},
		{"allZero", 0x0000, 0x0000, 0x0000, 0x0000},
		{"doubleCarry", 0xFFFF, 0x0000, 0x0001, 0xFFFE},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RecalcTCPChecksum16(tt.checksum, tt.oldVal, tt.newVal)
			if result != tt.expected {
				t.Fatalf("expected 0x%04X, got 0x%04X", tt.expected, result)
			}
		})
	}
}

func TestRecalcTCPChecksumIP(t *testing.T) {
	oldIP := [4]byte{10, 0, 0, 1}
	newIP := [4]byte{10, 0, 0, 2}
	result := RecalcTCPChecksumIP(0x1234, oldIP, newIP)
	if result != 0x1233 {
		t.Fatalf("expected 0x1233, got 0x%04X", result)
	}
}

func TestRecalcTCPChecksumIPFullChange(t *testing.T) {
	oldIP := [4]byte{10, 0, 0, 1}
	newIP := [4]byte{192, 168, 1, 1}
	result := RecalcTCPChecksumIP(0x0000, oldIP, newIP)
	expected := RecalcTCPChecksum16(
		RecalcTCPChecksum16(0x0000, 0x0A00, 0xC0A8),
		0x0001, 0x0101,
	)
	if result != expected {
		t.Fatalf("expected 0x%04X, got 0x%04X", expected, result)
	}
}

func TestCalcNewAddr(t *testing.T) {
	tests := []struct {
		name      string
		oldAddr   [4]byte
		newPrefix netip.Prefix
		expected  [4]byte
	}{
		{
			"fullReplace",
			[4]byte{10, 0, 0, 5},
			netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 168, 1, 0}), 24),
			[4]byte{192, 168, 1, 5},
		},
		{
			"partialPreserve",
			[4]byte{10, 0, 0, 5},
			netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, 0, 0}), 16),
			[4]byte{10, 0, 0, 5},
		},
		{
			"hostOnly",
			[4]byte{172, 16, 5, 10},
			netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 168, 0, 0}), 16),
			[4]byte{192, 168, 5, 10},
		},
		{
			"singleBitPrefix",
			[4]byte{128, 0, 0, 1},
			netip.PrefixFrom(netip.AddrFrom4([4]byte{0, 0, 0, 0}), 1),
			[4]byte{0, 0, 0, 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calcNewAddr(tt.oldAddr, tt.newPrefix)
			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestReplaceAddrsTCP(t *testing.T) {
	pkt := makeTCPPacket(
		[4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2},
		12345, 80,
		[]byte("hello"),
	)

	newSrcPrefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 168, 1, 0}), 24)
	newDstPrefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{172, 16, 0, 0}), 16)

	err := ReplaceAddrs(pkt, newSrcPrefix, newDstPrefix)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if src := Src(pkt); src != [4]byte{192, 168, 1, 1} {
		t.Fatalf("src: expected 192.168.1.1, got %v", src)
	}
	if dst := Dst(pkt); dst != [4]byte{172, 16, 0, 2} {
		t.Fatalf("dst: expected 172.16.0.2, got %v", dst)
	}

	ipCs := IPChecksum(pkt)
	storedIpCs := binary.BigEndian.Uint16(pkt[ChecksummOffset:])
	if ipCs != storedIpCs {
		t.Fatalf("IP checksum: computed=0x%04X, stored=0x%04X", ipCs, storedIpCs)
	}

	storedTcpCs := TCPChecksum(pkt[IPHeaderSize(pkt):])
	expectedTcpCs := transportChecksumFull(pkt, ProtocolTcp)
	if storedTcpCs != expectedTcpCs {
		t.Fatalf("TCP checksum: stored=0x%04X, expected=0x%04X", storedTcpCs, expectedTcpCs)
	}
}

func TestReplaceAddrsTCPSrcOnly(t *testing.T) {
	pkt := makeTCPPacket(
		[4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2},
		12345, 80,
		[]byte("hello"),
	)
	origDst := Dst(pkt)

	newSrcPrefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 168, 1, 0}), 24)

	err := ReplaceAddrs(pkt, newSrcPrefix, netip.Prefix{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if src := Src(pkt); src != [4]byte{192, 168, 1, 1} {
		t.Fatalf("src: expected 192.168.1.1, got %v", src)
	}
	if dst := Dst(pkt); dst != origDst {
		t.Fatal("dst should not change")
	}

	storedTcpCs := TCPChecksum(pkt[IPHeaderSize(pkt):])
	expectedTcpCs := transportChecksumFull(pkt, ProtocolTcp)
	if storedTcpCs != expectedTcpCs {
		t.Fatalf("TCP checksum: stored=0x%04X, expected=0x%04X", storedTcpCs, expectedTcpCs)
	}
}

func TestReplaceAddrsTCPDstOnly(t *testing.T) {
	pkt := makeTCPPacket(
		[4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2},
		12345, 80,
		[]byte("hello"),
	)
	origSrc := Src(pkt)

	newDstPrefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{172, 16, 0, 0}), 16)

	err := ReplaceAddrs(pkt, netip.Prefix{}, newDstPrefix)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if src := Src(pkt); src != origSrc {
		t.Fatal("src should not change")
	}
	if dst := Dst(pkt); dst != [4]byte{172, 16, 0, 2} {
		t.Fatalf("dst: expected 172.16.0.2, got %v", dst)
	}

	storedTcpCs := TCPChecksum(pkt[IPHeaderSize(pkt):])
	expectedTcpCs := transportChecksumFull(pkt, ProtocolTcp)
	if storedTcpCs != expectedTcpCs {
		t.Fatalf("TCP checksum: stored=0x%04X, expected=0x%04X", storedTcpCs, expectedTcpCs)
	}
}

func TestReplaceAddrsUDP(t *testing.T) {
	pkt := makeUDPPacket(
		[4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2},
		12345, 53,
		[]byte("data"),
	)

	newSrcPrefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 168, 1, 0}), 24)
	newDstPrefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{172, 16, 0, 0}), 16)

	err := ReplaceAddrs(pkt, newSrcPrefix, newDstPrefix)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if src := Src(pkt); src != [4]byte{192, 168, 1, 1} {
		t.Fatalf("src: expected 192.168.1.1, got %v", src)
	}
	if dst := Dst(pkt); dst != [4]byte{172, 16, 0, 2} {
		t.Fatalf("dst: expected 172.16.0.2, got %v", dst)
	}

	storedUdpCs := UDPChecksum(pkt[IPHeaderSize(pkt):])
	expectedUdpCs := transportChecksumFull(pkt, ProtocolUdp)
	if storedUdpCs != expectedUdpCs {
		t.Fatalf("UDP checksum: stored=0x%04X, expected=0x%04X", storedUdpCs, expectedUdpCs)
	}
}

func TestReplaceAddrsUDPZeroChecksum(t *testing.T) {
	pkt := makeUDPPacket(
		[4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2},
		12345, 53,
		[]byte("data"),
	)
	PutUDPChecksum(pkt[20:], 0)

	newSrcPrefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 168, 1, 0}), 24)

	err := ReplaceAddrs(pkt, newSrcPrefix, netip.Prefix{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if udpCs := UDPChecksum(pkt[20:]); udpCs != 0 {
		t.Fatalf("UDP checksum should stay 0, got 0x%04X", udpCs)
	}
}

func TestReplaceAddrsNoChange(t *testing.T) {
	pkt := makeTCPPacket(
		[4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2},
		12345, 80,
		[]byte("hello"),
	)
	origSrc := Src(pkt)
	origDst := Dst(pkt)
	origIpCs := binary.BigEndian.Uint16(pkt[ChecksummOffset:])
	origTcpCs := TCPChecksum(pkt[20:])

	err := ReplaceAddrs(pkt, netip.Prefix{}, netip.Prefix{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if src := Src(pkt); src != origSrc {
		t.Fatal("src should not change")
	}
	if dst := Dst(pkt); dst != origDst {
		t.Fatal("dst should not change")
	}
	if cs := binary.BigEndian.Uint16(pkt[ChecksummOffset:]); cs != origIpCs {
		t.Fatal("IP checksum should not change")
	}
	if cs := TCPChecksum(pkt[20:]); cs != origTcpCs {
		t.Fatal("TCP checksum should not change")
	}
}

func TestReplaceAddrsTooSmallIP(t *testing.T) {
	err := ReplaceAddrs(make([]byte, 19), netip.Prefix{}, netip.Prefix{})
	if !errors.Is(err, ErrTooSmallIP) {
		t.Fatalf("expected ErrTooSmallIP, got %v", err)
	}
}

func TestReplaceAddrsTooSmallTCP(t *testing.T) {
	pkt := makeIPv4Header([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, ProtocolTcp, 39)
	pkt = append(pkt, make([]byte, 19)...)

	err := ReplaceAddrs(pkt,
		netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 168, 1, 0}), 24),
		netip.Prefix{},
	)
	if !errors.Is(err, ErrTooSmallTcp) {
		t.Fatalf("expected ErrTooSmallTcp, got %v", err)
	}
}

func TestReplaceAddrsTooSmallUDP(t *testing.T) {
	pkt := makeIPv4Header([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, ProtocolUdp, 27)
	pkt = append(pkt, make([]byte, 7)...)

	err := ReplaceAddrs(pkt,
		netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 168, 1, 0}), 24),
		netip.Prefix{},
	)
	if !errors.Is(err, ErrTooSmallUdp) {
		t.Fatalf("expected ErrTooSmallUdp, got %v", err)
	}
}

func TestReplaceIPs(t *testing.T) {
	pkt := makeTCPPacket(
		[4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2},
		12345, 80,
		[]byte("hello"),
	)

	err := ReplaceIPs(pkt, net.IPv4(192, 168, 1, 1), net.IPv4(172, 16, 0, 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if src := Src(pkt); src != [4]byte{192, 168, 1, 1} {
		t.Fatalf("src: expected 192.168.1.1, got %v", src)
	}
	if dst := Dst(pkt); dst != [4]byte{172, 16, 0, 1} {
		t.Fatalf("dst: expected 172.16.0.1, got %v", dst)
	}
}

func TestReplaceIPsNilSrc(t *testing.T) {
	pkt := makeTCPPacket(
		[4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2},
		12345, 80,
		[]byte("hello"),
	)

	err := ReplaceIPs(pkt, nil, net.IPv4(172, 16, 0, 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if src := Src(pkt); src != [4]byte{10, 0, 0, 1} {
		t.Fatalf("src should not change: got %v", src)
	}
	if dst := Dst(pkt); dst != [4]byte{172, 16, 0, 1} {
		t.Fatalf("dst: expected 172.16.0.1, got %v", dst)
	}
}

func TestReplaceIPsNilDst(t *testing.T) {
	pkt := makeTCPPacket(
		[4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2},
		12345, 80,
		[]byte("hello"),
	)

	err := ReplaceIPs(pkt, net.IPv4(192, 168, 1, 1), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if src := Src(pkt); src != [4]byte{192, 168, 1, 1} {
		t.Fatalf("src: expected 192.168.1.1, got %v", src)
	}
	if dst := Dst(pkt); dst != [4]byte{10, 0, 0, 2} {
		t.Fatalf("dst should not change: got %v", dst)
	}
}
