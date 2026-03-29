package iptool

import (
	"errors"
	"fmt"
	"io"
	"net"
)

var (
	ErrTooSmallUdp = errors.New("too small tcp packet")
	ErrTooSmallTcp = errors.New("too small udp packet")
)

type NAT struct {
	io.ReadWriteCloser
	write NATActions
	read  NATActions
}

type NATActions struct {
	src net.IP
	dst net.IP
}

func NewNAT(rwc io.ReadWriteCloser, writeSrc, writeDst, readSrc, readDst net.IP) *NAT {
	return &NAT{rwc, NATActions{writeSrc, writeDst}, NATActions{readSrc, readDst}}
}

func (nat *NAT) Read(buf []byte) (n int, err error) {
	n, err = nat.ReadWriteCloser.Read(buf)
	if err != nil || nat.read.src == nil && nat.read.dst == nil {
		return
	}
	packet := buf[:n]
	err = DoNAT(packet, nat.read.src, nat.read.dst)
	return
}

func (nat *NAT) Write(packet []byte) (n int, err error) {
	if nat.write.src != nil || nat.write.dst != nil {
		err = DoNAT(packet, nat.write.src, nat.write.dst)
		if err != nil {
			return
		}
	}
	n, err = nat.ReadWriteCloser.Write(packet)
	return
}

func (nat *NAT) Close() error {
	return nat.ReadWriteCloser.Close()
}

func DoNAT(packet []byte, newSrc, newDst net.IP) error {
	if len(packet) < IPHeaderMinSize {
		return fmt.Errorf("ip packet less min size")
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
