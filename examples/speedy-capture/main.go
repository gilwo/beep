package main

import (
	"fmt"
	"os"
	"time"
	"unicode"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	"github.com/gdamore/tcell"
)

func drawTextLine(screen tcell.Screen, x, y int, s string, style tcell.Style) {
	for _, r := range s {
		screen.SetContent(x, y, r, nil, style)
		x++
	}
}

type audioPanel struct {
	sampleRate beep.SampleRate
	streamer   beep.Streamer
	gain       *effects.Gain
	ctrl       *beep.Ctrl
	resampler  *beep.Resampler
	volume     *effects.Volume
}

func newAudioPanel(sampleRate beep.SampleRate, streamer beep.Streamer) *audioPanel {
	gain := &effects.Gain{Streamer: streamer, Gain: 0} // normal on start
	ctrl := &beep.Ctrl{Streamer: gain}
	resampler := beep.ResampleRatio(4, 1, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2}
	return &audioPanel{sampleRate, streamer, gain, ctrl, resampler, volume}
}

func (ap *audioPanel) play() {
	speaker.Play(ap.volume)
}

func (ap *audioPanel) draw(screen tcell.Screen) {
	mainStyle := tcell.StyleDefault.
		Background(tcell.NewHexColor(0x473437)).
		Foreground(tcell.NewHexColor(0xD7D8A2))
	statusStyle := mainStyle.
		Foreground(tcell.NewHexColor(0xDDC074)).
		Bold(true)

	screen.Fill(' ', mainStyle)

	drawTextLine(screen, 0, 0, "Welcome to the Speedy Player!", mainStyle)
	drawTextLine(screen, 0, 1, "Press [ESC] to quit.", mainStyle)
	drawTextLine(screen, 0, 2, "Press [SPACE] to pause/resume.", mainStyle)
	drawTextLine(screen, 0, 3, "Use keys in (?/?) to turn the buttons.", mainStyle)

	speaker.Lock()
	gain := ap.gain.Gain
	volume := ap.volume.Volume
	speed := ap.resampler.Ratio()
	speaker.Unlock()

	gainStatus := fmt.Sprintf("%.1f", gain)
	volumeStatus := fmt.Sprintf("%.1f", volume)
	speedStatus := fmt.Sprintf("%.3fx", speed)

	drawTextLine(screen, 0, 6, "Gain     (Q/W):", mainStyle)
	drawTextLine(screen, 16, 6, gainStatus, statusStyle)

	drawTextLine(screen, 0, 7, "Volume   (A/S):", mainStyle)
	drawTextLine(screen, 16, 7, volumeStatus, statusStyle)

	drawTextLine(screen, 0, 8, "Speed    (Z/X):", mainStyle)
	drawTextLine(screen, 16, 8, speedStatus, statusStyle)
}

func (ap *audioPanel) handle(event tcell.Event) (changed, quit bool) {
	switch event := event.(type) {
	case *tcell.EventKey:
		if event.Key() == tcell.KeyESC {
			return false, true
		}

		if event.Key() != tcell.KeyRune {
			return false, false
		}

		switch unicode.ToLower(event.Rune()) {
		case ' ':
			speaker.Lock()
			ap.ctrl.Paused = !ap.ctrl.Paused
			speaker.Unlock()
			return false, false

		case 'q':
			speaker.Lock()
			if ap.gain.Gain > 1 {
				ap.gain.Gain -= 1.0
			}
			speaker.Unlock()
			return true, false

		case 'w':
			speaker.Lock()
			ap.gain.Gain += 1.0
			speaker.Unlock()
			return true, false

		case 'a':
			speaker.Lock()
			ap.volume.Volume -= 0.1
			speaker.Unlock()
			return true, false

		case 's':
			speaker.Lock()
			ap.volume.Volume += 0.1
			speaker.Unlock()
			return true, false

		case 'z':
			speaker.Lock()
			ap.resampler.SetRatio(ap.resampler.Ratio() * 15 / 16)
			speaker.Unlock()
			return true, false

		case 'x':
			speaker.Lock()
			ap.resampler.SetRatio(ap.resampler.Ratio() * 16 / 15)
			speaker.Unlock()
			return true, false
		}
	}
	return false, false
}

func main() {
	sr := beep.SampleRate(44000)
	streamer, err := speaker.DeviceCapture(sr, speaker.DefaultCaptureDevice(), sr.N(time.Second/10), 2)
	if err != nil {
		panic(err)
	}

	speaker.Init(sr, sr.N(time.Second/30))

	screen, err := tcell.NewScreen()
	if err != nil {
		report(err)
	}
	err = screen.Init()
	if err != nil {
		report(err)
	}
	defer screen.Fini()

	ap := newAudioPanel(sr, streamer)

	screen.Clear()
	ap.draw(screen)
	screen.Show()

	ap.play()

	seconds := time.Tick(time.Second)
	events := make(chan tcell.Event)
	go func() {
		for {
			events <- screen.PollEvent()
		}
	}()

loop:
	for {
		select {
		case event := <-events:
			changed, quit := ap.handle(event)
			if quit {
				break loop
			}
			if changed {
				screen.Clear()
				ap.draw(screen)
				screen.Show()
			}
		case <-seconds:
			screen.Clear()
			ap.draw(screen)
			screen.Show()
		}
	}
}

func report(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}