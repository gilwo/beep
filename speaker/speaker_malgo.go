//go:build malgo
// +build malgo

// Package speaker implements playback of beep.Streamer values through physical speakers.
package speaker

import (
	"fmt"
	"log"
	"sync"

	"github.com/faiface/beep"
	"github.com/gen2brain/malgo"
	"github.com/glycerine/rbuf"
	"github.com/pkg/errors"
)

var (
	mu               sync.Mutex
	mixer            beep.Mixer
	samples          [][2]float64
	context          *malgo.AllocatedContext
	playerDeviceInfo PlaybackDeviceInfo = PlaybackDeviceInfo{info: malgo.DeviceInfo{}, Name: "default"}
	player           *malgo.Device
	done             chan struct{}
	buf              []byte
)

type PlaybackDeviceInfo struct {
	info malgo.DeviceInfo
	Name string
}

func (sd *PlaybackDeviceInfo) IsDefault() bool {
	return sd.info.IsDefault != 0
}

func init() {
	var err error
	context, err = malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		fmt.Printf("LOG <%v>\n", message)
	})
	errMsg := errors.Wrap(err, "failed to initialize speaker (context)")
	if errMsg != nil {
		panic(errMsg)
	}
}

type chooseDeviceCB func(deviceList []PlaybackDeviceInfo) *PlaybackDeviceInfo

// Init initializes audio playback through speaker. Must be called before using this package.
//
// The bufferSize argument specifies the number of samples of the speaker's buffer. Bigger
// bufferSize means lower CPU usage and more reliable playback. Lower bufferSize means better
// responsiveness and less delay.
func Init(sampleRate beep.SampleRate, bufferSize int) error {
	fmt.Println("using malgo")
	return InitDeviceSelection(sampleRate, bufferSize, nil)
}

func GetPlaybackDevices() ([]PlaybackDeviceInfo, error) {
	playbackDevices, err := context.Devices(malgo.Playback)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get playback device list (enumeration)")
	}
	playbackList := []PlaybackDeviceInfo{}
	for _, device := range playbackDevices {
		playbackList = append(playbackList, PlaybackDeviceInfo{device, device.Name()})
	}
	return playbackList, nil
}

func SetPlaybackDevice(device PlaybackDeviceInfo) {
	playerDeviceInfo = device
}

func configure(sampleRate beep.SampleRate, cb chooseDeviceCB) (malgo.DeviceConfig, error) {

	emptyInfo := malgo.DeviceInfo{}
	var deviceConfig malgo.DeviceConfig
	if playerDeviceInfo.Name == "default" && playerDeviceInfo.info == emptyInfo {
		deviceConfig = malgo.DefaultDeviceConfig(malgo.Playback)
	} else {
		deviceConfig = malgo.DeviceConfig{}
		deviceConfig.DeviceType = malgo.Playback
		deviceConfig.Playback.DeviceID = playerDeviceInfo.info.ID.Pointer()
	}
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = 2 //channels
	deviceConfig.SampleRate = uint32(sampleRate)
	deviceConfig.Alsa.NoMMap = 1

	if cb != nil {
		playbackDevices, err := context.Devices(malgo.Playback)
		if err != nil {
			return malgo.DeviceConfig{}, err
		}
		speakerList := []PlaybackDeviceInfo{}
		for _, device := range playbackDevices {
			speakerList = append(speakerList, PlaybackDeviceInfo{device, device.Name()})
		}
		ret := cb(speakerList)
		if ret != nil {
			deviceConfig.Playback.DeviceID = ret.info.ID.Pointer()
		}
	}
	return deviceConfig, nil
}

