package main

import (
    "context"
    "errors"
    "fmt"
    "math"
    "net/http"
    "strconv"
    "strings"
    "sync"
    "time"
)

const rawcoreUnit = "rawcore-survival.service"
const rawcoreDirectory = "/opt/rawcore-survival"

type rawcoreSystemdState struct {
    Installed bool `json:"installed"`
    ActiveState string `json:"activeState"`
    SubState string `json:"subState"`
    PID int64 `json:"pid"`
    MemoryBytes uint64 `json:"memoryBytes"`
    CPUPercent float64 `json:"cpuPercent"`
    CPUAvailable bool `json:"cpuAvailable"`
    DiskBytes uint64 `json:"diskBytes"`
    DiskAvailable bool `json:"diskAvailable"`
    UptimeSec uint64 `json:"uptimeSec"`
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

// Linux /proc/PID/stat uses whitespace fields after the final ')'
// surrounding the command name, which may itself contain spaces.
func parseRawcoreProcStat(raw string) (uint64,uint64,error) {
    end:=strings.LastIndex(raw,")")
    if end<0 {return 0,0,errors.New("Niepoprawne dane /proc")}
    fields:=strings.Fields(raw[end+1:])
    if len(fields)<20 {return 0,0,errors.New("Brak pól /proc")}
    user,err:=strconv.ParseUint(fields[11],10,64);if err!=nil{return 0,0,err}
    system,err:=strconv.ParseUint(fields[12],10,64);if err!=nil{return 0,0,err}
    started,err:=strconv.ParseUint(fields[19],10,64);if err!=nil{return 0,0,err}
    return user+system,started,nil
}

var rawcoreCPU = struct {
    sync.Mutex
    Key string
    Ticks uint64
    At time.Time
}{}

func rawcoreCPUSample(key string, ticks uint64, now time.Time, hz uint64) (float64,bool) {
    rawcoreCPU.Lock();defer rawcoreCPU.Unlock()
    lastTicks,lastAt,lastKey:=rawcoreCPU.Ticks,rawcoreCPU.At,rawcoreCPU.Key
    rawcoreCPU.Key,rawcoreCPU.Ticks,rawcoreCPU.At=key,ticks,now
    elapsed:=now.Sub(lastAt).Seconds()
    if key!=lastKey||hz==0||ticks<lastTicks||elapsed<0.5||elapsed>30{return 0,false}
    percent:=float64(ticks-lastTicks)/float64(hz)/elapsed*100
    if math.IsNaN(percent)||math.IsInf(percent,0)||percent<0{return 0,false}
    return percent,true
}

func enrichRawcoreCPU(ctx context.Context,c Credentials,s *rawcoreSystemdState) {
    if s.PID<=0||s.ActiveState!="active" {return}
    command:=fmt.Sprintf("cat /proc/%d/stat && printf '\\n__RAWCORE_UPTIME__\\n' && cat /proc/uptime && printf '\\n__RAWCORE_HZ__\\n' && getconf CLK_TCK",s.PID)
    out,err:=runRemote(ctx,c,command)
    if err!=nil{return}
    parts:=strings.Split(out,"\n__RAWCORE_UPTIME__\n")
    if len(parts)!=2{return}
    later:=strings.Split(parts[1],"\n__RAWCORE_HZ__\n")
    if len(later)!=2{return}
    ticks,start,err:=parseRawcoreProcStat(parts[0]);if err!=nil{return}
    hz,err:=strconv.ParseUint(strings.TrimSpace(later[1]),10,64);if err!=nil||hz==0{return}
    uptime,err:=strconv.ParseFloat(strings.Fields(later[0])[0],64)
    if err==nil&&uptime>=float64(start)/float64(hz) {
        s.UptimeSec=uint64(uptime-float64(start)/float64(hz))
    }
    key:=fmt.Sprintf("%s:%d:%s:%d:%d",c.Host,c.Port,c.User,s.PID,start)
    s.CPUPercent,s.CPUAvailable=rawcoreCPUSample(key,ticks,time.Now(),hz)
}

var rawcoreDiskCache = struct {
    sync.Mutex
    Key string
    At time.Time
    Bytes uint64
    Available bool
}{}

func rawcoreDiskUsage(ctx context.Context,c Credentials) (uint64,bool) {
    key:=fmt.Sprintf("%s:%d:%s",c.Host,c.Port,c.User)
    rawcoreDiskCache.Lock()
    if rawcoreDiskCache.Key==key && time.Since(rawcoreDiskCache.At)<time.Minute {
        size,ok:=rawcoreDiskCache.Bytes,rawcoreDiskCache.Available
        rawcoreDiskCache.Unlock()
        return size,ok
    }
    // Reserve this snapshot for a minute even when a directory scan fails.
    rawcoreDiskCache.Key=key
    rawcoreDiskCache.At=time.Now()
    rawcoreDiskCache.Bytes=0
    rawcoreDiskCache.Available=false
    rawcoreDiskCache.Unlock()

    diskContext,cancel:=context.WithTimeout(ctx,10*time.Second)
    defer cancel()
    out,err:=runRemote(diskContext,c,"LC_ALL=C du -s -B1 -- "+rawcoreDirectory+" 2>/dev/null")
    if err!=nil{return 0,false}
    fields:=strings.Fields(out)
    if len(fields)==0{return 0,false}
    value,err:=strconv.ParseUint(fields[0],10,64)
    if err!=nil{return 0,false}
    rawcoreDiskCache.Lock()
    if rawcoreDiskCache.Key==key {
        rawcoreDiskCache.Bytes=value
        rawcoreDiskCache.Available=true
    }
    rawcoreDiskCache.Unlock()
    return value,true
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
    s:=parseRawcoreSystemdStatus(out)
    if s.Installed {
        enrichRawcoreCPU(ctx,c,&s)
        s.DiskBytes,s.DiskAvailable=rawcoreDiskUsage(ctx,c)
    }
    ok(w,s)
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
