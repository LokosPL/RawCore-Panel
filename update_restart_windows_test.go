//go:build windows

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// This child runs the production download/apply handlers. The spawned installer and
// reopened panel are the actual Windows release EXE, not a replacement mock.
func TestUpdaterProcessChild(t *testing.T) {
	if os.Getenv("RAWCRAFT_SMOKE_CHILD") != "1" {
		t.Skip("subprocess only")
	}
	appExe = os.Getenv("RAWCRAFT_SMOKE_TARGET")
	payload, e := os.ReadFile(os.Getenv("RAWCRAFT_SMOKE_PAYLOAD"))
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(payload)
	sum := hex.EncodeToString(h[:])
	settings.Lock()
	settings.Token = "unit-test-token"
	settings.Prefs.AutoUpdate = true
	settings.Unlock()
	githubClient = &http.Client{Transport: mockTransport(func(r *http.Request) (*http.Response, error) {
		var body string
		switch r.URL.Path {
		case "/repos/" + updateRepository + "/releases/latest":
			body = fmt.Sprintf(`{"tag_name":"v99.0.0","assets":[{"name":"RAWCRAFT-Control.exe","url":"https://api.github.com/repos/%s/releases/assets/1","size":%d},{"name":"SHA256SUMS","url":"https://api.github.com/repos/%s/releases/assets/2"}]}`, updateRepository, len(payload), updateRepository)
		case "/repos/" + updateRepository + "/releases/assets/1":
			body = string(payload)
		case "/repos/" + updateRepository + "/releases/assets/2":
			body = sum + "  " + updateAsset
		default:
			return nil, fmt.Errorf("unexpected endpoint %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	checkForUpdate(true)
	if updateSnapshot().State != "ready" {
		t.Fatalf("download failed: %+v", updateSnapshot())
	}
	w := httptest.NewRecorder()
	handleApplyUpdate(w, httptest.NewRequest("POST", "/api/update/apply", strings.NewReader("{}")))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	time.Sleep(10 * time.Second)
	t.Fatal("old process did not exit after applying update")
}
func nativeVisibleWindow(match func(string, uint32) bool) uint32 {
	user := syscall.NewLazyDLL("user32.dll")
	kernel := syscall.NewLazyDLL("kernel32.dll")
	var found uint32
	callback := syscall.NewCallback(func(hwnd, unused uintptr) uintptr {
		visible, _, _ := user.NewProc("IsWindowVisible").Call(hwnd)
		if visible == 0 {
			return 1
		}
		var pid uint32
		user.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		var title [512]uint16
		user.NewProc("GetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(&title[0])), 512)
		if match(syscall.UTF16ToString(title[:]), pid) {
			found = pid
			return 0
		}
		_ = kernel
		return 1
	})
	user.NewProc("EnumWindows").Call(callback, 0)
	return found
}
func visiblePanelPID(target string) uint32 {
	return nativeVisibleWindow(func(title string, pid uint32) bool {
		if title != "RAWCRAFT Control" {
			return false
		}
		h, e := syscall.OpenProcess(0x1000, false, pid)
		if e != nil {
			return false
		}
		defer syscall.CloseHandle(h)
		var path [32768]uint16
		size := uint32(len(path))
		ok, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("QueryFullProcessImageNameW").Call(uintptr(h), 0, uintptr(unsafe.Pointer(&path[0])), uintptr(unsafe.Pointer(&size)))
		return ok != 0 && strings.EqualFold(filepath.Clean(syscall.UTF16ToString(path[:size])), filepath.Clean(target))
	})
}
func visibleInstallerPID() uint32 {
	return nativeVisibleWindow(func(title string, pid uint32) bool { return title == "RAWCRAFT — Aktualizacja panelu" })
}
func stopSmokeProcess(pid uint32) {
	if pid == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "taskkill", "/PID", strconv.Itoa(int(pid)), "/T", "/F").Run()
}
func TestFullUpdateRestartsVisiblePanel(t *testing.T) {
	dir := os.Getenv("RAWCRAFT_UPDATE_SMOKE_DIR")
	if dir == "" {
		t.Skip("release EXE integration test")
	}
	target := filepath.Join(dir, updateAsset)
	payload, e := filepath.Abs(filepath.Join("dist", updateAsset))
	if e != nil {
		t.Fatal(e)
	}
	oldBackup := target + ".old"
	oldSentinel := []byte("MZlocked legacy backup must survive")
	lockLegacyBackup(t, oldBackup, oldSentinel)
	lockedNext := target + ".new"
	nextSentinel := []byte("MZlocked legacy staging file")
	lockLegacyBackup(t, lockedNext, nextSentinel)
	testExe, _ := os.Executable()
	cmd := exec.Command(testExe, "-test.run=^TestUpdaterProcessChild$")
	cache := t.TempDir()
	profile := t.TempDir()
	cmd.Env = append(os.Environ(), "RAWCRAFT_SMOKE_CHILD=1", "RAWCRAFT_SMOKE_TARGET="+target, "RAWCRAFT_SMOKE_PAYLOAD="+payload, "LOCALAPPDATA="+cache, "APPDATA="+profile)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	ended := make(chan error, 1)
	go func() { ended <- cmd.Wait() }()
	var panel, installer uint32
	seenInstaller := false
	success := false
	defer func() {
		stopSmokeProcess(panel)
		stopSmokeProcess(installer)
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		time.Sleep(time.Second)
	}()
	deadline := time.Now().Add(70 * time.Second)
	for time.Now().Before(deadline) {
		if p := visibleInstallerPID(); p != 0 {
			installer = p
			seenInstaller = true
		}
		if p := visiblePanelPID(target); p != 0 {
			panel = p
		}
		if seenInstaller && panel != 0 && visibleInstallerPID() == 0 {
			success = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !success {
		t.Fatalf("installer=%v panel=%d; full GUI update did not complete", seenInstaller, panel)
	}
	select {
	case e := <-ended:
		if e != nil {
			t.Fatalf("parent failed: %v %s", e, out.String())
		}
	case <-time.After(time.Second):
		t.Fatal("old process remains alive")
	}
	actual, _ := os.ReadFile(target)
	wanted, _ := os.ReadFile(payload)
	if !bytes.Equal(actual, wanted) {
		t.Fatal("restarted wrong release")
	}
	if _, e := os.Stat(filepath.Join(cache, "RawcraftControl", "updates", "install-error.txt")); !os.IsNotExist(e) {
		t.Fatal("installer reported an error")
	}
	oldBytes, _ := os.ReadFile(oldBackup)
	nextBytes, _ := os.ReadFile(lockedNext)
	if !bytes.Equal(oldBytes, oldSentinel) || !bytes.Equal(nextBytes, nextSentinel) {
		t.Fatal("legacy locked files were altered")
	}
	t.Log("PASS: locked/read-only .old and .new did not block real EXE replacement")
	t.Log("PASS: verified download -> visible RAWCRAFT installer -> parent exits -> binary replaced -> visible panel responds -> installer closes")
}
func TestLegacyHiddenUpdaterReopensVisiblePanel(t *testing.T) {
	previous := os.Getenv("RAWCRAFT_PREVIOUS_EXE")
	if previous == "" {
		t.Skip("previous release integration test")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, updateAsset)
	helper := filepath.Join(dir, "old-updater.exe")
	staged := filepath.Join(dir, "new-panel.exe")
	for _, path := range []string{target, helper} {
		if e := copyFile(previous, path); e != nil {
			t.Fatal(e)
		}
	}
	payload, e := os.ReadFile(filepath.Join("dist", updateAsset))
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(staged, payload, 0600)
	h := sha256.Sum256(payload)
	cmd := exec.Command(helper, "--install-update", "2147483647", target, staged, hex.EncodeToString(h[:]))
	cmd.Env = append(os.Environ(), "LOCALAPPDATA="+filepath.Join(dir, "cache"), "APPDATA="+filepath.Join(dir, "profile"))
	hideWindow(cmd)
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	go cmd.Wait()
	var panel uint32
	defer func() { stopSmokeProcess(panel); _ = cmd.Process.Kill(); time.Sleep(time.Second) }()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		panel = visiblePanelPID(target)
		if panel != 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if panel == 0 {
		t.Fatal("old SW_HIDE updater did not reopen a visible new panel")
	}
	actual, _ := os.ReadFile(target)
	if !bytes.Equal(actual, payload) {
		t.Fatal("old helper did not replace application")
	}
	t.Log("PASS: real 1.4.9 helper with SW_HIDE installs " + appVersion + " and the new panel is visible")
}

func TestFailedUpdateKeepsOnlyInstaller(t *testing.T) {
	if os.Getenv("RAWCRAFT_UPDATE_SMOKE_DIR") == "" {
		t.Skip("release EXE integration test")
	}
	dir := t.TempDir()
	payload, e := os.ReadFile(filepath.Join("dist", updateAsset))
	if e != nil {
		t.Fatal(e)
	}
	helper := filepath.Join(dir, "installer.exe")
	staged := filepath.Join(dir, "update.exe")
	target := filepath.Join(dir, "missing-folder", updateAsset)
	for _, path := range []string{helper, staged} {
		if e = os.WriteFile(path, payload, 0700); e != nil {
			t.Fatal(e)
		}
	}
	sum := sha256.Sum256(payload)
	cmd := exec.Command(helper, "--install-update", "2147483647", target, staged, hex.EncodeToString(sum[:]))
	cmd.Env = append(os.Environ(), "LOCALAPPDATA="+filepath.Join(dir, "cache"), "APPDATA="+filepath.Join(dir, "profile"))
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	ended := make(chan error, 1)
	go func() { ended <- cmd.Wait() }()
	defer func() { stopSmokeProcess(uint32(cmd.Process.Pid)); _ = cmd.Process.Kill() }()
	deadline := time.Now().Add(50 * time.Second)
	failed := false
	for time.Now().Before(deadline) {
		if visiblePanelPID(target) != 0 {
			t.Fatal("failed update reopened panel")
		}
		if _, e = os.Stat(filepath.Join(dir, "install-error.txt")); e == nil {
			failed = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !failed {
		t.Fatal("failure was not recorded")
	}
	if visibleInstallerPID() != uint32(cmd.Process.Pid) {
		t.Fatal("small error window did not remain visible")
	}
	select {
	case <-ended:
		t.Fatal("error window closed automatically")
	default:
	}
	t.Log("PASS: replacement failure leaves only visible installer with diagnostic; panel not reopened")
}
