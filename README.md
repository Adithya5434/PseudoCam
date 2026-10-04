# PseudoCam

Use your Android phone's camera as a webcam on Windows using ADB.

PseudoCam streams the phone camera over USB or Wi-Fi using [scrcpy](https://github.com/Genymobile/scrcpy), decodes it with [FFmpeg](https://ffmpeg.org/), and feeds the frames into a virtual camera ([SoftCam](https://github.com/tshino/softcam)). Apps like OBS and Discord then see it as a normal webcam.

```
Android camera -> scrcpy server (adb) -> FFmpeg (H.264 -> YUV420) -> BGR24 -> SoftCam virtual camera
```

## Requirements

- Windows 10/11 (64-bit)
- Android 12 or newer with USB debugging enabled (required by scrcpy camera mirroring)
- [scrcpy](https://github.com/Genymobile/scrcpy/releases) 2.2 or newer, with `scrcpy.exe` and `scrcpy-server` in the same folder, and that folder in `PATH`
- `adb` in `PATH` (included with scrcpy releases)
- `ffmpeg` in `PATH`

## Build

To build from source you need:

- [Go](https://go.dev/dl/) 1.21+
- A C compiler such as [MinGW-w64](https://www.mingw-w64.org/) (required by Fyne)

`softcam/softcam.dll` and `softcam/softcam_installer.exe` (64-bit, compiled from the [SoftCam source](https://github.com/tshino/softcam)) are already included in this repo and are embedded into the executable at build time.

```bash
git clone https://github.com/adithya5434/PseudoCam.git
cd PseudoCam
go mod tidy
```

**Normal build:**

```bash
go build .
```

**Smaller build:**

```bash
go build -trimpath -ldflags "-s -w -H=windowsgui" -o PseudoCam.exe .
```

What the flags do:

- `-s -w` strips the symbol table and debug info (smaller file)
- `-trimpath` removes local file paths from the binary
- `-H=windowsgui` hides the console window (use the normal build while debugging to see the `[scrcpy]` and `[FFmpeg]` logs)


## Usage

1. On your phone, enable **Developer options -> USB debugging**, connect it by USB and allow the connection.
2. Start `PseudoCam.exe`.
3. Open **Settings** (gear icon, top right) and click **Install Driver**. Accept the Windows administrator prompt. This only needs to be done once.
4. Choose your **Device**, **Camera** and **Resolution**. Use the refresh button if your phone is not listed.
5. Click **Start Camera**.
6. In your video app, select **PseudoCam** as the camera.

Click **Stop Camera** when you are done.

### Wireless

PseudoCam uses ADB, so wireless works naturally: any device that `adb devices` lists also shows up in the **Device** dropdown, whether it is connected by USB or Wi-Fi. Connect your phone over Wi-Fi first, then press the refresh button in PseudoCam.

On Android 11 or newer (phone and PC on the same network):

1. Enable **Developer options -> Wireless debugging**.
2. Tap **Pair device with pairing code** and run, using the IP, port and code shown on the phone:
   ```bash
   adb pair <ip>:<pairing-port>
   ```
3. Connect using the IP and port shown on the main Wireless debugging screen:
   ```bash
   adb connect <ip>:<port>
   ```

On older versions, connect the phone by USB once and run `adb tcpip 5555`, unplug it, then run `adb connect <phone-ip>:5555`.

Wireless adds some delay and can drop frames on a weak connection. A lower resolution usually helps.

## Credits

- [scrcpy](https://github.com/Genymobile/scrcpy) by Genymobile (Apache 2.0)
- [SoftCam](https://github.com/tshino/softcam) by tshino (MIT). The `softcam/` folder contains binaries built from its source.
- [FFmpeg](https://ffmpeg.org/)
- [Fyne](https://fyne.io/)

Made by [adithya5434](https://github.com/adithya5434).

## License

[MIT](LICENSE). Third-party notices: [softcam/LICENSE](softcam/LICENSE).