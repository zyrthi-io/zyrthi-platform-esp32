// ESP32 Flash Plugin - 协议层测试
package main

import (
	"testing"
)

// ============================================
// 测试 SLIP 编码
// ============================================

func TestSlipEncode(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "普通数据",
			input:    []byte{0x01, 0x02, 0x03},
			expected: []byte{0xC0, 0x01, 0x02, 0x03, 0xC0},
		},
		{
			name:     "包含 END 字符",
			input:    []byte{0x01, 0xC0, 0x03},
			expected: []byte{0xC0, 0x01, 0xDB, 0xDC, 0x03, 0xC0},
		},
		{
			name:     "包含 ESC 字符",
			input:    []byte{0x01, 0xDB, 0x03},
			expected: []byte{0xC0, 0x01, 0xDB, 0xDD, 0x03, 0xC0},
		},
		{
			name:     "空数据",
			input:    []byte{},
			expected: []byte{0xC0, 0xC0},
		},
		{
			name:     "连续特殊字符",
			input:    []byte{0xC0, 0xDB, 0xC0},
			expected: []byte{0xC0, 0xDB, 0xDC, 0xDB, 0xDD, 0xDB, 0xDC, 0xC0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := make([]byte, 1024)
			n := SlipEncode(tt.input, out)
			result := out[:n]

			if string(result) != string(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// ============================================
// 测试 CRC32 校验和
// ============================================

func TestCalcChecksum(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected uint32
	}{
		{
			name:     "空数据",
			input:    []byte{},
			expected: 0xEF, // ESP32 ROM bootloader 的默认值
		},
		{
			name:     "单个字节",
			input:    []byte{0x00},
			expected: 0xD202EF8D, // CRC32 of 0x00
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalcChecksum(tt.input)
			if tt.expected != 0 && result != tt.expected {
				t.Errorf("expected 0x%08X, got 0x%08X", tt.expected, result)
			}
		})
	}
}

// ============================================
// 测试帧编码
// ============================================

func TestEncodeFrame(t *testing.T) {
	tests := []struct {
		name   string
		cmd    byte
		data   []byte
		minLen int
	}{
		{
			name:   "同步命令",
			cmd:    cmdSync,
			data:   make([]byte, 36),
			minLen: 44, // 8 header + 36 data
		},
		{
			name:   "空数据命令",
			cmd:    cmdSpiAttach,
			data:   []byte{},
			minLen: 8, // 只有 header
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := make([]byte, 1024)
			n := EncodeFrame(tt.cmd, tt.data, out)

			if n < tt.minLen {
				t.Errorf("expected at least %d bytes, got %d", tt.minLen, n)
			}

			// 检查帧头
			if out[0] != 0x00 {
				t.Error("first byte should be 0x00 (direction)")
			}
			if out[1] != tt.cmd {
				t.Errorf("expected cmd 0x%02X, got 0x%02X", tt.cmd, out[1])
			}
		})
	}
}

// ============================================
// 测试同步帧
// ============================================

func TestGetSyncFrameData(t *testing.T) {
	syncData := GetSyncFrameData()

	// 验证同步帧数据格式正确
	if syncData[0] != 0x07 || syncData[1] != 0x07 {
		t.Error("sync frame should start with 0x07 0x07")
	}
	if syncData[2] != 0x12 || syncData[3] != 0x20 {
		t.Error("sync frame bytes 2-3 should be 0x12 0x20")
	}

	// 验证后面的 32 个 0x55
	for i := 4; i < 36; i++ {
		if syncData[i] != 0x55 {
			t.Errorf("sync frame byte %d should be 0x55", i)
		}
	}
}

// ============================================
// 测试 Flash 参数
// ============================================

func TestEncodeFlashParams(t *testing.T) {
	params := FlashParams{
		ID:         0,
		TotalSize:  0x00400000, // 4MB
		BlockSize:  BlockSize,
		SectorSize: SectorSize,
		PageSize:   PageSize,
		StatusMask: 0xFFFFFFFF,
	}

	encoded := EncodeFlashParams(params)

	// 验证 TotalSize
	totalSize := uint32(encoded[4]) | uint32(encoded[5])<<8 | uint32(encoded[6])<<16 | uint32(encoded[7])<<24
	if totalSize != params.TotalSize {
		t.Errorf("expected TotalSize 0x%08X, got 0x%08X", params.TotalSize, totalSize)
	}

	// 验证 BlockSize
	blockSize := uint32(encoded[8]) | uint32(encoded[9])<<8 | uint32(encoded[10])<<16 | uint32(encoded[11])<<24
	if blockSize != params.BlockSize {
		t.Errorf("expected BlockSize 0x%08X, got 0x%08X", params.BlockSize, blockSize)
	}
}

// ============================================
// 测试 Flash Begin 参数编码
// ============================================

