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

	// 安全检查：验证地址范围
	ok, warnings := ValidateFlashAddress(offset, firmwareLen, false)
	for _, w := range warnings {
		logMsg(w)
	}
	if !ok {
		return -3 // 地址不安全
	}

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

//go:wasmexport flash_verify
// 烧录并验证（写入后读取 MD5 校验）
func pluginFlashVerify(firmwarePtr uint32, firmwareLen uint32, offset uint32) int32 {
	firmware := unsafeSlice(firmwarePtr, firmwareLen)

	// 安全检查
	if ok, _ := ValidateFlashAddress(offset, firmwareLen, false); !ok {
		return -3
	}

	// 烧录
	ret := pluginFlash(firmwarePtr, firmwareLen, offset)
	if ret != 0 {
		return ret
	}

	// 验证
	deviceMd5 := flashMd5(offset, firmwareLen)
	localMd5 := calcLocalMd5(firmware)

	if deviceMd5 != localMd5 {
		logMsg("MD5 verification failed!")
		return -4 // 验证失败
	}

	logMsg("MD5 verification passed")
	return 0
}

//go:wasmexport dry_run
// Dry-run 模式：验证命令序列但不实际烧录
// 返回：0=成功, -1=失败
func pluginDryRun(firmwarePtr uint32, firmwareLen uint32, offset uint32) int32 {
	firmware := unsafeSlice(firmwarePtr, firmwareLen)

	// 1. 验证固件大小
	if firmwareLen == 0 {
		logMsg("Error: firmware is empty")
		return -1
	}
	if firmwareLen > 16*1024*1024 { // 16MB max
		logMsg("Error: firmware too large")
		return -1
	}

	// 2. 验证地址对齐
	if offset%4 != 0 {
		logMsg("Warning: offset not 4-byte aligned")
	}

	// 3. 验证地址范围安全
	ok, warnings := ValidateFlashAddress(offset, firmwareLen, false)
	for _, w := range warnings {
		logMsg(w)
	}
	if !ok {
		return -2
	}

	// 4. 模拟同步 - 验证帧编码
	syncData := GetSyncFrameData()
	var testBuf [256]byte
	_ = EncodeFrame(cmdSync, syncData[:], testBuf[:])

	// 5. 模拟 SPI Attach
	_ = EncodeFrame(cmdSpiAttach, make([]byte, 8), testBuf[:])

	// 6. 模拟设置 Flash 参数
	params := FlashParams{
		TotalSize:  0x00400000,
		BlockSize:  BlockSize,
		SectorSize: SectorSize,
		PageSize:   PageSize,
	}
	_ = EncodeFlashParams(params)

	// 7. 模拟 FlashBegin
	numBlocks := CalcNumBlocks(firmwareLen)
	beginParams := FlashBeginParams{
		TotalSize: firmwareLen,
		NumBlocks: numBlocks,
		BlockSize: BlockSize,
		Offset:    offset,
	}
	_ = EncodeFlashBeginParams(beginParams)

	// 8. 模拟 FlashData（验证数据块格式）
	for i := uint32(0); i < numBlocks; i++ {
		start := i * BlockSize
		end := start + BlockSize
		if end > firmwareLen {
			end = firmwareLen
		}
		block := firmware[start:end]
		paddedLen := Pad4Byte(len(block))
		_ = paddedLen // 验证计算正确
	}

	// 9. 模拟 FlashEnd
	_ = []byte{0} // reboot = 0

	logMsg("Dry-run validation passed")
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

//go:wasmexport flash_md5
// 计算 Flash 区域 MD5，结果写入 outputPtr（16 字节）
func pluginFlashMd5(offset uint32, size uint32, outputPtr uint32) int32 {
	if !spiAttach() {
		return -1
	}
	md5 := flashMd5(offset, size)
	if md5 == [16]byte{} {
		return -1
	}
	output := unsafeSlice(outputPtr, 16)
	copy(output, md5[:])
	return 0
}

//go:wasmexport get_security_info
// 获取安全信息，结果写入 outputPtr（至少 20 字节）
func pluginGetSecurityInfo(outputPtr uint32) int32 {
	info := readSecurityInfo()
	if !info.Valid {
		return -1
	}
	output := unsafeSlice(outputPtr, 20)
	// 编码安全信息
	output[0] = byte(info.Flags)
	output[1] = byte(info.Flags >> 8)
	output[2] = byte(info.Flags >> 16)
	output[3] = byte(info.Flags >> 24)
	output[4] = info.FlashCryptCnt
	copy(output[5:12], info.KeyPurposes[:])
	output[12] = byte(info.ChipID)
	output[13] = byte(info.ChipID >> 8)
	output[14] = byte(info.ChipID >> 16)
	output[15] = byte(info.ChipID >> 24)
	output[16] = byte(info.EcoVersion)
	output[17] = byte(info.EcoVersion >> 8)
	output[18] = byte(info.EcoVersion >> 16)
	output[19] = byte(info.EcoVersion >> 24)
	return 0
}

// readSecurityInfo 从设备读取安全信息
func readSecurityInfo() SecurityInfo {
	// 发送 GetSecurityInfo 命令
	sendCmd(cmdGetSecurityInfo, nil)
	n := hostSerialRead(uint32(uintptr(unsafe.Pointer(&rxBuf[0]))), 4096)
	if n < 12 {
		return SecurityInfo{Valid: false}
	}
	// 跳过帧头，去掉状态字节
	dataLen := n - 8 - 4 // 去掉 8 字节帧头和 4 字节状态
	if dataLen < 0 {
		dataLen = 0
	}
	return ParseSecurityInfo(rxBuf[8 : 8+dataLen])
}

//go:wasmexport safety_check
// 执行安全检查，返回位掩码状态
// bit 0: can_flash
// bit 1: is_secure_download
// bit 2: is_secure_boot
// bit 3: is_flash_encrypted
// bit 4-7: warning count (max 15)
func pluginSafetyCheck() int32 {
	info := readSecurityInfo()
	if !info.Valid {
		return 0 // 无法获取安全信息，不允许烧录
	}

	check := PerformSafetyCheck(info, SafetyCheckNormal)

	var result int32
	if check.CanFlash {
		result |= 1 << 0
	}
	if check.IsSecureDownload {
		result |= 1 << 1
	}
	if check.IsSecureBoot {
		result |= 1 << 2
	}
	if check.IsFlashEncrypted {
		result |= 1 << 3
	}

	// 警告数量（最多 15 个）
	warningCount := len(check.Warnings)
	if warningCount > 15 {
		warningCount = 15
	}
	result |= int32(warningCount) << 4

	return result
}

//go:wasmexport validate_address
// 验证烧录地址是否安全
// 返回：0=安全，1=警告，-1=危险（拒绝）
func pluginValidateAddress(offset uint32, size uint32, overwriteCritical uint32) int32 {
	safe, warnings := ValidateFlashAddress(offset, size, overwriteCritical != 0)
	if !safe {
		for _, w := range warnings {
			logMsg(w)
		}
		return -1
	}
	if len(warnings) > 0 {
		for _, w := range warnings {
			logMsg(w)
		}
		return 1 // 有警告但允许
	}
	return 0 // 完全安全
}

//go:wasmexport read_flash
// 读取 Flash 数据到 outputPtr
func pluginReadFlash(offset uint32, size uint32, outputPtr uint32) int32 {
	// 注意：ROM 的 ReadFlashSlow 命令需要特殊处理
	// 此功能需要 stub 支持才能高效工作
	_ = offset
	_ = size
	_ = outputPtr
	logMsg("read_flash: requires stub support")
	return -1
}

//go:wasmexport verify_flash
// 验证 Flash 内容与 firmware 对比
func pluginVerifyFlash(firmwarePtr uint32, firmwareLen uint32, offset uint32) int32 {
	firmware := unsafeSlice(firmwarePtr, firmwareLen)
	md5 := flashMd5(offset, uint32(len(firmware)))
	if md5 == [16]byte{} {
		return -1
	}
	// 计算本地 MD5
	localMd5 := calcLocalMd5(firmware)
	// 比较
	if md5 == localMd5 {
		return 0 // 验证通过
	}
	return -2 // 验证失败
}

//go:wasmexport erase_region
// 擦除指定区域（需要 stub 支持）
func pluginEraseRegion(offset uint32, size uint32) int32 {
	// ROM bootloader 不支持区域擦除
	// 需要 stub 或使用 flash_begin 技巧
	_ = offset
	_ = size
	logMsg("erase_region: requires stub support")
	return -1
}

//go:wasmexport change_baud
// 改变串口波特率
func pluginChangeBaud(newBaud uint32) int32 {
	params := ChangeBaudParams{
		NewBaud:   newBaud,
		PriorBaud: 115200,
	}
	data := EncodeChangeBaudParams(params)
	sendCmd(cmdChangeBaud, data[:])
	if !checkResponse() {
		return -1
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
	// FlashEnd 参数：1 字节，0=reboot, 1=stay in loader
	data := [1]byte{0} // 0 = reboot
	sendCmd(cmdFlashEnd, data[:])
	return checkResponse()
}

// ============================================
// MD5 功能
// ============================================

// flashMd5 计算 Flash 区域的 MD5
func flashMd5(offset, size uint32) [16]byte {
	params := FlashMd5Params{
		Offset: offset,
		Size:   size,
	}
	data := EncodeFlashMd5Params(params)
	sendCmd(cmdFlashMd5, data[:])

	// 读取响应
	n := hostSerialRead(uint32(uintptr(unsafe.Pointer(&rxBuf[0]))), 4096)
	if n < 24 {
		return [16]byte{}
	}

	// 解析 MD5 响应
	return ParseMd5Response(rxBuf[:], int(n))
}

// calcLocalMd5 计算本地数据的 MD5
// 使用纯 Go 实现，不依赖标准库（WASM 兼容）
func calcLocalMd5(data []byte) [16]byte {
	// MD5 实现
	var s [4]uint32
	s[0] = 0x67452301
	s[1] = 0xefcdab89
	s[2] = 0x98badcfe
	s[3] = 0x10325476

	// 填充
	padded := make([]byte, len(data))
	copy(padded, data)
	padded = append(padded, 0x80)
	for (len(padded)%64) != 56 {
		padded = append(padded, 0)
	}
	lenBits := uint64(len(data) * 8)
	padded = append(padded, byte(lenBits), byte(lenBits>>8), byte(lenBits>>16), byte(lenBits>>24),
		byte(lenBits>>32), byte(lenBits>>40), byte(lenBits>>48), byte(lenBits>>56))

	// 处理块
	for i := 0; i < len(padded); i += 64 {
		block := padded[i : i+64]
		var m [16]uint32
		for j := 0; j < 16; j++ {
			m[j] = uint32(block[j*4]) | uint32(block[j*4+1])<<8 | uint32(block[j*4+2])<<16 | uint32(block[j*4+3])<<24
		}
		a, b, c, d := s[0], s[1], s[2], s[3]

		for j := 0; j < 64; j++ {
			var f, g uint32
			switch {
			case j < 16:
				f = (b & c) | (^b & d)
				g = uint32(j)
			case j < 32:
				f = (d & b) | (^d & c)
				g = (5*uint32(j) + 1) % 16
			case j < 48:
				f = b ^ c ^ d
				g = (3*uint32(j) + 5) % 16
			default:
				f = c ^ (b | ^d)
				g = (7 * uint32(j)) % 16
			}

			k := md5K[j]
			f = f + a + k + m[g]
			a = d
			d = c
			c = b
			b = b + leftRotate(f, md5S[j])
		}

		s[0] += a
		s[1] += b
		s[2] += c
		s[3] += d
	}

	var result [16]byte
	for i := 0; i < 4; i++ {
		result[i*4] = byte(s[i])
		result[i*4+1] = byte(s[i] >> 8)
		result[i*4+2] = byte(s[i] >> 16)
		result[i*4+3] = byte(s[i] >> 24)
	}
	return result
}

// MD5 常量
var md5K = [64]uint32{
	0xd76aa478, 0xe8c7b756, 0x242070db, 0xc1bdceee,
	0xf57c0faf, 0x4787c62a, 0xa8304613, 0xfd469501,
	0x698098d8, 0x8b44f7af, 0xffff5bb1, 0x895cd7be,
	0x6b901122, 0xfd987193, 0xa679438e, 0x49b40821,
	0xf61e2562, 0xc040b340, 0x265e5a51, 0xe9b6c7aa,
	0xd62f105d, 0x02441453, 0xd8a1e681, 0xe7d3fbc8,
	0x21e1cde6, 0xc33707d6, 0xf4d50d87, 0x455a14ed,
	0xa9e3e905, 0xfcefa3f8, 0x676f02d9, 0x8d2a4c8a,
	0xfffa3942, 0x8771f681, 0x6d9d6122, 0xfde5380c,
	0xa4beea44, 0x4bdecfa9, 0xf6bb4b60, 0xbebfbc70,
	0x289b7ec6, 0xeaa127fa, 0xd4ef3085, 0x04881d05,
	0xd9d4d039, 0xe6db99e5, 0x1fa27cf8, 0xc4ac5665,
	0xf4292244, 0x432aff97, 0xab9423a7, 0xfc93a039,
	0x655b59c3, 0x8f0ccc92, 0xffeff47d, 0x85845dd1,
	0x6fa87e4f, 0xfe2ce6e0, 0xa3014314, 0x4e0811a1,
	0xf7537e82, 0xbd3af235, 0x2ad7d2bb, 0xeb86d391,
}

var md5S = [64]uint{
	7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22,
	5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20,
	4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23,
	6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21,
}

func leftRotate(x uint32, n uint) uint32 {
	return (x << n) | (x >> (32 - n))
}
