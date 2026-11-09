package main
import "testing"
func TestParseRawcoreSystemdStatus(t *testing.T) {
    s:=parseRawcoreSystemdStatus("LoadState=loaded\nActiveState=active\nSubState=running\nMainPID=102079\nMemoryCurrent=321098765\n")
    if !s.Installed||s.ActiveState!="active"||s.SubState!="running"||s.PID!=102079||s.MemoryBytes!=321098765 {t.Fatalf("%+v",s)}
    absent:=parseRawcoreSystemdStatus("LoadState=not-found\nActiveState=inactive\nMemoryCurrent=18446744073709551615\n")
    if absent.Installed||absent.MemoryBytes!=0 {t.Fatalf("%+v",absent)}
}
