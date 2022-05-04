//go:build malgo
// +build malgo

package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
)

func main() {
	// defer func() {
	// 	b := recover()
	// 	fmt.Println(b)
	// }()

	list, err := speaker.GetPlaybackDevices()
	if err != nil {
		panic(err)
	}
	for _, e := range list {
		fmt.Println(e.Name, "default:", e.IsDefault())
	}
	playbackSelect := func(deviceList []speaker.PlaybackDeviceInfo) *speaker.PlaybackDeviceInfo {
		for {
			fmt.Printf("choose a device from the list: (hitting enter without choosing will choose the default device)\n")
			fmt.Printf("==============================================================================================\n")
			defaultIndex := 0
			for i, d := range deviceList {
				fmt.Printf("%d: %s%s\n", i+1, d.Name,
					func(pd speaker.PlaybackDeviceInfo) string {
						if pd.IsDefault() {
							defaultIndex = i
							return " (DEFAULT)"
						}
						return ""
					}(d),
				)
			}

			var sindex string
			_, err := fmt.Scanf("%s\n", &sindex)
			if err != nil && err.Error() == "unexpected newline" {
				return &deviceList[defaultIndex]
			}
			if err != nil {
				fmt.Println("invalid selection, try again...", err)
				continue
			}
			if sindex == "q" {
				os.Exit(0)
			}
			index, _ := strconv.Atoi(sindex)
			if index > len(deviceList) {
				fmt.Printf("invalid selection [%d], try again...\n", index)
				continue
			}
			return &deviceList[index-1]
		}
	}
	playbackDeviceInfo := playbackSelect(list)
	fmt.Println("\nplayback device:", playbackDeviceInfo.Name, "\n")
	speaker.SetPlaybackDevice(*playbackDeviceInfo)

	list2, err2 := speaker.GetCaptureDevices()
	if err2 != nil {
		panic(err2)
	}
	for _, e := range list2 {
		fmt.Println(e.Name, "default:", e.IsDefault())
	}
	captureSelect := func(deviceList []speaker.CaptureDeviceInfo) *speaker.CaptureDeviceInfo {
		for {
			fmt.Printf("choose a device from the list: (hitting enter without choosing will choose the default device)\n")
			fmt.Printf("==============================================================================================\n")
			defaultIndex := 0
			for i, d := range deviceList {
				fmt.Printf("%d: %s%s\n", i+1, d.Name,
					func(pd speaker.CaptureDeviceInfo) string {
						if pd.IsDefault() {
							defaultIndex = i
							return " (DEFAULT)"
						}
						return ""
					}(d),
				)
			}

			var sindex string
			_, err := fmt.Scanf("%s\n", &sindex)
			if err != nil && err.Error() == "unexpected newline" {
				return &deviceList[defaultIndex]
			}
			if err != nil {
				fmt.Println("invalid selection, try again...", err)
				continue
			}
			if sindex == "q" {
				os.Exit(0)
			}
			index, _ := strconv.Atoi(sindex)
			if index > len(deviceList) {
				fmt.Printf("invalid selection [%d], try again...\n", index)
				continue
			}
			return &deviceList[index-1]
		}
	}

	captureDeviceInfo := captureSelect(list2)
	fmt.Println("\ncapture device:", captureDeviceInfo.Name, "\n")

	sr := beep.SampleRate(44000)
	captureStream, err := speaker.DeviceCapture(sr, *captureDeviceInfo, sr.N(time.Second/2), 2)
	if err != nil {
		panic(err)
	}
	gained := &effects.Gain{Streamer: captureStream, Gain: 0}

	speaker.Init(sr, sr.N(time.Second/10))
	// speaker.InitDeviceSelection(format.SampleRate, format.SampleRate.N(time.Second/10), nil)

	speaker.Play(gained)
	// done := make(chan bool)
	// speaker.Play(beep.Seq(streamer, beep.Callback(func() {
	// 	done <- true
	// })))

	// <-done
	// time.Sleep(10 * time.Second)
	fmt.Println("press any key to stop")
	input := bufio.NewScanner(os.Stdin)
	input.Scan()
}

func report(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
