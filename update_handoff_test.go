package main

import (
 "net/http"
 "strings"
 "testing"
)
func TestVisibleInstallerHandoffRequiresExactHashAndLoopbackCapability(t *testing.T){
 hash:=strings.Repeat("a",64)
 endpoint,wait,closeHandoff,e:=installerHandoff(hash);if e!=nil{t.Fatal(e)};defer closeHandoff()
 if e=confirmInstaller(endpoint,strings.Repeat("b",64));e==nil{t.Fatal("wrong payload accepted")}
 for _,address:=range []string{"https://evil.example/ready","http://localhost:1234/ready","http://127.0.0.1:1234/admin","http://127.0.0.1:1234/rawcraft-installer/../ready"}{if confirmInstaller(address,hash)==nil{t.Fatal("unsafe callback accepted",address)}}
 response,e:=http.Get(endpoint);if e!=nil{t.Fatal(e)};response.Body.Close();if response.StatusCode!=403{t.Fatal("GET accepted")}
 if e=confirmInstaller(endpoint,hash);e!=nil{t.Fatal(e)};if e=wait();e!=nil{t.Fatal(e)}
 if e=confirmInstaller(endpoint,hash);e!=nil{t.Fatal("idempotent ACK failed",e)}
}
