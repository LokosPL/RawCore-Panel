package main

import (
	"errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"rawcraft-control/shared/terminalbuf"
	"strconv"
	"sync"
	"time"
)

type desktopShell struct {
	mu       sync.Mutex
	writeMu  sync.Mutex
	client   *ssh.Client
	session  *ssh.Session
	input    io.WriteCloser
	output   *terminalbuf.Buffer
	identity string
	id       string
	instance string
}

var shell desktopShell

func shellIdentity(c Credentials) string {
	return c.User + "@" + net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}
func closeShell() { shell.mu.Lock(); defer shell.mu.Unlock(); closeShellLocked() }
func closeShellLocked() {
	if shell.client != nil {
		shell.client.Close()
	}
	if shell.session != nil {
		shell.session.Close()
	}
	if shell.output != nil {
		shell.output.Close()
	}
	shell.session = nil
	shell.client = nil
	shell.input = nil
	shell.id = ""
}
func openDesktopShell(c Credentials) error {
	home, e := os.UserHomeDir()
	if e != nil {
		return e
	}
	check, e := knownhosts.New(filepath.Join(home, ".ssh", "known_hosts"))
	if e != nil {
		return errors.New("Najpierw połącz VPS w panelu, aby zweryfikować klucz SSH")
	}
	address := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	socket, e := net.DialTimeout("tcp", address, 20*time.Second)
	if e != nil {
		return e
	}
	socket.SetDeadline(time.Now().Add(20 * time.Second))
	cfg := &ssh.ClientConfig{User: c.User, HostKeyCallback: check, Auth: []ssh.AuthMethod{ssh.Password(c.Password), ssh.KeyboardInteractive(func(_, _ string, q []string, _ []bool) ([]string, error) {
		a := make([]string, len(q))
		for i := range a {
			a[i] = c.Password
		}
		return a, nil
	})}}
	conn, ch, req, e := ssh.NewClientConn(socket, address, cfg)
	if e != nil {
		socket.Close()
		return errors.New("Terminal: nie udało się zweryfikować klucza SSH lub zalogować")
	}
	socket.SetDeadline(time.Time{})
	client := ssh.NewClient(conn, ch, req)
	session, e := client.NewSession()
	if e != nil {
		client.Close()
		return e
	}
	if e = session.RequestPty("xterm-256color", 30, 100, ssh.TerminalModes{ssh.ECHO: 1}); e != nil {
		session.Close()
		client.Close()
		return e
	}
	input, e := session.StdinPipe()
	if e != nil {
		session.Close()
		client.Close()
		return e
	}
	output := new(terminalbuf.Buffer)
	session.Stdout = output
	session.Stderr = output
	if e = session.Shell(); e != nil {
		session.Close()
		client.Close()
		return e
	}
	shell.client = client
	shell.session = session
	shell.input = input
	shell.output = output
	shell.identity = shellIdentity(c)
	shell.id = token()
	go func() { _ = session.Wait(); output.Close(); client.Close() }()
	return nil
}
func handleShell(w http.ResponseWriter, r *http.Request) {
	c, e := currentCred()
	if e != nil {
		bad(w, e)
		return
	}
	var req struct {
		Action string `json:"action"`
		Data   string `json:"data"`
		Offset int64  `json:"offset"`
		ID     string `json:"id"`
		Cols   int    `json:"cols"`
		Rows   int    `json:"rows"`
		WaitMs int    `json:"waitMs"`
		Script string `json:"script"`
	}
	if e = fromJSON(r, &req); e != nil {
		bad(w, e)
		return
	}
	scope, scopeErr := requestInstance(r)
	if scopeErr != nil {
		bad(w, scopeErr)
		return
	}
	shell.mu.Lock()
	if req.Action != "open" && shell.instance != scope.ID+":"+scope.Revision {
		shell.mu.Unlock()
		bad(w, errors.New("Terminal należy do innej instancji"))
		return
	}
	if shell.identity != shellIdentity(c) {
		closeShellLocked()
	}
	if req.Action == "open" {
		defer shell.mu.Unlock()
		closeShellLocked()
		if e = openDesktopShell(c); e != nil {
			bad(w, e)
			return
		}
		instance, err := requestInstance(r)
		if err != nil {
			closeShellLocked()
			bad(w, err)
			return
		}
		if _, err = io.WriteString(shell.input, "cd -- "+quote(instance.ServerPath)+"\r"); err != nil {
			closeShellLocked()
			bad(w, err)
			return
		}
		shell.instance = scope.ID + ":" + scope.Revision
		ok(w, map[string]string{"id": shell.id})
		return
	}
	if req.ID == "" || req.ID != shell.id || shell.session == nil {
		shell.mu.Unlock()
		bad(w, errors.New("Terminal został zamknięty. Otwórz nową sesję."))
		return
	}
	output, input, session, client := shell.output, shell.input, shell.session, shell.client
	if req.Action == "close" {
		closeShellLocked()
		shell.mu.Unlock()
		ok(w, map[string]bool{"ok": true})
		return
	}
	shell.mu.Unlock()
	switch req.Action {
	case "poll":
		if req.WaitMs < 0 || req.WaitMs > 1000 {
			bad(w, errors.New("Niepoprawny czas odczytu"))
			return
		}
		if req.WaitMs == 0 {
			ok(w, output.Read(req.Offset))
			return
		}
		var data terminalbuf.Snapshot
		data, e = output.Wait(r.Context(), req.Offset, time.Duration(req.WaitMs)*time.Millisecond)
		if e != nil {
			return
		}
		ok(w, data)
	case "write":
		if len(req.Data) > 32768 {
			bad(w, errors.New("Za dużo danych terminala"))
			return
		}
		shell.writeMu.Lock()
		_, e = io.WriteString(input, req.Data)
		shell.writeMu.Unlock()
		if e != nil {
			bad(w, e)
			return
		}
		ok(w, map[string]bool{"ok": true})
	case "batch":
		result, runErr := runShellBatch(r.Context(), client, scope.ServerPath, req.Script)
		if runErr != nil {
			bad(w, runErr)
			return
		}
		ok(w, result)
	case "resize":
		if req.Cols < 20 || req.Cols > 400 || req.Rows < 5 || req.Rows > 200 {
			bad(w, errors.New("Niepoprawny rozmiar terminala"))
			return
		}
		if e = session.WindowChange(req.Rows, req.Cols); e != nil {
			bad(w, e)
			return
		}
		ok(w, map[string]bool{"ok": true})
	default:
		bad(w, errors.New("Niepoprawna operacja terminala"))
	}
}