func InitDeviceSelection(sampleRate beep.SampleRate, bufferSize int, cb chooseDeviceCB) error {
	mu.Lock()
	defer mu.Unlock()

	Close()

	mixer = beep.Mixer{}

	samples = make([][2]float64, bufferSize)

	var err error
	var deviceConfig malgo.DeviceConfig
	deviceConfig, err = configure(sampleRate, cb)
	if err != nil {
		return errors.Wrap(err, "failed to initialize speaker (configure)")
	}

	onSamples := func(pOutputSample, pInputSamples []byte, framecount uint32) {
		byteCount := framecount * deviceConfig.Playback.Channels * uint32(malgo.SampleSizeInBytes(deviceConfig.Playback.Format))
		if len(buf) < int(byteCount) {
			update()
		}
		copy(pOutputSample, buf[:byteCount])
		buf = append([]byte{}, buf[byteCount:]...)
	}

	deviceCallbacks := malgo.DeviceCallbacks{
		Data: onSamples,
		Stop: func() {
			log.Println("stop playback device requested")
		},
	}
	player, err = malgo.InitDevice(context.Context, deviceConfig, deviceCallbacks)
	if err != nil {
		return errors.Wrap(err, "failed to initialize speaker (player)")
	}

	err = player.Start()
	if err != nil {
		return errors.Wrap(err, "failed to initialize speaker (player start)")
	}

	done = make(chan struct{})

	go func() {
		for {
			select {
			default:
			case <-done:
				player.Stop()
				return
			}
		}
	}()

	return nil
}

// Close closes the playback and the driver. In most cases, there is certainly no need to call Close
// even when the program doesn't play anymore, because in properly set systems, the default mixer
// handles multiple concurrent processes. It's only when the default device is not a virtual but hardware
// device, that you'll probably want to manually manage the device from your application.
func Close() {
	if player != nil {
		if done != nil {
			done <- struct{}{}
			done = nil
		}
		player.Stop()
		player.Uninit()
		player = nil
	}
}

// Lock locks the speaker. While locked, speaker won't pull new data from the playing Streamers. Lock
// if you want to modify any currently playing Streamers to avoid race conditions.
//
// Always lock speaker for as little time as possible, to avoid playback glitches.
func Lock() {
	mu.Lock()
}

// Unlock unlocks the speaker. Call after modifying any currently playing Streamer.
func Unlock() {
	mu.Unlock()
}

// Play starts playing all provided Streamers through the speaker.
func Play(s ...beep.Streamer) {
	mu.Lock()
	mixer.Add(s...)
	mu.Unlock()
}

// Clear removes all currently playing Streamers from the speaker.
func Clear() {
	mu.Lock()
	mixer.Clear()
	mu.Unlock()
}

// update pulls new data from the playing Streamers and sends it to the speaker. Blocks until the
// data is sent and started playing.
func update() {
	mu.Lock()
	mixer.Stream(samples)
	mu.Unlock()

	for i := range samples {
		for c := range samples[i] {
			val := samples[i][c]
			if val < -1 {
				val = -1
			}
			if val > +1 {
				val = +1
			}
			valInt16 := int16(val * (1<<15 - 1))
			low := byte(valInt16)
			high := byte(valInt16 >> 8)
			buf = append(buf, low, high)
		}
	}
}

func DefaultCaptureDevice() CaptureDeviceInfo {
	return CaptureDeviceInfo{
		Name: "default",
		info: malgo.DeviceInfo{},
	}
}

func configureCapture(sampleRate beep.SampleRate, device CaptureDeviceInfo, numChannels int) malgo.DeviceConfig {
	emptyInfo := malgo.DeviceInfo{}
	var deviceConfig malgo.DeviceConfig
	if device.Name == "default" && device.info == emptyInfo {
		deviceConfig = malgo.DefaultDeviceConfig(malgo.Capture)
	} else {
		deviceConfig = malgo.DeviceConfig{}
		deviceConfig.DeviceType = malgo.Capture
		deviceConfig.Capture.DeviceID = device.info.ID.Pointer()
	}
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = uint32(numChannels)
	deviceConfig.SampleRate = uint32(sampleRate)
	deviceConfig.Alsa.NoMMap = 1
	return deviceConfig
}

type CaptureDeviceInfo struct {
	info malgo.DeviceInfo
	Name string
}

func (sd *CaptureDeviceInfo) IsDefault() bool {
	return sd.info.IsDefault != 0
}

