//go:build windows

package pinger

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"
)

var (
	iphlpapi       = syscall.NewLazyDLL("iphlpapi.dll")
	procIcmpCreate = iphlpapi.NewProc("IcmpCreateFile")
	procIcmpSend   = iphlpapi.NewProc("IcmpSendEcho")
	procIcmpClose  = iphlpapi.NewProc("IcmpCloseHandle")
)

type windowsPinger struct{}

func newPlatformPinger() Pinger {
	return &windowsPinger{}
}

// icmpEchoReply соответствует структуре ICMP_ECHO_REPLY из Win32 API
type icmpEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       struct {
		Ttl         uint8
		Tos         uint8
		Flags       uint8
		OptionsSize uint8
		OptionsData uintptr
	}
}

// Ping выполняет отправку эхо-запроса через Win32 API IcmpSendEcho (не требует прав администратора)
func (p *windowsPinger) Ping(target string, timeout time.Duration) PingResult {
	res := PingResult{
		Target: target,
		Status: "error",
	}

	// Разрешаем DNS или IP
	ipAddr, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		res.ErrorMsg = fmt.Sprintf("ошибка разрешения хоста %s: %v", target, err)
		return res
	}
	res.IP = ipAddr.IP.String()

	ip4 := ipAddr.IP.To4()
	if ip4 == nil {
		res.ErrorMsg = "поддерживается только IPv4 в нативном Windows ICMP API"
		return res
	}

	// В IcmpSendEcho IPAddr ожидается в сетевом порядке байт (в памяти 192.168.1.1 -> [192, 168, 1, 1],
	// что при чтении LittleEndian дает uint32 в нужном формате памяти)
	destAddr := binary.LittleEndian.Uint32(ip4)

	handle, _, errCall := procIcmpCreate.Call()
	if handle == 0 || handle == uintptr(syscall.InvalidHandle) {
		res.ErrorMsg = fmt.Sprintf("ошибка IcmpCreateFile: %v", errCall)
		return res
	}
	defer procIcmpClose.Call(handle)

	sendData := []byte("PING_MONITOR_TEST_DATA_32_BYTES!")
	replySize := unsafe.Sizeof(icmpEchoReply{}) + uintptr(len(sendData)) + 8
	replyBuf := make([]byte, replySize)

	timeoutMs := uint32(timeout.Milliseconds())
	if timeoutMs == 0 {
		timeoutMs = 1000
	}

	ret, _, _ := procIcmpSend.Call(
		handle,
		uintptr(destAddr),
		uintptr(unsafe.Pointer(&sendData[0])),
		uintptr(len(sendData)),
		0,
		uintptr(unsafe.Pointer(&replyBuf[0])),
		replySize,
		uintptr(timeoutMs),
	)

	if ret == 0 {
		res.Status = "timeout"
		res.ErrorMsg = "превышено время ожидания ответа (таймаут)"
		return res
	}

	reply := (*icmpEchoReply)(unsafe.Pointer(&replyBuf[0]))
	if reply.Status != 0 {
		// 11003: IP_REQ_TIMED_OUT, 11010: IP_REQ_TIMED_OUT, 11002: IP_DEST_NET_UNREACHABLE
		if reply.Status == 11003 || reply.Status == 11010 || reply.Status == 11002 {
			res.Status = "timeout"
			res.ErrorMsg = fmt.Sprintf("таймаут ответа (код ICMP: %d)", reply.Status)
		} else {
			res.Status = "error"
			res.ErrorMsg = fmt.Sprintf("ошибка доставки ICMP (код: %d)", reply.Status)
		}
		return res
	}

	res.Status = "success"
	rtt := time.Duration(reply.RoundTripTime) * time.Millisecond
	if rtt == 0 {
		// Если время менее 1 мс, Win32 API возвращает 0. Установим 500 мкс для корректности расчетов
		rtt = 500 * time.Microsecond
	}
	res.RTT = rtt
	return res
}
