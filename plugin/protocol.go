// ESP32 Flash Plugin - 协议层（可测试）
// 此文件不依赖 WASM，可被普通 Go 测试
//
// 命令码参考：https://docs.espressif.com/projects/esptool/en/latest/esp32/advanced-topics/serial-protocol.html

package main

// ============================================
// 命令码常量（ROM + Stub）
// ============================================

const (
	// ROM 命令
	cmdFlashBegin   = 0x02 // 开始 Flash 下载
	cmdFlashData    = 0x03 // Flash 数据传输
	cmdFlashEnd     = 0x04 // 结束 Flash 下载
	cmdMemBegin     = 0x05 // 开始 RAM 下载
	cmdMemEnd       = 0x06 // 结束 RAM 下载
	cmdMemData      = 0x07 // RAM 数据传输
	cmdSync         = 0x08 // 同步帧
	cmdWriteReg     = 0x09 // 写寄存器
	cmdReadReg      = 0x0A // 读寄存器
	cmdSpiSetParams = 0x0B // 设置 SPI Flash 参数
	cmdSpiAttach    = 0x0D // 连接 SPI Flash
	cmdReadFlashSlow = 0x0E // 读取 Flash（ROM，慢）
	cmdChangeBaud   = 0x0F // 改变波特率
	cmdFlashDeflBegin = 0x10 // 开始压缩 Flash 下载
	cmdFlashDeflData  = 0x11 // 压缩 Flash 数据
	cmdFlashDeflEnd   = 0x12 // 结束压缩 Flash 下载
	cmdFlashMd5     = 0x13 // 计算 Flash 区域 MD5
	cmdGetSecurityInfo = 0x14 // 获取安全信息

	// Stub 命令（仅 stub 加载后可用）
	cmdEraseFlash   = 0xD0 // 全片擦除
	cmdEraseRegion  = 0xD1 // 区域擦除
	cmdReadFlash    = 0xD2 // 读取 Flash（stub）
	cmdRunUserCode  = 0xD3 // 运行用户代码

	// 其他
	cmdFlashDetect  = 0x9F // 检测 Flash ID
)

// ============================================
// SLIP 常量
// ============================================

const (
	slipEnd    = 0xC0
	slipEsc    = 0xDB
	slipEscEnd = 0xDC
	slipEscEsc = 0xDD
)

// ============================================
// Flash 尺寸常量
// ============================================

