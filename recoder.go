package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	sampleRate        = 8000
	bitsPerSample     = 8
	channels          = 1 // Mono
	minRecordDuration = 2 * time.Second
)

// WAVHeader 定义WAV文件头结构
type WAVHeader struct {
	RIFFID        [4]byte // "RIFF"
	FileSize      uint32
	WAVEID        [4]byte // "WAVE"
	FMTID         [4]byte // "fmt "
	FMTSize       uint32  // 16 for PCM
	AudioFormat   uint16  // 1 for PCM
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
	DATAID        [4]byte // "data"
	DataSize      uint32
}

// createWAVHeader 创建WAV文件头
func createWAVHeader(dataSize uint32) WAVHeader {
	byteRate := uint32(sampleRate * channels * bitsPerSample / 8)
	blockAlign := uint16(channels * bitsPerSample / 8)

	return WAVHeader{
		RIFFID:        [4]byte{'R', 'I', 'F', 'F'},
		FileSize:      36 + dataSize, // 36 bytes for header + dataSize
		WAVEID:        [4]byte{'W', 'A', 'V', 'E'},
		FMTID:         [4]byte{'f', 'm', 't', ' '},
		FMTSize:       16,
		AudioFormat:   1, // PCM
		NumChannels:   channels,
		SampleRate:    sampleRate,
		ByteRate:      byteRate,
		BlockAlign:    blockAlign,
		BitsPerSample: bitsPerSample,
		DATAID:        [4]byte{'d', 'a', 't', 'a'},
		DataSize:      dataSize,
	}
}

// Recorder 结构体用于管理录音过程
type Recorder struct {
	speakerCallsign string
	outputDir       string
	mu              sync.Mutex
	currentBuffer   *bytes.Buffer
	recordStartTime time.Time
	isRecording     bool
	lastDataTime    time.Time
}

// NewRecorder 创建一个新的Recorder实例
func NewRecorder(speakerCallsign, baseOutputDir string) *Recorder {
	return &Recorder{
		speakerCallsign: speakerCallsign,
		outputDir:       baseOutputDir,
		currentBuffer:   new(bytes.Buffer),
	}
}

// ProcessPCMData 处理传入的PCM数据
func (r *Recorder) ProcessPCMData(data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 如果当前没有录音，并且数据长度大于0，则开始记录
	if !r.isRecording && len(data) > 0 {
		r.recordStartTime = time.Now()
		r.isRecording = true
		r.currentBuffer.Reset() // 清空缓冲区，准备开始新录音
		fmt.Printf("[%s] 开始记录...\n", r.speakerCallsign)
	}

	if r.isRecording {
		// 写入数据
		r.currentBuffer.Write(data)
		r.lastDataTime = time.Now()

		// 检查是否需要结束当前录音并保存
		r.checkAndSaveRecord()
	}
}

// checkAndSaveRecord 检查是否需要保存当前录音
func (r *Recorder) checkAndSaveRecord() {
	if !r.isRecording {
		return
	}

	// 计算当前录音时长
	currentDuration := time.Since(r.recordStartTime)

	// 如果数据停止超过2秒，或者当前录音时长超过某个阈值（例如，我们可以设定最大文件时长）
	// 这里我们主要关注数据停止超过2秒的情况
	if time.Since(r.lastDataTime) > minRecordDuration {
		r.saveCurrentRecord(currentDuration)
		r.isRecording = false
		fmt.Printf("[%s] 结束记录。\n", r.speakerCallsign)
	}
}

// saveCurrentRecord 保存当前录音到文件
func (r *Recorder) saveCurrentRecord(duration time.Duration) {
	if r.currentBuffer.Len() == 0 {
		return // 没有数据，无需保存
	}

	// 确保录音时长大于最小记录时长
	if duration < minRecordDuration {
		fmt.Printf("[%s] 录音时长不足 %.2f 秒，不保存。\n", r.speakerCallsign, minRecordDuration.Seconds())
		return
	}

	// 获取日期作为子目录
	today := time.Now().Format("2006-01-02")
	dayOutputDir := filepath.Join(r.outputDir, today)
	if err := os.MkdirAll(dayOutputDir, 0755); err != nil {
		fmt.Printf("创建目录 %s 失败: %v\n", dayOutputDir, err)
		return
	}

	// 构造文件名
	startTimeStr := r.recordStartTime.Format("150405") // HHMMSS
	durationSeconds := int(duration.Seconds())
	filename := fmt.Sprintf("%s_%s_%dms.wav", r.speakerCallsign, startTimeStr, durationSeconds*1000)
	filePath := filepath.Join(dayOutputDir, filename)

	fmt.Printf("[%s] 保存录音到: %s (时长: %.2f秒)\n", r.speakerCallsign, filePath, duration.Seconds())

	// 创建WAV文件
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Printf("创建文件 %s 失败: %v\n", filePath, err)
		return
	}
	defer file.Close()

	// 计算PCM数据大小
	pcmDataSize := uint32(r.currentBuffer.Len())

	// 创建并写入WAV头
	wavHeader := createWAVHeader(pcmDataSize)
	if err := binary.Write(file, binary.LittleEndian, wavHeader); err != nil {
		fmt.Printf("写入WAV头失败: %v\n", err)
		return
	}

	// 写入PCM数据
	if _, err := io.Copy(file, r.currentBuffer); err != nil {
		fmt.Printf("写入PCM数据失败: %v\n", err)
		return
	}

	// 清空缓冲区以便下一次录音
	r.currentBuffer.Reset()
}

