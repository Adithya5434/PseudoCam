package main

import (
	"fmt"
	"sync"
	"time"

	"pseudocam/adb"
	"pseudocam/ffmpeg"
	"pseudocam/frame"
	"pseudocam/proc"
	"pseudocam/scrcpy"
	"pseudocam/softcam"
)

// Pipeline owns scrcpy server -> ffmpeg decoder -> softcam output.
type Pipeline struct {
	device  adb.Device
	port    int
	server  *scrcpy.ServerProcess
	decoder *ffmpeg.Decoder
	cam     *softcam.Camera

	stop  chan struct{}
	ended chan struct{}
	once  sync.Once
	wg    sync.WaitGroup
}

// StartPipeline blocks until the scrcpy server is up (call it off the UI thread).
// status is called from background goroutines.
func StartPipeline(sc *scrcpy.Scrcpy, device adb.Device, cameraID int, size scrcpy.CameraSize,
	port int, fps float32, status func(string)) (*Pipeline, error) {

	p := &Pipeline{
		device: device,
		port:   port,
		stop:   make(chan struct{}),
		ended:  make(chan struct{}),
	}

	status("Starting scrcpy server...")
	server, err := sc.StartServer(device, cameraID, &size, port)
	if err != nil {
		p.removeForward()
		return nil, err
	}
	p.server = server

	status("Starting FFmpeg...")
	decoder, err := ffmpeg.Start(port, size.Width, size.Height)
	if err != nil {
		server.Close()
		p.removeForward()
		return nil, err
	}
	p.decoder = decoder

	cam, err := softcam.New(size.Width, size.Height, fps)
	if err != nil {
		decoder.Close()
		server.Close()
		p.removeForward()
		return nil, err
	}
	p.cam = cam

	latest := frame.NewLatest()

	// Decoder goroutine: must start immediately so FFmpeg's pipe never fills up.
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			f := frame.New(size.Width, size.Height)
			if err := decoder.ReadFrame(f); err != nil {
				select {
				case <-p.stop: // normal shutdown
				default:
					status("Stream ended: " + err.Error())
					go p.Stop()
				}
				return
			}
			latest.Publish(f)
		}
	}()

	// Output goroutine: wait for an app to connect, then send at a fixed FPS.
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		status("Waiting for camera application...")
		for !cam.WaitForConnection(1) {
			select {
			case <-p.stop:
				return
			default:
			}
		}
		status(fmt.Sprintf("Streaming %dx%d @ %.0f FPS", size.Width, size.Height, fps))

		ticker := time.NewTicker(time.Second / time.Duration(fps))
		defer ticker.Stop()

		var lastVersion uint64
		var bgr []byte

		for {
			select {
			case <-p.stop:
				return
			case <-ticker.C:
			}

			f := latest.GetFrame()
			if f == nil {
				continue
			}

			// Only convert when there's a new frame.
			if v := latest.Version(); v != lastVersion || bgr == nil {
				bgr = f.BGR24()
				lastVersion = v
			}

			if err := cam.SendFrame(bgr); err != nil {
				status("SoftCam error: " + err.Error())
				go p.Stop()
				return
			}
		}
	}()

	return p, nil
}

// Ended is closed once the pipeline has fully shut down.
func (p *Pipeline) Ended() <-chan struct{} { return p.ended }

// Stop is safe to call multiple times and from any goroutine. It blocks until cleanup is done.
func (p *Pipeline) Stop() {
	p.once.Do(func() {
		close(p.stop)
		p.decoder.Close()
		p.server.Close()
		p.removeForward()
		p.wg.Wait()
		p.cam.Close() // only after the output goroutine has exited
		close(p.ended)
	})
	<-p.ended
}

func (p *Pipeline) removeForward() {
	_ = proc.Command("adb", "-s", p.device.Serial, "forward", "--remove", fmt.Sprintf("tcp:%d", p.port)).Run()
}