type deviceCapture struct {
	device *malgo.Device
	rbuf   *rbuf.FixedSizeRingBuf
	buf    []byte
	ready  chan (struct{})
	more   bool
}

// Creates a streamer which will stream audio from a capture device
// sampleRate - ....
// buf - ...
func DeviceCapture(sr beep.SampleRate, device CaptureDeviceInfo, bufferSize int, numChannels int) (beep.Streamer, error) {
	deviceConfig := configureCapture(sr, device, numChannels)

	capture := &deviceCapture{
		rbuf:  rbuf.NewFixedSizeRingBuf(bufferSize * numChannels),
		buf:   make([]byte, bufferSize*numChannels, bufferSize*numChannels),
		ready: make(chan struct{}),
		more:  false,
	}

	onCapSamples := func(pOutputSample, pInputSamples []byte, framecount uint32) {
		byteCount := framecount * deviceConfig.Capture.Channels * uint32(malgo.SampleSizeInBytes(deviceConfig.Capture.Format))

		_, err := capture.rbuf.WriteAndMaybeOverwriteOldestData(pInputSamples[:byteCount])
		if err != nil {
			log.Println("got an error writing to rbuf: ", err)
			log.Println("TODO: add recovery here: reset rbuf and try again...")
		}
		if capture.more {
			capture.more = false
			capture.ready <- struct{}{}
		}
	}

	deviceCallbacks := malgo.DeviceCallbacks{
		Data: onCapSamples,
		Stop: func() {
			log.Println("stop capture device requested")
		},
	}
	var err error
	capture.device, err = malgo.InitDevice(context.Context, deviceConfig, deviceCallbacks)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize capture (device)")
	}

	err = capture.device.Start()
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize capture (device start)")
	}

	return capture, nil
}

func (g *deviceCapture) Stream(samples [][2]float64) (n int, ok bool) {
	samplesSize := int(g.device.CaptureChannels()) * len(samples) * malgo.SampleSizeInBytes(g.device.CaptureFormat())
	buf := make([]byte, samplesSize, samplesSize)

	// if there is not enough data in the buffer let the writer know and wait for signal...
	if g.rbuf.Avail() == 0 {
		g.more = true
		<-g.ready
	}

	readCount, err := g.rbuf.ReadAndMaybeAdvance(buf, true)
	if err != nil {
		log.Println("error in rbuf read and maybe advance:", err)
		log.Println("TODO: figure out how to recover from this case")
		log.Fatalln("ouch")
	}

	sampleLen := len(samples)
	count := sampleLen
	actLen := sampleLen
	if readCount < sampleLen {
		actLen = readCount
		count = readCount
	}
	if g.device.CaptureChannels() == 1 {
		for i := range samples {
			e1 := buf[i*2]
			e2 := buf[i*2+1]
			val := float64(int16(e1)+int16(e2)*(1<<8)) / (1<<16 - 1)
			samples[i][0] = val
			samples[i][1] = val
			count -= 1
			if count == 0 {
				break
			}
		}
	} else if g.device.CaptureChannels() == 2 {
		for i := range samples {
			e1 := buf[i*4]
			e2 := buf[i*4+1]
			e3 := buf[i*4+1]
			e4 := buf[i*4+1]
			val1 := float64(int16(e1)+int16(e2)*(1<<8)) / (1<<16 - 1)
			val2 := float64(int16(e3)+int16(e4)*(1<<8)) / (1<<16 - 1)
			samples[i][0] = val1
			samples[i][1] = val2
			count -= 1
			if count == 0 {
				break
			}
		}
	}
	return actLen, true
}

func (*deviceCapture) Err() error {
	return nil
}

func GetCaptureDevices() ([]CaptureDeviceInfo, error) {
	captureDevices, err := context.Devices(malgo.Capture)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get capture device list (enumeration)")
	}
	captureList := []CaptureDeviceInfo{}
	for _, device := range captureDevices {
		captureList = append(captureList, CaptureDeviceInfo{device, device.Name()})
	}
	return captureList, nil
}
