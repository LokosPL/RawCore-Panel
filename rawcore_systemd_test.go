package main
import ("testing";"time";"math";"strings")
func TestParseRawcoreSystemdStatus(t *testing.T) {
    s:=parseRawcoreSystemdStatus("LoadState=loaded\nActiveState=active\nSubState=running\nMainPID=102079\nMemoryCurrent=321098765\n")
    if !s.Installed||s.ActiveState!="active"||s.SubState!="running"||s.PID!=102079||s.MemoryBytes!=321098765 {t.Fatalf("%+v",s)}
    absent:=parseRawcoreSystemdStatus("LoadState=not-found\nActiveState=inactive\nMemoryCurrent=18446744073709551615\n")
    if absent.Installed||absent.MemoryBytes!=0 {t.Fatalf("%+v",absent)}
}

func TestRawcoreProcessCounters(t *testing.T) {
    raw:="42 (Java Main (Paper)) S "+strings.Repeat("0 ",10)+"1000 500 "+strings.Repeat("0 ",6)+"12345 0 0 0"
    ticks,start,err:=parseRawcoreProcStat(raw)
    if err!=nil||ticks!=1500||start!=12345{t.Fatalf("ticks=%d start=%d err=%v",ticks,start,err)}
    rawcoreCPU.Lock();rawcoreCPU.Key="";rawcoreCPU.At=time.Time{};rawcoreCPU.Ticks=0;rawcoreCPU.Unlock()
    now:=time.Now()
    if _,ok:=rawcoreCPUSample("fixture",100,now,100);ok{t.Fatal("first sample must not give a percentage")}
    cpu,ok:=rawcoreCPUSample("fixture",200,now.Add(2*time.Second),100)
    if !ok||math.Abs(cpu-50)>0.01{t.Fatalf("cpu=%v ok=%v",cpu,ok)}
    if _,ok:=rawcoreCPUSample("another-pid",240,now.Add(4*time.Second),100);ok{t.Fatal("PID reuse must reset sample")}
}
