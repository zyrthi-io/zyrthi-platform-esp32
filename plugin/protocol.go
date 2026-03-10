// ESP32 Flash Plugin - 协议层（可测试）
// 此文件不依赖 WASM，可被普通 Go 测试

package main

// ============================================
// 常量
// ============================================

const (
	cmdSync         = 0x08
	cmdReadReg      = 0x0A
	cmdWriteReg     = 0x09
	cmdFlashBegin   = 0x0B
	cmdFlashData    = 0x07
	cmdFlashEnd     = 0x06
	cmdSpiAttach    = 0x0D
	cmdSpiSetParams = 0x0F
	cmdChangeBaud   = 0x0E

	slipEnd    = 0xC0
	slipEsc    = 0xDB
	slipEscEnd = 0xDC
	slipEscEsc = 0xDD

	BlockSize  = 0x4000 // 16KB
	SectorSize = 0x1000 // 4KB
	PageSize   = 0x100  // 256B
)

// ============================================
// SLIP 编码
// ============================================

// SlipEncode 将数据进行 SLIP 编码
func SlipEncode(in []byte, out []byte) int {
	out[0] = slipEnd
	j := 1
	for _, b := range in {
		switch b {
		case slipEnd:
			out[j] = slipEsc
			out[j+1] = slipEscEnd
			j += 2
		case slipEsc:
			out[j] = slipEsc
			out[j+1] = slipEscEsc
			j += 2
		default:
			out[j] = b
			j++
		}
	}
	out[j] = slipEnd
	return j + 1
}

// ============================================
// CRC32 校验和
// ============================================

// CalcChecksum 计算 ESP32 ROM bootloader 使用的 CRC32
func CalcChecksum(data []byte) uint32 {
	if len(data) == 0 {
		return 0xEF
	}
	crc := uint32(0xFFFFFFFF)
	for _, b := range data {
		crc ^= uint32(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xEDB88320
			} else {
				crc >>= 1
			}
		}
	}
	return crc ^ 0xFFFFFFFF
}

// ============================================
// 帧编码
// ============================================

// EncodeFrame 编码命令帧
func EncodeFrame(cmd byte, data []byte, out []byte) int {
	out[0] = 0x00 // direction
	out[1] = cmd
	out[2] = byte(len(data))
	out[3] = byte(len(data) >> 8)
	cs := CalcChecksum(data)
	out[4] = byte(cs)
	out[5] = byte(cs >> 8)
	out[6] = byte(cs >> 16)
	out[7] = byte(cs >> 24)
	copy(out[8:], data)
	return 8 + len(data)
}

// ============================================
// 同步帧数据
// ============================================

// GetSyncFrameData 返回 ESP32 同步帧数据
func GetSyncFrameData() [36]byte {
	return [36]byte{
		0x07, 0x07, 0x12, 0x20,
		0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
		0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
		0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
		0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
	}
}

// ============================================
// Flash 参数
// ============================================

// FlashParams Flash 参数结构
type FlashParams struct {
	ID         uint32
	TotalSize  uint32
	BlockSize  uint32
	SectorSize uint32
	PageSize   uint32
	StatusMask uint32
}

// EncodeFlashParams 编码 Flash 参数为 24 字节
func EncodeFlashParams(p FlashParams) [24]byte {
	return [24]byte{
		0: byte(p.ID), 1: byte(p.ID >> 8), 2: byte(p.ID >> 16), 3: byte(p.ID >> 24),
		4: byte(p.TotalSize), 5: byte(p.TotalSize >> 8), 6: byte(p.TotalSize >> 16), 7: byte(p.TotalSize >> 24),
		8: byte(p.BlockSize), 9: byte(p.BlockSize >> 8), 10: byte(p.BlockSize >> 16), 11: byte(p.BlockSize >> 24),
		12: byte(p.SectorSize), 13: byte(p.SectorSize >> 8), 14: byte(p.SectorSize >> 16), 15: byte(p.SectorSize >> 24),
		16: byte(p.PageSize), 17: byte(p.PageSize >> 8), 18: byte(p.PageSize >> 16), 19: byte(p.PageSize >> 24),
		20: byte(p.StatusMask), 21: byte(p.StatusMask >> 8), 22: byte(p.StatusMask >> 16), 23: byte(p.StatusMask >> 24),
	}
}

// ============================================
// Flash Begin 参数
// ============================================

// FlashBeginParams Flash Begin 参数
type FlashBeginParams struct {
	TotalSize  uint32
	NumBlocks  uint32
	BlockSize  uint32
	Offset     uint32
}

// EncodeFlashBeginParams 编码 Flash Begin 参数
func EncodeFlashBeginParams(p FlashBeginParams) [16]byte {
	return [16]byte{
		0: byte(p.TotalSize), 1: byte(p.TotalSize >> 8), 2: byte(p.TotalSize >> 16), 3: byte(p.TotalSize >> 24),
		4: byte(p.NumBlocks), 5: byte(p.NumBlocks >> 8), 6: byte(p.NumBlocks >> 16), 7: byte(p.NumBlocks >> 24),
		8: byte(p.BlockSize), 9: byte(p.BlockSize >> 8), 10: byte(p.BlockSize >> 16), 11: byte(p.BlockSize >> 24),
		12: byte(p.Offset), 13: byte(p.Offset >> 8), 14: byte(p.Offset >> 16), 15: byte(p.Offset >> 24),
	}
}

// ============================================
// 响应检查
// ============================================

// CheckResponse 检查响应是否成功
func CheckResponse(rxBuf []byte, n int) bool {
	if n < 10 {
		return false
	}
	return rxBuf[n-2] == 0x00
}

// ============================================
// 块计算
// ============================================

// CalcNumBlocks 计算需要的块数
func CalcNumBlocks(totalSize uint32) uint32 {
	return (totalSize + BlockSize - 1) / BlockSize
}

// ============================================
// 4 字节对齐填充
// ============================================

// Pad4Byte 计算对齐后的长度
func Pad4Byte(len int) int {
	return (len + 3) &^ 3
}

// ============================================
// 下载模式序列
// ============================================

// DownloadModeStep 下载模式的一个步骤
type DownloadModeStep struct {
	DTR uint32
	RTS uint32
}

// GetDownloadModeSequence 返回进入下载模式的 DTR/RTS 序列
func GetDownloadModeSequence() []DownloadModeStep {
	return []DownloadModeStep{
		{0, 1}, // DTR=0, RTS=1 -> EN=0, IO0=0
		{1, 0}, // DTR=1, RTS=0 -> EN=1, IO0=0
		{0, 0}, // 释放
	}
}

// ============================================
// 复位序列
// ============================================

// GetResetSequence 返回复位的 DTR/RTS 序列
func GetResetSequence() []DownloadModeStep {
	return []DownloadModeStep{
		{0, 0}, // DTR=0
		{0, 1}, // RTS=1
		{0, 0}, // RTS=0
	}
}
