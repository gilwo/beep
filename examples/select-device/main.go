//go:build malgo
// +build malgo

package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

func main() {
	defer func() {
		b := recover()
		fmt.Println(b)
	}()
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s song.mp3\n", os.Args[0])
		os.Exit(1)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		report(err)
	}
	streamer, format, err := mp3.Decode(f)
	if err != nil {
		report(err)
	}
	defer streamer.Close()

	list, err := speaker.GetPlaybackDevices()
	if err != nil {
		panic(err)
	}
	for _, e := range list {
		fmt.Println(e.Name, "default:", e.IsDefault())
	}
	deviceSelect := func(deviceList []speaker.PlaybackDeviceInfo) *speaker.PlaybackDeviceInfo {
		for {
			fmt.Printf("choose a device from the list: (hitting enter without choosing will choose the default device)\n")
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
			_, err := fmt.Scanf("%s", &sindex)
			if err.Error() == "unexpected newline" {
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
	selectedDevice := deviceSelect(list)
	speaker.SetPlaybackDevice(*selectedDevice)

	speaker.InitDeviceSelection(format.SampleRate, format.SampleRate.N(time.Second/10), nil)

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))

	<-done
}

func report(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
