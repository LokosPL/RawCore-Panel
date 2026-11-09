package main

import (
    "context"
    "errors"
    "net/http"
    "strconv"
    "strings"
    "time"
)

const rawcoreUnit = "rawcore-survival.service"

type rawcoreSystemdState struct {
    Installed bool `json:"installed"`
    ActiveState string `json:"activeState"`
    SubState string `json:"subState"`
    PID int64 `json:"pid"`
    MemoryBytes uint64 `json:"memoryBytes"`
}

func parseRawcoreSystemdStatus(raw string) rawcoreSystemdState {
    values:=map[string]string{}
    for _,line:=range strings.Split(raw,"\n") {
        parts:=strings.SplitN(strings.TrimSpace(line),"=",2)
        if len(parts)==2 {values[parts[0]]=parts[1]}
    }
    s:=rawcoreSystemdState{
        Installed: values["LoadState"]=="loaded" || values["LoadState"]=="masked",
        ActiveState:values["ActiveState"],
        SubState:values["SubState"],
    }
    if pid,err:=strconv.ParseInt(values["MainPID"],10,64);err==nil && pid>=0{s.PID=pid}
    if memory,err:=strconv.ParseUint(values["MemoryCurrent"],10,64);err==nil && memory<(1<<48){s.MemoryBytes=memory}
    return s
}

func handleRawcoreService(w http.ResponseWriter,r *http.Request) {
    if r.Method!="GET" && r.Method!="POST" {http.Error(w,"Method",405);return}
    c,e:=currentCred();if e!=nil {bad(w,e);return}
    ctx,cancel:=context.WithTimeout(r.Context(),35*time.Second);defer cancel()
    if r.Method=="POST" {
        var req struct{Action string `json:"action"`}
        if e=fromJSON(r,&req);e!=nil{bad(w,e);return}
        switch req.Action {case "start","stop","restart":default:bad(w,errors.New("Nieobsługiwana operacja systemd"));return}
        c,e=currentPrivilegedCred();if e!=nil{bad(w,e);return}
        out,err:=runRemote(ctx,c,"systemctl --no-ask-password "+req.Action+" "+rawcoreUnit)
        if err!=nil{bad(w,err);return}
        ok(w,map[string]string{"message":strings.TrimSpace(out)})
        return
    }
    out,err:=runRemote(ctx,c,"LC_ALL=C systemctl show --no-pager -p LoadState -p ActiveState -p SubState -p MainPID -p MemoryCurrent "+rawcoreUnit)
    if err!=nil{bad(w,err);return}
    ok(w,parseRawcoreSystemdStatus(out))
}

func handleRawcoreLogs(w http.ResponseWriter,r *http.Request) {
    if r.Method!="GET"{http.Error(w,"Method",405);return}
    c,e:=currentPrivilegedCred();if e!=nil{bad(w,e);return}
    ctx,cancel:=context.WithTimeout(r.Context(),18*time.Second);defer cancel()
    out,err:=runRemote(ctx,c,"journalctl -u "+rawcoreUnit+" --no-pager --output=short-iso -n 90")
    if err!=nil{bad(w,err);return}
    if len(out)>100000 {out=out[len(out)-100000:]}
    ok(w,map[string]string{"logs":out})
}