func TestEncodeFlashBeginParams(t *testing.T) {
	params := FlashBeginParams{
		TotalSize: 0x10000, // 64KB
		NumBlocks: 16,
		BlockSize: BlockSize,
		Offset:    0x1000, // 4KB offset
	}

	encoded := EncodeFlashBeginParams(params)

	// 验证 TotalSize
	totalSize := uint32(encoded[0]) | uint32(encoded[1])<<8 | uint32(encoded[2])<<16 | uint32(encoded[3])<<24
	if totalSize != params.TotalSize {
		t.Errorf("expected TotalSize %d, got %d", params.TotalSize, totalSize)
	}

	// 验证 Offset
	offset := uint32(encoded[12]) | uint32(encoded[13])<<8 | uint32(encoded[14])<<16 | uint32(encoded[15])<<24
	if offset != params.Offset {
		t.Errorf("expected Offset %d, got %d", params.Offset, offset)
	}
}

// ============================================
// 测试响应检查
// ============================================

func TestCheckResponse(t *testing.T) {
	tests := []struct {
		name     string
		response []byte
		expected bool
	}{
		{
			name:     "成功响应",
			response: []byte{0xC0, 0x01, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0},
			expected: true, // 最后第二字节是 0x00
		},
		{
			name:     "失败响应",
			response: []byte{0xC0, 0x01, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05, 0xC0},
			expected: false, // 最后第二字节非 0x00
		},
		{
			name:     "响应太短",
			response: []byte{0xC0, 0xC0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckResponse(tt.response, len(tt.response))
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// ============================================
// 测试下载模式序列
// ============================================

func TestGetDownloadModeSequence(t *testing.T) {
	seq := GetDownloadModeSequence()

	if len(seq) != 3 {
		t.Errorf("expected 3 steps, got %d", len(seq))
	}

	// 验证最终状态是释放
	if seq[2].DTR != 0 || seq[2].RTS != 0 {
		t.Error("final state should be DTR=0, RTS=0")
	}
}

// ============================================
// 测试复位序列
// ============================================

func TestGetResetSequence(t *testing.T) {
	seq := GetResetSequence()

	if len(seq) != 3 {
		t.Errorf("expected 3 steps, got %d", len(seq))
	}
}

// ============================================
// 测试分块计算
// ============================================

func TestCalcNumBlocks(t *testing.T) {
	tests := []struct {
		name      string
		totalSize uint32
		expected  uint32
	}{
		{"小于一块", 0x1000, 1},   // 4KB -> 1 block
		{"正好一块", 0x4000, 1},   // 16KB -> 1 block
		{"一块多", 0x5000, 2},     // 20KB -> 2 blocks
		{"正好两块", 0x8000, 2},   // 32KB -> 2 blocks
		{"大文件", 0x40000, 16},   // 256KB -> 16 blocks
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalcNumBlocks(tt.totalSize)
			if result != tt.expected {
				t.Errorf("expected %d blocks, got %d", tt.expected, result)
			}
		})
	}
}

// ============================================
// 测试 4 字节对齐填充
// ============================================

func TestPad4Byte(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"不需要填充", 16, 16},
		{"需要填充1字节", 17, 20},
		{"需要填充2字节", 18, 20},
		{"需要填充3字节", 19, 20},
		{"正好对齐", 20, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Pad4Byte(tt.input)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// ============================================
// 测试常量
// ============================================

func TestConstants(t *testing.T) {
	// 验证常量定义正确
	if BlockSize != 0x4000 {
		t.Errorf("expected BlockSize 0x4000, got 0x%04X", BlockSize)
	}
	if SectorSize != 0x1000 {
		t.Errorf("expected SectorSize 0x1000, got 0x%04X", SectorSize)
	}
	if PageSize != 0x100 {
		t.Errorf("expected PageSize 0x100, got 0x%04X", PageSize)
	}

	// 验证命令常量（参考 ESP32 ROM bootloader 协议规范）
	// https://docs.espressif.com/projects/esptool/en/latest/esp32/advanced-topics/serial-protocol.html
	if cmdSync != 0x08 {
		t.Errorf("expected cmdSync 0x08, got 0x%02X", cmdSync)
	}
	if cmdFlashBegin != 0x02 {
		t.Errorf("expected cmdFlashBegin 0x02, got 0x%02X", cmdFlashBegin)
	}
	if cmdFlashData != 0x03 {
		t.Errorf("expected cmdFlashData 0x03, got 0x%02X", cmdFlashData)
	}
	if cmdFlashEnd != 0x04 {
		t.Errorf("expected cmdFlashEnd 0x04, got 0x%02X", cmdFlashEnd)
	}
	if cmdSpiSetParams != 0x0B {
		t.Errorf("expected cmdSpiSetParams 0x0B, got 0x%02X", cmdSpiSetParams)
	}
	if cmdChangeBaud != 0x0F {
		t.Errorf("expected cmdChangeBaud 0x0F, got 0x%02X", cmdChangeBaud)
	}
	if cmdFlashMd5 != 0x13 {
		t.Errorf("expected cmdFlashMd5 0x13, got 0x%02X", cmdFlashMd5)
	}
	if cmdEraseFlash != 0xD0 {
		t.Errorf("expected cmdEraseFlash 0xD0, got 0x%02X", cmdEraseFlash)
	}
	if cmdReadFlash != 0xD2 {
		t.Errorf("expected cmdReadFlash 0xD2, got 0x%02X", cmdReadFlash)
	}
}
