//go:build windows

package main

import (
	"os/exec"
 "fmt"
	"syscall"
	"unsafe"
 "path/filepath"
 "strings"
 "golang.org/x/sys/windows/registry"
)

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

type dataBlob struct {
	Size uint32
	Data *byte
}

func cryptToken(data []byte, decrypt bool) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	in := dataBlob{uint32(len(data)), &data[0]}
	var out dataBlob
	name := "CryptProtectData"
	if decrypt {
		name = "CryptUnprotectData"
	}
	proc := syscall.NewLazyDLL("crypt32.dll").NewProc(name)
	result, _, err := proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	if result == 0 {
		return nil, err
	}
	defer syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree").Call(uintptr(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}
func waitParent(pid int) error {
	handle, err := syscall.OpenProcess(0x00100000, false, uint32(pid))
	if err != nil {
  if err==syscall.Errno(87){return nil};return err
 }
 defer syscall.CloseHandle(handle)
 result,err:=syscall.WaitForSingleObject(handle,60000)
 if err!=nil{return err};if result!=0{return fmt.Errorf("Poprzedni panel nie zakończył pracy")};return nil
}

// Only an EXE running from the registered per-user install may update its metadata.
func syncInstalledVersion(executable,version string) {
 key,e:=registry.OpenKey(registry.CURRENT_USER,`Software\Microsoft\Windows\CurrentVersion\Uninstall\RAWCRAFT-Control_is1`,registry.QUERY_VALUE|registry.SET_VALUE)
 if e!=nil{return};defer key.Close()
 location,_,e:=key.GetStringValue("InstallLocation");if e!=nil{return}
 actual,e:=filepath.Abs(filepath.Dir(executable));if e!=nil{return}
 installed,e:=filepath.Abs(location);if e!=nil||!strings.EqualFold(filepath.Clean(actual),filepath.Clean(installed)){return}
 _=key.SetStringValue("DisplayVersion",version)
}

