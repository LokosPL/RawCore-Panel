//go:build windows

package main

import (
	webview2 "github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/webviewloader"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// WebView2Loader is embedded by the dependency; no DLL or installer accompanies the EXE.
func runNativeApp(url string) bool { return runNativeWindow(url, nil) }
func runNativeWindow(url string, ready func(uintptr)) bool {
	return runSizedNativeWindow(url,"RAWCRAFT Control",0,0,"WebView2",ready)
}
func runUpdateWindow(url string, ready func(uintptr)) bool {
 return runSizedNativeWindow(url,"RAWCRAFT — Aktualizacja panelu",640,550,"UpdaterWebView2",ready)
}
func runSizedNativeWindow(url,title string,width,height int,profile string,ready func(uintptr)) bool {
 runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if _, e := webviewloader.GetInstalledVersion(); e != nil {
		return false
	}
	dir, e := os.UserCacheDir()
	if e != nil {
		return false
	}
	if width==0 {width,height=nativeInitialSize()}
 w := webview2.NewWithOptions(webview2.WebViewOptions{AutoFocus: true, DataPath: filepath.Join(dir, "RawcraftControl", profile), WindowOptions: webview2.WindowOptions{Title: title, Width: uint(width), Height: uint(height), Center: true}})
	if w == nil {
		return false
	}
	defer w.Destroy()
	// Use the first icon embedded in the executable for the native window and taskbar.
	module, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	icon, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("ExtractIconW").Call(module, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(appExe))), 0)
	if icon > 1 {
		send := syscall.NewLazyDLL("user32.dll").NewProc("SendMessageW")
		send.Call(uintptr(w.Window()), 0x80, 0, icon)
		send.Call(uintptr(w.Window()), 0x80, 1, icon)
	}
	nativeWindow.Lock()
	nativeWindow.handle = uintptr(w.Window())
	nativeWindow.Unlock()
	defer func() { nativeWindow.Lock(); nativeWindow.handle = 0; nativeWindow.Unlock() }()
	updateNativeTheme(currentTheme())
	if profile=="UpdaterWebView2" {w.SetSize(540,480,webview2.HintMin)} else {w.SetSize(800,650,webview2.HintMin)}
 // Older updaters launched us with SW_HIDE. Explicitly show again after creation.
 show:=syscall.NewLazyDLL("user32.dll").NewProc("ShowWindow")
 show.Call(uintptr(w.Window()),5)
 show.Call(uintptr(w.Window()),9)
 syscall.NewLazyDLL("user32.dll").NewProc("SetForegroundWindow").Call(uintptr(w.Window()))
	w.Navigate(url)
	if ready != nil {
		ready(uintptr(w.Window()))
	}
	w.Run()
	state.Lock()
	state.creds = Credentials{}
	state.connected = false
	state.admin = false
	state.root = false
	state.sudoPassword = ""
	state.Unlock()
	return true
}

var nativeWindow struct {
	sync.Mutex
	handle uintptr
}

func updateNativeTheme(theme string) {
	dark := theme == "dark"
	if theme == "system" {
		var key syscall.Handle
		path, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
		if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, path, 0, syscall.KEY_QUERY_VALUE, &key) == nil {
			var value uint32
			length := uint32(4)
			name, _ := syscall.UTF16PtrFromString("AppsUseLightTheme")
			if syscall.RegQueryValueEx(key, name, nil, nil, (*byte)(unsafe.Pointer(&value)), &length) == nil {
				dark = value == 0
			}
			syscall.RegCloseKey(key)
		}
	}
	nativeWindow.Lock()
	handle := nativeWindow.handle
	nativeWindow.Unlock()
	if handle == 0 {
		return
	}
	value := uint32(0)
	if dark {
		value = 1
	}
	proc := syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	proc.Call(handle, 20, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
}

// Choose a useful initial size on Full HD and 4K, within the Windows work area.
func nativeInitialSize()(int,int){
 var area struct{Left,Top,Right,Bottom int32}
 ok,_,_:=syscall.NewLazyDLL("user32.dll").NewProc("SystemParametersInfoW").Call(0x30,0,uintptr(unsafe.Pointer(&area)),0)
 if ok==0{return 1440,950}
 availableW,availableH:=int(area.Right-area.Left),int(area.Bottom-area.Top)
 width,height:=availableW*85/100,availableH*88/100
 if width>2560{width=2560};if height>1600{height=1600}
 if width<800{width=800};if height<650{height=650}
 if width>availableW{width=availableW};if height>availableH{height=availableH}
 return width,height
}


func closeUpdateWindow(){
 nativeWindow.Lock();handle:=nativeWindow.handle;nativeWindow.Unlock()
 if handle!=0 {syscall.NewLazyDLL("user32.dll").NewProc("PostMessageW").Call(handle,0x10,0,0)}
}
