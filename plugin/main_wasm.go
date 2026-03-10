// zyrthi-flash-plugins-esp32 - ESP32 烧录插件
// 标准 Go WASI 实现（Go 1.24+）

//go:build wasip1

package main

import (
	"unsafe"
)

// 注：核心协议逻辑在 protocol.go 中，已通过测试验证
// main.go 仅包含 WASM 导出函数和 Host API 声明

// ============================================
// Host API - 宿主提供的函数（导入）
// ============================================

//go:wasmimport env serial_write
func hostSerialWrite(bufPtr uint32, bufLen uint32) uint32

//go:wasmimport env serial_read
func hostSerialRead(bufPtr uint32, bufLen uint32) uint32

//go:wasmimport env serial_set_dtr
func hostSerialSetDTR(level uint32)

//go:wasmimport env serial_set_rts
func hostSerialSetRTS(level uint32)

//go:wasmimport env delay
func hostDelay(ms uint32)

//go:wasmimport env log_info
func hostLogInfo(msgPtr uint32, msgLen uint32)

//go:wasmimport env progress
func hostProgress(current uint32, total uint32)

// ============================================
// Plugin API - 插件导出函数（宿主调用）
// ============================================

//go:wasmexport init
func pluginInit() int32 {
	return 0
}

//go:wasmexport detect
func pluginDetect() int32 {
	enterDownloadMode()
	if doSync() {
		return 0
	}
	return -1
}

//go:wasmexport flash
func pluginFlash(firmwarePtr uint32, firmwareLen uint32, offset uint32) int32 {
	firmware := unsafeSlice(firmwarePtr, firmwareLen)

	if !spiAttach() {
		return -1
	}
	if !setFlashParams() {
		return -1
	}
	if !flashWrite(firmware, offset) {
		return -1
	}
	return 0
}

//go:wasmexport erase
func pluginErase(offset uint32, size uint32) int32 {
	if !spiAttach() || !setFlashParams() {
		return -1
	}
	numBlocks := CalcNumBlocks(size)
	if !flashBegin(size, numBlocks, offset) {
		return -1
	}
	return 0
}

//go:wasmexport reset
func pluginReset() int32 {
	seq := GetResetSequence()
	for _, step := range seq {
		hostSerialSetDTR(step.DTR)
		hostSerialSetRTS(step.RTS)
		hostDelay(100)
	}
	return 0
}

func main() {}

// ============================================
// 缓冲区
// ============================================

var txBuf [8192]byte
var rxBuf [8192]byte
var frameBuf [2048]byte

// ============================================
// 工具函数
// ============================================

func unsafeSlice(ptr uint32, len uint32) []byte {
	return (*[1 << 28]byte)(unsafe.Pointer(uintptr(ptr)))[:len:len]
}

func logMsg(msg string) {
	ptr := unsafe.Pointer(unsafe.StringData(msg))
	hostLogInfo(uint32(uintptr(ptr)), uint32(len(msg)))
}

// ============================================
// ROM Bootloader 协议实现
// ============================================

func enterDownloadMode() {
	seq := GetDownloadModeSequence()
	for _, step := range seq {
		hostSerialSetDTR(step.DTR)
		hostSerialSetRTS(step.RTS)
		hostDelay(100)
	}
	hostDelay(400)
	drainBuffer()
}

func drainBuffer() {
	for {
		n := hostSerialRead(uint32(uintptr(unsafe.Pointer(&rxBuf[0]))), 4096)
		if n == 0 {
			break
		}
	}
}

func doSync() bool {
	syncData := GetSyncFrameData()

	for i := 0; i < 10; i++ {
		sendCmd(cmdSync, syncData[:])
		hostDelay(100)
		if checkResponse() {
			return true
		}
	}
	return false
}

func sendCmd(cmd byte, data []byte) {
	n := EncodeFrame(cmd, data, frameBuf[:])
	encoded := SlipEncode(frameBuf[:n], txBuf[:])
	hostSerialWrite(uint32(uintptr(unsafe.Pointer(&txBuf[0]))), uint32(encoded))
}

func checkResponse() bool {
	n := hostSerialRead(uint32(uintptr(unsafe.Pointer(&rxBuf[0]))), 4096)
	return CheckResponse(rxBuf[:], int(n))
}

func spiAttach() bool {
	sendCmd(cmdSpiAttach, make([]byte, 8))
	return checkResponse()
}

func setFlashParams() bool {
	params := FlashParams{
		ID:         0,
		TotalSize:  0x00400000, // 4MB
		BlockSize:  BlockSize,
		SectorSize: SectorSize,
		PageSize:   PageSize,
		StatusMask: 0xFFFFFFFF,
	}
	encoded := EncodeFlashParams(params)
	sendCmd(cmdSpiSetParams, encoded[:])
	return checkResponse()
}

func flashBegin(totalSize, numBlocks, offset uint32) bool {
	params := FlashBeginParams{
		TotalSize: totalSize,
		NumBlocks: numBlocks,
		BlockSize: BlockSize,
		Offset:    offset,
	}
	data := EncodeFlashBeginParams(params)
	sendCmd(cmdFlashBegin, data[:])
	return checkResponse()
}

func flashWrite(firmware []byte, offset uint32) bool {
	total := uint32(len(firmware))
	numBlocks := CalcNumBlocks(total)

	if !flashBegin(total, numBlocks, offset) {
		return false
	}

	seq := uint32(0)
	for i := uint32(0); i < total; i += BlockSize {
		end := i + BlockSize
		if end > total {
			end = total
		}
		block := firmware[i:end]

		// 4字节对齐填充
		paddedLen := Pad4Byte(len(block))
		data := make([]byte, 12+paddedLen)
		data[0] = byte(paddedLen)
		data[1] = byte(paddedLen >> 8)
		data[2] = byte(paddedLen >> 16)
		data[3] = byte(paddedLen >> 24)
		data[4] = byte(seq)
		data[5] = byte(seq >> 8)
		data[6] = byte(seq >> 16)
		data[7] = byte(seq >> 24)
		// data[8:12] = 0 (checksum placeholder)
		copy(data[12:], block)

		sendCmd(cmdFlashData, data)
		if !checkResponse() {
			return false
		}

		seq++
		hostProgress(i+uint32(len(block)), total)
	}

	return flashEnd()
}

func flashEnd() bool {
	data := [4]byte{0, 0, 0, 0}
	sendCmd(cmdFlashEnd, data[:])
	return checkResponse()
}
