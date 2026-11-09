package main

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestInstanceMigrationAndConflicts(t *testing.T){
    p:=Preferences{Host:"example.test",Port:22,User:"mc",ServerPath:"/srv/main",MinecraftProcess:"main",WWWProcess:"www"}
    items:=normalizedInstances(p);if len(items)!=1||items[0].ID!="normalny"||items[0].ServerPath!="/srv/main"{t.Fatal(items)}
    other:=ServerInstance{ID:"testowy",Name:"Test",ServerPath:"/srv/test",MinecraftProcess:"test",Port:25566,RCONPort:25576}
    if e:=validateInstances(append(items,other));e!=nil{t.Fatal(e)}
    for _,bad:=range []ServerInstance{
        {ID:"testowy",Name:"Test",ServerPath:"/srv/main/sub",MinecraftProcess:"test"},
        {ID:"testowy",Name:"Test",ServerPath:"/srv/test",MinecraftProcess:"main"},
        {ID:"testowy",Name:"Test",ServerPath:"/srv/test",MinecraftProcess:"test",Port:25566,RCONPort:25566},
    }{if validateInstances(append(items,bad))==nil{t.Fatalf("accepted conflict %+v",bad)}}
}
func TestStaleInstanceAndCrossInstanceFilesNeverReachHandler(t *testing.T){
    settings.Lock();old:=settings.Prefs;settings.Prefs=Preferences{Host:"example.test",Port:22,User:"mc",Instances:[]ServerInstance{{ID:"normalny",Name:"Main",ServerPath:"/srv/main",MinecraftProcess:"main"},{ID:"testowy",Name:"Test",ServerPath:"/srv/test",MinecraftProcess:"test"}}};items:=normalizedInstances(settings.Prefs);settings.Unlock()
    defer func(){settings.Lock();settings.Prefs=old;settings.Unlock()}()
    called:=false;handler:=scopeInstance(func(w http.ResponseWriter,r *http.Request){called=true;got,e:=requestInstance(r);if e!=nil||got.ID!="testowy"{t.Fatal(got,e)}})
    for _,tc:=range []struct{revision,path string;want bool}{{items[1].Revision,"/srv/test/server.properties",true},{"stale","/srv/test/server.properties",false},{items[1].Revision,"/srv/main/server.properties",false},{items[1].Revision,"/srv/test/../main/server.properties",false}}{
        called=false;r:=httptest.NewRequest("POST","http://127.0.0.1/api/save",strings.NewReader(`{"path":"`+tc.path+`"}`));r.Header.Set("Content-Type","application/json");r.Header.Set("X-RAWCRAFT-Instance","testowy");r.Header.Set("X-RAWCRAFT-Revision",tc.revision);w:=httptest.NewRecorder();handler(w,r);if called!=tc.want{t.Fatalf("path=%s status=%d called=%v",tc.path,w.Code,called)}
    }
}