const (
	BlockSize  = 0x4000 // 16KB - 数据块大小
	SectorSize = 0x1000 // 4KB - 扇区大小
	PageSize   = 0x100  // 256B - 页大小
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

// ============================================
// MD5 命令参数
// ============================================

// FlashMd5Params MD5 命令参数
type FlashMd5Params struct {
	Offset uint32
	Size   uint32
}

// EncodeFlashMd5Params 编码 MD5 参数为 16 字节
func EncodeFlashMd5Params(p FlashMd5Params) [16]byte {
	return [16]byte{
		0: byte(p.Offset), 1: byte(p.Offset >> 8), 2: byte(p.Offset >> 16), 3: byte(p.Offset >> 24),
		4: byte(p.Size), 5: byte(p.Size >> 8), 6: byte(p.Size >> 16), 7: byte(p.Size >> 24),
		8: 0, 9: 0, 10: 0, 11: 0, // reserved
		12: 0, 13: 0, 14: 0, 15: 0, // reserved
	}
}

// ParseMd5Response 解析 MD5 响应（返回 16 字节 BE 的 MD5）
func ParseMd5Response(rxBuf []byte, n int) [16]byte {
	var md5 [16]byte
	// 响应格式：方向(1) + 命令(1) + 长度(2) + 值(4) + MD5(16) + error(1) + status(1)
	// ROM loader: 44 字节响应，MD5 在 offset 8
	// Stub: 26 字节响应，MD5 在 offset 8
	if n >= 24 {
		copy(md5[:], rxBuf[8:24])
	}
	return md5
}

// ============================================
// 读取 Flash 命令参数
// ============================================

// ReadFlashParams 读取 Flash 参数
type ReadFlashParams struct {
	Offset      uint32
	Size        uint32
	BlockSize   uint32
	MaxInFlight uint32
}

// EncodeReadFlashParams 编码读取参数为 16 字节
func EncodeReadFlashParams(p ReadFlashParams) [16]byte {
	return [16]byte{
		0: byte(p.Offset), 1: byte(p.Offset >> 8), 2: byte(p.Offset >> 16), 3: byte(p.Offset >> 24),
		4: byte(p.Size), 5: byte(p.Size >> 8), 6: byte(p.Size >> 16), 7: byte(p.Size >> 24),
		8: byte(p.BlockSize), 9: byte(p.BlockSize >> 8), 10: byte(p.BlockSize >> 16), 11: byte(p.BlockSize >> 24),
		12: byte(p.MaxInFlight), 13: byte(p.MaxInFlight >> 8), 14: byte(p.MaxInFlight >> 16), 15: byte(p.MaxInFlight >> 24),
	}
}

// ============================================
// 擦除命令参数
// ============================================

// EraseRegionParams 区域擦除参数
type EraseRegionParams struct {
	Offset uint32
	Size   uint32
}

// EncodeEraseRegionParams 编码擦除参数为 8 字节
func EncodeEraseRegionParams(p EraseRegionParams) [8]byte {
	return [8]byte{
		0: byte(p.Offset), 1: byte(p.Offset >> 8), 2: byte(p.Offset >> 16), 3: byte(p.Offset >> 24),
		4: byte(p.Size), 5: byte(p.Size >> 8), 6: byte(p.Size >> 16), 7: byte(p.Size >> 24),
	}
}

// ============================================
// 压缩烧录参数
// ============================================

// FlashDeflBeginParams 压缩烧录开始参数
type FlashDeflBeginParams struct {
	UncompressedSize uint32
	NumBlocks        uint32
	BlockSize        uint32
	Offset           uint32
}

// EncodeFlashDeflBeginParams 编码压缩烧录参数
func EncodeFlashDeflBeginParams(p FlashDeflBeginParams) [16]byte {
	return [16]byte{
		0: byte(p.UncompressedSize), 1: byte(p.UncompressedSize >> 8), 2: byte(p.UncompressedSize >> 16), 3: byte(p.UncompressedSize >> 24),
		4: byte(p.NumBlocks), 5: byte(p.NumBlocks >> 8), 6: byte(p.NumBlocks >> 16), 7: byte(p.NumBlocks >> 24),
		8: byte(p.BlockSize), 9: byte(p.BlockSize >> 8), 10: byte(p.BlockSize >> 16), 11: byte(p.BlockSize >> 24),
		12: byte(p.Offset), 13: byte(p.Offset >> 8), 14: byte(p.Offset >> 16), 15: byte(p.Offset >> 24),
	}
}

// ============================================
// Flash Data 参数
// ============================================

// FlashDataParams Flash 数据传输参数
type FlashDataParams struct {
	DataSize uint32
	Sequence uint32
}

// EncodeFlashDataParams 编码 Flash Data 参数头（8 字节）
func EncodeFlashDataParams(p FlashDataParams) [8]byte {
	return [8]byte{
		0: byte(p.DataSize), 1: byte(p.DataSize >> 8), 2: byte(p.DataSize >> 16), 3: byte(p.DataSize >> 24),
		4: byte(p.Sequence), 5: byte(p.Sequence >> 8), 6: byte(p.Sequence >> 16), 7: byte(p.Sequence >> 24),
	}
}

// ============================================
// 波特率参数
// ============================================

// ChangeBaudParams 波特率切换参数
type ChangeBaudParams struct {
	NewBaud   uint32
	PriorBaud uint32
}

// EncodeChangeBaudParams 编码波特率参数为 8 字节
func EncodeChangeBaudParams(p ChangeBaudParams) [8]byte {
	return [8]byte{
		0: byte(p.NewBaud), 1: byte(p.NewBaud >> 8), 2: byte(p.NewBaud >> 16), 3: byte(p.NewBaud >> 24),
		4: byte(p.PriorBaud), 5: byte(p.PriorBaud >> 8), 6: byte(p.PriorBaud >> 16), 7: byte(p.PriorBaud >> 24),
	}
}

// ============================================
// 写寄存器参数
// ============================================

// WriteRegParams 写寄存器参数
type WriteRegParams struct {
	Address  uint32
	Value    uint32
	Mask     uint32
	DelayUs  uint32
}

// EncodeWriteRegParams 编码写寄存器参数为 16 字节
func EncodeWriteRegParams(p WriteRegParams) [16]byte {
	return [16]byte{
		0: byte(p.Address), 1: byte(p.Address >> 8), 2: byte(p.Address >> 16), 3: byte(p.Address >> 24),
		4: byte(p.Value), 5: byte(p.Value >> 8), 6: byte(p.Value >> 16), 7: byte(p.Value >> 24),
		8: byte(p.Mask), 9: byte(p.Mask >> 8), 10: byte(p.Mask >> 16), 11: byte(p.Mask >> 24),
		12: byte(p.DelayUs), 13: byte(p.DelayUs >> 8), 14: byte(p.DelayUs >> 16), 15: byte(p.DelayUs >> 24),
	}
}

// ============================================
// RAM 下载参数
// ============================================

// MemBeginParams RAM 下载开始参数
type MemBeginParams struct {
	TotalSize  uint32
	NumBlocks  uint32
	BlockSize  uint32
	Offset     uint32
}

// EncodeMemBeginParams 编码 RAM 下载参数
func EncodeMemBeginParams(p MemBeginParams) [16]byte {
	return [16]byte{
		0: byte(p.TotalSize), 1: byte(p.TotalSize >> 8), 2: byte(p.TotalSize >> 16), 3: byte(p.TotalSize >> 24),
		4: byte(p.NumBlocks), 5: byte(p.NumBlocks >> 8), 6: byte(p.NumBlocks >> 16), 7: byte(p.NumBlocks >> 24),
		8: byte(p.BlockSize), 9: byte(p.BlockSize >> 8), 10: byte(p.BlockSize >> 16), 11: byte(p.BlockSize >> 24),
		12: byte(p.Offset), 13: byte(p.Offset >> 8), 14: byte(p.Offset >> 16), 15: byte(p.Offset >> 24),
	}
}

// MemEndParams RAM 下载结束参数
type MemEndParams struct {
	NoEntry uint32 // 0 = 执行入口, 1 = 不执行
	Entry   uint32 // 入口地址
}

// EncodeMemEndParams 编码 RAM 结束参数
func EncodeMemEndParams(p MemEndParams) [8]byte {
	return [8]byte{
		0: byte(p.NoEntry), 1: byte(p.NoEntry >> 8), 2: byte(p.NoEntry >> 16), 3: byte(p.NoEntry >> 24),
		4: byte(p.Entry), 5: byte(p.Entry >> 8), 6: byte(p.Entry >> 16), 7: byte(p.Entry >> 24),
	}
}

// ============================================
// SLIP 解码
// ============================================

// SlipDecodeState SLIP 解码状态
type SlipDecodeState struct {
	escaping bool
}

// SlipDecode 解码 SLIP 数据，返回解码后的字节数
func SlipDecode(in []byte, inLen int, out []byte, state *SlipDecodeState) int {
	j := 0
	for i := 0; i < inLen; i++ {
		b := in[i]
		switch {
		case b == slipEnd:
			if j > 0 {
				return j // 包结束
			}
		case b == slipEsc:
			state.escaping = true
		case state.escaping:
			switch b {
			case slipEscEnd:
				out[j] = slipEnd
			case slipEscEsc:
				out[j] = slipEsc
			default:
				out[j] = b
			}
			j++
			state.escaping = false
		default:
			out[j] = b
			j++
		}
	}
	return j
}

// ============================================
// 响应解析
// ============================================

// Response 命令响应
type Response struct {
	Direction    byte
	Command      byte
	Length       uint16
	Value        uint32
	Error        byte
	Status       byte
	Data         []byte
}

// ParseResponse 解析响应
func ParseResponse(rxBuf []byte, n int) (Response, bool) {
	if n < 8 {
		return Response{}, false
	}
	resp := Response{
		Direction: rxBuf[0],
		Command:   rxBuf[1],
		Length:    uint16(rxBuf[2]) | uint16(rxBuf[3])<<8,
	}
	// 状态字节位置取决于响应长度
	statusLen := 2
	if n == 10 || n == 26 {
		statusLen = 2 // stub 响应
	} else if n >= 12 {
		statusLen = 4 // ROM 响应
	}
	if n >= 8 {
		resp.Value = uint32(rxBuf[4]) | uint32(rxBuf[5])<<8 | uint32(rxBuf[6])<<16 | uint32(rxBuf[7])<<24
	}
	if n >= statusLen+2 {
		resp.Error = rxBuf[n-statusLen+1]
		resp.Status = rxBuf[n-statusLen]
	}
	if resp.Length > 0 && n >= int(8+resp.Length) {
		resp.Data = rxBuf[8 : 8+resp.Length]
	}
	return resp, true
}

// IsSuccess 检查响应是否成功
func (r Response) IsSuccess() bool {
	return r.Status == 0
}

// ============================================
// Flash 大小编码
// ============================================

// FlashSize Flash 大小枚举
type FlashSize uint8

const (
	FlashSize1MB  FlashSize = 0
	FlashSize2MB  FlashSize = 1
	FlashSize4MB  FlashSize = 2
	FlashSize8MB  FlashSize = 3
	FlashSize16MB FlashSize = 4
	FlashSize32MB FlashSize = 5
	FlashSize64MB FlashSize = 6
	FlashSize128MB FlashSize = 7
	FlashSize256MB FlashSize = 8
)

// DetectFlashSize 从 Flash ID 检测大小
func DetectFlashSize(flashID uint32) FlashSize {
	sizeID := byte(flashID >> 16)
	switch sizeID {
	case 0x12, 0x32:
		return FlashSize1MB
	case 0x13, 0x33:
		return FlashSize1MB
	case 0x14, 0x34:
		return FlashSize1MB
	case 0x15, 0x35:
		return FlashSize2MB
	case 0x16, 0x36:
		return FlashSize4MB
	case 0x17, 0x37:
		return FlashSize8MB
	case 0x18, 0x38:
		return FlashSize16MB
	case 0x19, 0x39:
		return FlashSize32MB
	case 0x20, 0x1A, 0x3A:
		return FlashSize64MB
	case 0x21, 0x1B:
		return FlashSize128MB
	case 0x22, 0x1C:
		return FlashSize256MB
	default:
		return FlashSize4MB // 默认 4MB
	}
}

// ToBytes 转换为字节数
func (s FlashSize) ToBytes() uint32 {
	switch s {
	case FlashSize1MB:
		return 1 << 20
	case FlashSize2MB:
		return 2 << 20
	case FlashSize4MB:
		return 4 << 20
	case FlashSize8MB:
		return 8 << 20
	case FlashSize16MB:
		return 16 << 20
	case FlashSize32MB:
		return 32 << 20
	case FlashSize64MB:
		return 64 << 20
	case FlashSize128MB:
		return 128 << 20
	case FlashSize256MB:
		return 256 << 20
	default:
		return 4 << 20
	}
}

// ============================================
// 安全信息（Security Info）
// ============================================

// SecurityInfo 芯片安全信息
type SecurityInfo struct {
	Valid          bool     // 是否有效
	Flags          uint32   // 安全标志位
	FlashCryptCnt  uint8    // Flash 加密计数
	KeyPurposes    [7]byte  // 密钥用途
	ChipID         uint32   // 芯片 ID（可选）
	EcoVersion     uint32   // ECO 版本（可选）
}

// 安全标志位定义
const (
	SecFlagSecureBootEn           uint32 = 1 << 0  // 安全启动启用
	SecFlagSecureBootAggressiveRevoke uint32 = 1 << 1  // 激进密钥吊销
	SecFlagSecureDownloadEnable   uint32 = 1 << 2  // 安全下载模式启用
	SecFlagSecureBootKeyRevoke0   uint32 = 1 << 3  // 密钥0已吊销
	SecFlagSecureBootKeyRevoke1   uint32 = 1 << 4  // 密钥1已吊销
	SecFlagSecureBootKeyRevoke2   uint32 = 1 << 5  // 密钥2已吊销
	SecFlagSoftDisJTAG            uint32 = 1 << 6  // 软件禁用 JTAG
	SecFlagHardDisJTAG            uint32 = 1 << 7  // 硬件禁用 JTAG
	SecFlagDisUSB                 uint32 = 1 << 8  // 禁用 USB
	SecFlagDisDownloadDCache      uint32 = 1 << 9  // 禁用下载 DCache
	SecFlagDisDownloadICache      uint32 = 1 << 10 // 禁用下载 ICache
)

// ParseSecurityInfo 解析安全信息响应
func ParseSecurityInfo(data []byte) SecurityInfo {
	if len(data) < 12 {
		return SecurityInfo{Valid: false}
	}

	info := SecurityInfo{
		Valid:         true,
		Flags:         uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24,
		FlashCryptCnt: data[4],
	}
	copy(info.KeyPurposes[:], data[5:12])

	// ESP32-S2 只有 12 字节，其他芯片有 20 字节
	if len(data) >= 20 {
		info.ChipID = uint32(data[12]) | uint32(data[13])<<8 | uint32(data[14])<<16 | uint32(data[15])<<24
		info.EcoVersion = uint32(data[16]) | uint32(data[17])<<8 | uint32(data[18])<<16 | uint32(data[19])<<24
	}

	return info
}

// IsSecureBootEnabled 检查安全启动是否启用
func (s SecurityInfo) IsSecureBootEnabled() bool {
	return (s.Flags & SecFlagSecureBootEn) != 0
}

// IsSecureDownloadMode 检查是否处于安全下载模式
func (s SecurityInfo) IsSecureDownloadMode() bool {
	return (s.Flags & SecFlagSecureDownloadEnable) != 0
}

// IsFlashEncryptionEnabled 检查 Flash 加密是否启用
func (s SecurityInfo) IsFlashEncryptionEnabled() bool {
	// Flash 加密状态由 flash_crypt_cnt 的奇偶性决定
	return s.FlashCryptCnt%2 == 1
}

// IsJTAGDisabled 检查 JTAG 是否被禁用
func (s SecurityInfo) IsJTAGDisabled() bool {
	return (s.Flags & (SecFlagSoftDisJTAG | SecFlagHardDisJTAG)) != 0
}

// GetRevokedKeys 获取已吊销的密钥列表
func (s SecurityInfo) GetRevokedKeys() []int {
	var revoked []int
	if (s.Flags & SecFlagSecureBootKeyRevoke0) != 0 {
		revoked = append(revoked, 0)
	}
	if (s.Flags & SecFlagSecureBootKeyRevoke1) != 0 {
		revoked = append(revoked, 1)
	}
	if (s.Flags & SecFlagSecureBootKeyRevoke2) != 0 {
		revoked = append(revoked, 2)
	}
	return revoked
}

// ============================================
// 魔数检测（芯片识别）
// ============================================

// ChipMagic 芯片魔数定义
type ChipMagic uint32

const (
	MagicESP32    ChipMagic = 0x00f01d83
	MagicESP32S2  ChipMagic = 0x000007c6
	MagicESP32S3  ChipMagic = 0x00000009
	MagicESP32C2  ChipMagic = 0x6f51306f // ECO0
	MagicESP32C2v1 ChipMagic = 0x7c41a06f // ECO1
	MagicESP32C3  ChipMagic = 0x6921506f // ECO1+ECO2
	MagicESP32C3v3 ChipMagic = 0x1b31506f // ECO3
	MagicESP32C3v6 ChipMagic = 0x4881606F // ECO6
	MagicESP32C3v7 ChipMagic = 0x4361606f // ECO7
	MagicESP32C5  ChipMagic = 0x1101406f
	MagicESP32C6  ChipMagic = 0x2CE0806F
	MagicESP32H2  ChipMagic = 0xD7B73E80
	MagicESP32P4  ChipMagic = 0x0ADDBAD0
)

// ChipDetectMagicRegAddr 芯片检测魔数寄存器地址
const ChipDetectMagicRegAddr = 0x40001000

// DetectChipFromMagic 从魔数检测芯片类型
func DetectChipFromMagic(magic uint32) (string, bool) {
	switch ChipMagic(magic) {
	case MagicESP32:
		return "ESP32", true
	case MagicESP32S2:
		return "ESP32-S2", true
	case MagicESP32S3:
		return "ESP32-S3", true
	case MagicESP32C2, MagicESP32C2v1:
		return "ESP32-C2", true
	case MagicESP32C3, MagicESP32C3v3, MagicESP32C3v6, MagicESP32C3v7:
		return "ESP32-C3", true
	case MagicESP32C5:
		return "ESP32-C5", true
	case MagicESP32C6:
		return "ESP32-C6", true
	case MagicESP32H2:
		return "ESP32-H2", true
	case MagicESP32P4:
		return "ESP32-P4", true
	default:
		return "", false
	}
}

// ============================================
// 烧录安全检查
// ============================================

// FlashSafetyCheck 烧录安全检查结果
type FlashSafetyCheck struct {
	CanFlash           bool     // 是否可以烧录
	Warnings           []string // 警告信息
	Errors             []string // 错误信息（阻止烧录）
	IsSecureDownload   bool     // 是否安全下载模式
	IsSecureBoot       bool     // 是否安全启动
	IsFlashEncrypted   bool     // 是否 Flash 加密
}

// SafetyCheckLevel 安全检查级别
type SafetyCheckLevel int

const (
	SafetyCheckMinimal SafetyCheckLevel = iota // 最小检查
	SafetyCheckNormal                          // 正常检查
	SafetyCheckStrict                          // 严格检查
)

// PerformSafetyCheck 执行烧录前安全检查
// 检查内容包括：
// 1. Secure Download Mode 状态
// 2. Secure Boot 状态
// 3. Flash Encryption 状态
// 4. 密钥吊销状态
func PerformSafetyCheck(info SecurityInfo, level SafetyCheckLevel) FlashSafetyCheck {
	result := FlashSafetyCheck{
		CanFlash: true,
	}

	// 检查安全下载模式
	if info.IsSecureDownloadMode() {
		result.IsSecureDownload = true
		result.Warnings = append(result.Warnings,
			"Secure Download Mode enabled: flash read operations are restricted")
	}

	// 检查安全启动
	if info.IsSecureBootEnabled() {
		result.IsSecureBoot = true
		result.Warnings = append(result.Warnings,
			"Secure Boot enabled: only signed firmware can be flashed")
		
		// ��查激进吊销模式
		if (info.Flags & SecFlagSecureBootAggressiveRevoke) != 0 {
			result.Warnings = append(result.Warnings,
				"Aggressive key revocation enabled: flashing unsigned firmware may brick the device")
		}

		// 检查已吊销的密钥
		revoked := info.GetRevokedKeys()
		if len(revoked) > 0 {
			result.Warnings = append(result.Warnings,
				"Some secure boot keys have been revoked")
		}
	}

	// 检查 Flash 加密
	if info.IsFlashEncryptionEnabled() {
		result.IsFlashEncrypted = true
		result.Warnings = append(result.Warnings,
			"Flash Encryption enabled: plaintext firmware will be encrypted on write")
	}

	// 严格模式下的额外检查
	if level >= SafetyCheckStrict {
		if info.IsSecureBootEnabled() && !info.IsSecureDownloadMode() {
			// 安全启动但未启用安全下载模式，风险较高
			result.Warnings = append(result.Warnings,
				"Secure Boot without Secure Download Mode: verify firmware signature before flashing")
		}
	}

	// JTAG 被禁用时的警告
	if info.IsJTAGDisabled() {
		result.Warnings = append(result.Warnings,
			"JTAG is disabled: debugging may not be possible")
	}

	return result
}

// ============================================
// 烧录地址安全验证
// ============================================

// AddressRange 地址范围
type AddressRange struct {
	Start uint32
	End   uint32
	Name  string
}

// CriticalAddressRanges 关键地址范围（不应该随意覆盖）
var CriticalAddressRanges = []AddressRange{
	{0x0000, 0x1000, "Bootloader"},           // Bootloader
	{0x8000, 0x9000, "Partition Table"},       // 分区表
	{0x9000, 0xA000, "NVS Keys"},              // NVS 密钥
	{0xF000, 0x10000, "PHY Init Data"},        // PHY 初始化数据
}

// ValidateFlashAddress 验证烧录地址是否安全
func ValidateFlashAddress(offset uint32, size uint32, overwriteCritical bool) (bool, []string) {
	var warnings []string
	end := offset + size

	for _, r := range CriticalAddressRanges {
		// 检查是否与关键区域重叠
		if offset < r.End && end > r.Start {
			if !overwriteCritical {
				return false, []string{
					"Refusing to overwrite " + r.Name + " region (0x" +
						formatHex(r.Start) + " - 0x" + formatHex(r.End) + ")",
				}
			}
			warnings = append(warnings,
				"Overwriting "+r.Name+" region - this may brick the device!")
		}
	}

	return true, warnings
}

func formatHex(val uint32) string {
	const hexChars = "0123456789ABCDEF"
	var buf [8]byte
	for i := 7; i >= 0; i-- {
		buf[i] = hexChars[val&0xF]
		val >>= 4
	}
	return string(buf[:])
}

// ============================================
// 错误码定义
// ============================================

// RomError ROM 错误码
type RomError byte

const (
	RomErrInvalidMessage     RomError = 0x05
	RomErrFailed             RomError = 0x06
	RomErrInvalidCRC         RomError = 0x07
	RomErrFlashWriteErr      RomError = 0x08
	RomErrFlashReadErr       RomError = 0x09
	RomErrFlashReadLenErr    RomError = 0x0A
	RomErrDeflateErr         RomError = 0x0B
	RomErrNotSupported       RomError = 0x0C
	RomErrMemWriteErr        RomError = 0x0D
)

// ErrorString 返回错误描述
func (e RomError) ErrorString() string {
	switch e {
	case RomErrInvalidMessage:
		return "Invalid message"
	case RomErrFailed:
		return "Operation failed"
	case RomErrInvalidCRC:
		return "Invalid CRC"
	case RomErrFlashWriteErr:
		return "Flash write error"
	case RomErrFlashReadErr:
		return "Flash read error"
	case RomErrFlashReadLenErr:
		return "Flash read length error"
	case RomErrDeflateErr:
		return "Deflate error"
	case RomErrNotSupported:
		return "Not supported"
	case RomErrMemWriteErr:
		return "Memory write error"
	default:
		return "Unknown error"
	}
}