// Stop 停止录音，并保存所有未保存的数据
func (r *Recorder) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isRecording {
		r.saveCurrentRecord(time.Since(r.recordStartTime))
		r.isRecording = false
	}
	fmt.Printf("[%s] 录音器停止。\n", r.speakerCallsign)
}

func main2() {
	// 示例用法
	baseOutputDir := "recordings"
	if err := os.MkdirAll(baseOutputDir, 0755); err != nil {
		fmt.Printf("创建基础输出目录 %s 失败: %v\n", baseOutputDir, err)
		return
	}

	// 添加调试日志
	absPath, _ := filepath.Abs(baseOutputDir)
	fmt.Printf("基础输出目录绝对路径: %s\n", absPath)

	// 创建一个文件用于测试写入权限
	testFile := filepath.Join(baseOutputDir, "test.txt")
	fmt.Printf("创建测试文件: %s\n", testFile)
	f, err := os.Create(testFile)
	if err != nil {
		fmt.Printf("创建测试文件失败: %v\n", err)
	} else {
		f.WriteString("测试文件写入成功\n")
		f.Close()
	}

	// 修改录音器实例
	// 添加更详细的日志输出已被移除，使用原有方式
	// 修改录音器实例
	// 添加更详细的日志输出已被移除，使用原有方式
	recorder := NewRecorder("Speaker1", baseOutputDir)

	// 在主程序结束前打印所有文件
	go func() {
		<-time.After(35 * time.Second)
		fmt.Println("最终目录文件列表:")
		files, _ := os.ReadDir(baseOutputDir)
		for _, f := range files {
			info, _ := f.Info()
			fmt.Printf("- %s (%d bytes)\n", f.Name(), info.Size())

			// 如果是目录，打印目录中的文件
			if f.IsDir() {
				fmt.Printf("  %s 子目录内容:\n", f.Name())
				subFiles, _ := os.ReadDir(filepath.Join(baseOutputDir, f.Name()))
				for _, sf := range subFiles {
					subInfo, _ := sf.Info()
					fmt.Printf("  - %s (%d bytes)\n", sf.Name(), subInfo.Size())
				}
			}
		}
	}()

	// 模拟实时PCM数据输入
	go func() {
		// 模拟一段短数据 (小于2秒，不应该被保存)
		fmt.Println("模拟短数据输入 (1秒)...")
		for i := 0; i < sampleRate*1; i++ { // 1秒数据
			recorder.ProcessPCMData([]byte{byte(i % 256)}) // 模拟8位PCM数据
			time.Sleep(1 * time.Millisecond)
		}
		time.Sleep(3 * time.Second) // 停顿，模拟间歇性

		// 模拟一段长数据 (大于2秒，应该被保存)
		fmt.Println("模拟长数据输入 (3秒)...")
		for i := 0; i < sampleRate*3; i++ { // 3秒数据
			recorder.ProcessPCMData([]byte{byte(i % 256)})
			time.Sleep(1 * time.Millisecond)
		}
		time.Sleep(3 * time.Second) // 停顿，模拟间歇性

		// 模拟另一段长数据，并再次停顿，以触发新文件创建
		fmt.Println("模拟第二段长数据输入 (2.5秒)...")
		for i := 0; i < sampleRate*2+sampleRate/2; i++ { // 2.5秒数据
			recorder.ProcessPCMData([]byte{byte(i % 256)})
			time.Sleep(1 * time.Millisecond)
		}
		time.Sleep(3 * time.Second) // 停顿，模拟间歇性

		// 模拟短数据，但这次因为它前面有停顿，所以会触发一次保存（如果前面有足够长的数据）
		fmt.Println("模拟短数据输入 (1秒) after a pause...")
		for i := 0; i < sampleRate*1; i++ { // 1秒数据
			recorder.ProcessPCMData([]byte{byte(i % 256)})
			time.Sleep(1 * time.Millisecond)
		}
		time.Sleep(3 * time.Second) // 停顿，确保所有数据都已处理

		recorder.Stop() // 停止录音器，保存剩余的可能数据
	}()

	// 保持主程序运行，直到所有模拟数据处理完毕
	time.Sleep(35 * time.Second) // 等待足够长时间让所有数据保存
	fmt.Println("模拟结束。")

	// 打印最终目录文件列表
	fmt.Println("最终目录文件列表:")
	files, _ := os.ReadDir(baseOutputDir)
	for _, f := range files {
		info, _ := f.Info()
		fmt.Printf("- %s (%d bytes)\n", f.Name(), info.Size())

		// 如果是目录，打印目录中的文件
		if f.IsDir() {
			fmt.Printf("  %s 子目录内容:\n", f.Name())
			subFiles, _ := os.ReadDir(filepath.Join(baseOutputDir, f.Name()))
			for _, sf := range subFiles {
				subInfo, _ := sf.Info()
				fmt.Printf("  - %s (%d bytes)\n", sf.Name(), subInfo.Size())
			}
		}
	}
}
