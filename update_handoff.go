package main

import (
 "context"
 "errors"
 "net"
 "net/http"
 "net/url"
 "strings"
 "sync"
 "time"
)

// The old process may exit only after a visible new installer owns the job.
// The unguessable loopback capability is never persisted or sent to GitHub.
func installerHandoff(hash string)(string,func()error,func(),error){
 listener,err:=net.Listen("tcp4","127.0.0.1:0");if err!=nil{return "",nil,nil,err}
 path:="/rawcraft-installer/"+token()+"/ready";ready:=make(chan struct{});var once sync.Once
 mux:=http.NewServeMux();mux.HandleFunc(path,func(w http.ResponseWriter,r *http.Request){
  if r.Host!=listener.Addr().String()||r.Method!="POST"||r.Header.Get("X-RAWCRAFT-SHA256")!=hash{http.Error(w,"Confirmation",403);return}
  once.Do(func(){close(ready)});w.WriteHeader(http.StatusOK)
 })
 server:=&http.Server{Handler:mux,ReadHeaderTimeout:3*time.Second,ReadTimeout:5*time.Second,WriteTimeout:5*time.Second}
 go server.Serve(listener)
 wait:=func()error{select{case <-ready:return nil;case <-time.After(30*time.Second):return errors.New("Aktualizator nie otworzył okna. Panel pozostaje uruchomiony; spróbuj ponownie lub użyj instalatora z GitHub.")}}
 return "http://"+listener.Addr().String()+path,wait,func(){_=server.Close()},nil
}
func confirmInstaller(endpoint,hash string)error{
 if endpoint==""{return nil} // compatible with the 1.4.2–1.4.4 six-argument callers
 u,e:=url.Parse(endpoint)
 if e!=nil||u.Scheme!="http"||u.User!=nil||u.RawQuery!=""||u.Fragment!=""||u.RawPath!=""{return errors.New("Niepoprawne potwierdzenie instalatora")}
 host,_,e:=net.SplitHostPort(u.Host);if e!=nil||host!="127.0.0.1"{return errors.New("Potwierdzenie wymaga loopback")}
 nonce:=strings.TrimSuffix(strings.TrimPrefix(u.Path,"/rawcraft-installer/"),"/ready")
 if len(nonce)!=43||u.Path!="/rawcraft-installer/"+nonce+"/ready"{return errors.New("Niepoprawna ścieżka instalatora")}
 for _,c:=range nonce{if !(c>='a'&&c<='z'||c>='A'&&c<='Z'||c>='0'&&c<='9'||c=='_'||c=='-'){return errors.New("Niepoprawny identyfikator instalatora")}}
 ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second);defer cancel()
 req,e:=http.NewRequestWithContext(ctx,"POST",endpoint,strings.NewReader("{}"));if e!=nil{return e};req.Header.Set("X-RAWCRAFT-SHA256",hash)
 client:=&http.Client{Timeout:3*time.Second,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}
 response,e:=client.Do(req);if e!=nil{return e};defer response.Body.Close();if response.StatusCode!=200{return errors.New("Panel nie potwierdził przekazania aktualizacji")};return nil
}
