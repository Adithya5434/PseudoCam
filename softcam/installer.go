package softcam

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

//go:embed softcam.dll
var embeddedDLL []byte

//go:embed softcam_installer.exe
var embeddedInstaller []byte

// CLSID softcam registers its DirectShow filter under.
// Verify with regedit (HKEY_CLASSES_ROOT\CLSID) if IsInstalled() ever looks wrong.
const softcamCLSID = `CLSID\{F4C0CFDA-B57B-4E65-9892-C3A955BDB920}\InprocServer32`

// dataDir is a permanent location: the registered DLL path is stored in the
// registry, so the DLL must NOT live in a temp directory.
func dataDir() (string, error) {
	base, err := os.UserCacheDir() // %LocalAppData% on Windows
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "PseudoCam", "softcam"), nil
}

// DLLPath is where softcam.dll is extracted to.
func DLLPath() string {
	dir, err := dataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "softcam.dll")
}

func writeIfDifferent(path string, data []byte, perm os.FileMode) error {
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil // already there (also avoids overwriting a DLL that's in use)
	}
	return os.WriteFile(path, data, perm)
}

func extract() (dllPath, installerPath string, err error) {
	dir, err := dataDir()
	if err != nil {
		return "", "", fmt.Errorf("locate data directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", fmt.Errorf("create %s: %w", dir, err)
	}

	dllPath = filepath.Join(dir, "softcam.dll")
	installerPath = filepath.Join(dir, "softcam_installer.exe")

	if err := writeIfDifferent(dllPath, embeddedDLL, 0644); err != nil {
		return "", "", fmt.Errorf("extract softcam.dll: %w", err)
	}
	if err := writeIfDifferent(installerPath, embeddedInstaller, 0755); err != nil {
		return "", "", fmt.Errorf("extract softcam_installer.exe: %w", err)
	}
	return dllPath, installerPath, nil
}

// IsInstalled reports whether the SoftCam DirectShow filter is registered.
func IsInstalled() bool {
	k, err := registry.OpenKey(registry.CLASSES_ROOT, softcamCLSID, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	k.Close()
	return true
}

// Install registers the driver. Shows a UAC prompt.
func Install() error {
	dll, installer, err := extract()
	if err != nil {
		return err
	}
	if err := runElevated(installer, fmt.Sprintf(`register "%s"`, dll)); err != nil {
		return fmt.Errorf("driver installation failed: %w", err)
	}
	if !IsInstalled() {
		return fmt.Errorf("driver installer finished but the driver is not registered")
	}
	return nil
}

// Uninstall unregisters the driver. Shows a UAC prompt.
func Uninstall() error {
	dll, installer, err := extract()
	if err != nil {
		return err
	}
	if err := runElevated(installer, fmt.Sprintf(`unregister "%s"`, dll)); err != nil {
		return fmt.Errorf("driver uninstallation failed: %w", err)
	}
	if IsInstalled() {
		return fmt.Errorf("uninstaller finished but the driver is still registered")
	}
	return nil
}

// ---- UAC elevation via ShellExecuteEx("runas") ----

type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         windows.Handle
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     windows.Handle
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    windows.Handle
	dwHotKey     uint32
	hIcon        windows.Handle
	hProcess     windows.Handle
}

const seeMaskNoCloseProcess = 0x00000040

var procShellExecuteExW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

// runElevated runs exe with a UAC prompt, waits for it, and checks its exit code.
func runElevated(exe, args string) error {
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	params, err := windows.UTF16PtrFromString(args)
	if err != nil {
		return err
	}

	info := shellExecuteInfo{
		fMask:        seeMaskNoCloseProcess,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: params,
		nShow:        windows.SW_HIDE,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))

	r, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		if callErr == windows.ERROR_CANCELLED {
			return fmt.Errorf("administrator permission was denied")
		}
		return fmt.Errorf("could not request elevation: %w", callErr)
	}
	defer windows.CloseHandle(info.hProcess)

	if _, err := windows.WaitForSingleObject(info.hProcess, windows.INFINITE); err != nil {
		return err
	}

	var code uint32
	if err := windows.GetExitCodeProcess(info.hProcess, &code); err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("installer exited with code %d", code)
	}
	return nil
}