package main

import (
	"fmt"
	"image/color"
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"pseudocam/adb"
	"pseudocam/scrcpy"
	"pseudocam/softcam"
)

const (
	port = 1234
	fps  = 25
)

//go:embed icon.png
var iconPNG []byte

func main() {
	a := app.New()
	w := a.NewWindow("PseudoCam")
	icon := fyne.NewStaticResource("icon.png", iconPNG)
	a.SetIcon(icon)
	w.SetIcon(icon)
	w.SetFixedSize(true)

	// vertical spacer
	gap := func(h float32) fyne.CanvasObject {
		r := canvas.NewRectangle(color.Transparent)
		r.SetMinSize(fyne.NewSize(1, h))
		return r
	}

	sc, scErr := scrcpy.FindScrcpy()

	// ---- state (only touched from the UI thread) ----
	var (
		devices  []adb.Device
		cameras  []scrcpy.Camera
		sizes    []scrcpy.CameraSize
		pipeline *Pipeline
		gen      int // invalidates stale async results
	)

	// ---- widgets ----
	deviceSelect := widget.NewSelect(nil, nil)
	deviceSelect.PlaceHolder = "Select device"
	refreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), nil)

	cameraSelect := widget.NewSelect(nil, nil)
	cameraSelect.PlaceHolder = "Select camera"
	cameraSelect.Disable()

	resSelect := widget.NewSelect(nil, nil)
	resSelect.PlaceHolder = "Select resolution"
	resSelect.Disable()

	startBtn := widget.NewButton("Start Camera", nil)
	startBtn.Importance = widget.HighImportance
	startBtn.Disable()

	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	status.Alignment = fyne.TextAlignCenter

	setStatus := func(s string) { status.SetText(s) }
	// Safe to call from any goroutine.
	setStatusAsync := func(s string) { fyne.Do(func() { setStatus(s) }) }

	updateStartBtn := func() {
		if pipeline == nil && deviceSelect.SelectedIndex() >= 0 &&
			cameraSelect.SelectedIndex() >= 0 && resSelect.SelectedIndex() >= 0 {
			startBtn.Enable()
		} else if pipeline == nil {
			startBtn.Disable()
		}
	}

	resetCameras := func() {
		cameras = nil
		cameraSelect.Options = nil
		cameraSelect.ClearSelected()
		cameraSelect.Disable()
	}
	resetSizes := func() {
		sizes = nil
		resSelect.Options = nil
		resSelect.ClearSelected()
		resSelect.Disable()
	}

	// ---- settings / about dialog ----
	showSettings := func() {
		driverStatus := widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		driverMsg := widget.NewLabel("")
		driverMsg.Wrapping = fyne.TextWrapWord
		driverMsg.Alignment = fyne.TextAlignCenter
		driverMsg.Hide()
		driverBtn := widget.NewButton("", nil)

		refreshDriver := func() {
			if softcam.IsInstalled() {
				driverStatus.SetText("Virtual camera driver: Installed")
				driverBtn.SetText("Uninstall Driver")
				driverBtn.Importance = widget.DangerImportance
			} else {
				driverStatus.SetText("Virtual camera driver: Not installed")
				driverBtn.SetText("Install Driver")
				driverBtn.Importance = widget.HighImportance
			}
			driverBtn.Refresh()
		}
		refreshDriver()

		driverBtn.OnTapped = func() {
			installed := softcam.IsInstalled()
			driverBtn.Disable()
			driverMsg.SetText("Waiting for administrator approval...")
			driverMsg.Show()

			go func() {
				var err error
				if installed {
					err = softcam.Uninstall()
				} else {
					err = softcam.Install()
				}
				fyne.Do(func() {
					driverBtn.Enable()
					refreshDriver()
					if err != nil {
						driverMsg.SetText(err.Error())
					} else {
						driverMsg.SetText("Done")
					}
				})
			}()
		}

		about := widget.NewLabelWithStyle("PseudoCam v1.0", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		credits := widget.NewLabel("Use your Android phone as a webcam.\nMade by github.com/adithya5434")
		credits.Alignment = fyne.TextAlignCenter
		credits.Wrapping = fyne.TextWrapWord
		powered := widget.NewLabel("Usage: Turn on USB debugging on your phone, connect it via USB, and allow the connection.")
		powered.Alignment = fyne.TextAlignCenter
		powered.Wrapping = fyne.TextWrapWord

		content := container.NewVBox(
			gap(0),
			driverStatus,
			gap(2),
			driverBtn,
			driverMsg,
			gap(0),
			widget.NewSeparator(),
			gap(0),
			about,
			credits,
			powered,
			gap(0),
		)
		d := dialog.NewCustom("Settings", "Close", content, w)
		d.Resize(fyne.NewSize(200, 0))
		d.Show()
	}

	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), showSettings)
	settingsBtn.Importance = widget.LowImportance

	// ---- device -> cameras ----
	loadCameras := func(dev adb.Device) {
		gen++
		myGen := gen
		setStatus("Loading cameras...")

		go func() {
			cams, err := sc.ListCameraSizes(dev.Serial)
			fyne.Do(func() {
				if myGen != gen {
					return
				}
				if err != nil {
					setStatus("Could not list cameras: " + err.Error())
					return
				}
				cameras = cams
				opts := make([]string, len(cams))
				for i, c := range cams {
					opts[i] = fmt.Sprintf("%d: %s (%dx%d)", c.ID, c.Name, c.MaxWidth, c.MaxHeight)
				}
				cameraSelect.Options = opts
				cameraSelect.Refresh()
				cameraSelect.Enable()
				setStatus(fmt.Sprintf("%d camera(s) found", len(cams)))
				if len(cams) == 1 {
					cameraSelect.SetSelectedIndex(0)
				}
			})
		}()
	}

	deviceSelect.OnChanged = func(string) {
		gen++ // cancel in-flight camera loads
		resetSizes()
		resetCameras()
		if i := deviceSelect.SelectedIndex(); i >= 0 {
			loadCameras(devices[i])
		}
		updateStartBtn()
	}

	// ---- camera -> resolutions ----
	cameraSelect.OnChanged = func(string) {
		resetSizes()
		if i := cameraSelect.SelectedIndex(); i >= 0 {
			sizes = cameras[i].Sizes
			opts := make([]string, len(sizes))
			for j, s := range sizes {
				opts[j] = fmt.Sprintf("%dx%d", s.Width, s.Height)
			}
			resSelect.Options = opts
			resSelect.Refresh()
			resSelect.Enable()
			if len(sizes) > 0 {
				resSelect.SetSelectedIndex(0)
			}
		}
		updateStartBtn()
	}

	resSelect.OnChanged = func(string) { updateStartBtn() }

	// ---- refresh devices ----
	refreshDevices := func() {
		gen++
		refreshBtn.Disable()
		setStatus("Searching for devices...")

		go func() {
			devs, err := adb.ListDevices()
			fyne.Do(func() {
				refreshBtn.Enable()
				devices = devs
				deviceSelect.Options = nil
				deviceSelect.ClearSelected()
				resetCameras()
				resetSizes()

				if err != nil {
					setStatus("adb error: " + err.Error())
					updateStartBtn()
					return
				}
				opts := make([]string, len(devs))
				for i, d := range devs {
					opts[i] = d.Serial
				}
				deviceSelect.Options = opts
				deviceSelect.Refresh()

				if len(devs) == 0 {
					setStatus("No Android devices found")
				} else if len(devs) == 1 {
					deviceSelect.SetSelectedIndex(0)
				} else {
					setStatus(fmt.Sprintf("%d devices found", len(devs)))
				}
				updateStartBtn()
			})
		}()
	}
	refreshBtn.OnTapped = refreshDevices

	// ---- running state ----
	setRunning := func(running bool) {
		if running {
			deviceSelect.Disable()
			cameraSelect.Disable()
			resSelect.Disable()
			refreshBtn.Disable()
			settingsBtn.Disable()
			startBtn.SetText("Stop Camera")
			startBtn.Importance = widget.DangerImportance
			startBtn.Enable()
		} else {
			deviceSelect.Enable()
			if len(cameras) > 0 {
				cameraSelect.Enable()
			}
			if len(sizes) > 0 {
				resSelect.Enable()
			}
			refreshBtn.Enable()
			settingsBtn.Enable()
			startBtn.SetText("Start Camera")
			startBtn.Importance = widget.HighImportance
		}
		startBtn.Refresh()
	}

	// ---- start / stop ----
	startBtn.OnTapped = func() {
		// Stop
		if pipeline != nil {
			p := pipeline
			startBtn.Disable()
			setStatus("Stopping...")
			go p.Stop()
			return
		}

		// Start
		di, ci, ri := deviceSelect.SelectedIndex(), cameraSelect.SelectedIndex(), resSelect.SelectedIndex()
		if di < 0 || ci < 0 || ri < 0 {
			return
		}
		dev, cam, size := devices[di], cameras[ci], sizes[ri]

		if size.Width%4 != 0 || size.Height%4 != 0 {
			setStatus(fmt.Sprintf("%dx%d is not supported (must be multiples of 4)", size.Width, size.Height))
			return
		}

		if !softcam.IsInstalled() {
			setStatus("Driver not installed. Open Settings (top right) to install it.")
			return
		}

		startBtn.Disable()
		deviceSelect.Disable()
		cameraSelect.Disable()
		resSelect.Disable()
		refreshBtn.Disable()
		setStatus("Starting...")

		go func() {
			p, err := StartPipeline(sc, dev, cam.ID, size, port, fps, setStatusAsync)
			if err != nil {
				fyne.Do(func() {
					setRunning(false)
					updateStartBtn()
					setStatus("Failed to start: " + err.Error())
				})
				return
			}

			fyne.DoAndWait(func() {
				pipeline = p
				setRunning(true)
			})

			<-p.Ended() // user pressed stop, device unplugged, etc.

			fyne.Do(func() {
				pipeline = nil
				setRunning(false)
				updateStartBtn()
				// keep a more specific message if one was set (e.g. stream error)
				if status.Text == "" || status.Text == "Stopping..." {
					setStatus("Stopped")
				}
			})
		}()
	}

	w.SetOnClosed(func() {
		if pipeline != nil {
			pipeline.Stop()
		}
	})

	// ---- layout ----
	title := canvas.NewText("PseudoCam", theme.ForegroundColor())
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}

	// label | control rows, with a gap between each row
	form := container.New(layout.NewFormLayout(),
		widget.NewLabel("Device"), container.NewBorder(nil, nil, nil, refreshBtn, deviceSelect),
		gap(10), gap(10),
		widget.NewLabel("Camera"), cameraSelect,
		gap(10), gap(10),
		widget.NewLabel("Resolution"), resSelect,
	)

	// reserve room for ~2 lines of status text so the window never needs extra height
	statusBox := container.NewStack(gap(120), status)

	content := container.NewPadded(container.NewVBox(
		container.NewBorder(nil, nil, nil, settingsBtn, title),
		gap(6),
		widget.NewSeparator(),
		gap(6),
		form,
		gap(10),
		widget.NewSeparator(),
		gap(8),
		startBtn,
		gap(6),
		statusBox,
	))

	w.SetContent(content)
	w.Resize(fyne.NewSize(260, content.MinSize().Height))

	if scErr != nil {
		setStatus(scErr.Error())
		refreshBtn.Disable()
	} else {
		a.Lifecycle().SetOnStarted(refreshDevices)
	}

	w.ShowAndRun()
}