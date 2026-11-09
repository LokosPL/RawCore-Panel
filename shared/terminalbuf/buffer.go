package terminalbuf

import (
 "context"
 "encoding/base64"
 "io"
 "sync"
 "time"
)
// Ring offsets count bytes. UTF-8 and ANSI fragments remain untouched for xterm.
type Buffer struct {mu sync.Mutex;data []byte;head,size int;end int64;closed bool;changed chan struct{}}
type Snapshot struct {Data string `json:"data"`;Start int64 `json:"start"`;Next int64 `json:"next"`;Lost bool `json:"lost"`;Closed bool `json:"closed"`}
const Capacity=256*1024
const ChunkSize=64*1024
func(b *Buffer)signal(){if b.changed!=nil{close(b.changed)};b.changed=make(chan struct{})}
func(b *Buffer)Write(p []byte)(int,error){
 b.mu.Lock();defer b.mu.Unlock();if b.closed{return 0,io.ErrClosedPipe};n:=len(p);if n==0{return 0,nil}
 if b.data==nil{b.data=make([]byte,Capacity)}
 b.end+=int64(n)
 if n>=Capacity{copy(b.data,p[n-Capacity:]);b.head=0;b.size=Capacity}else{
  drop:=b.size+n-Capacity;if drop>0{b.head=(b.head+drop)%Capacity;b.size-=drop}
  tail:=(b.head+b.size)%Capacity;first:=copy(b.data[tail:],p);copy(b.data,p[first:]);b.size+=n
 }
 b.signal();return n,nil
}
func(b *Buffer)Close(){b.mu.Lock();defer b.mu.Unlock();if !b.closed{b.closed=true;b.signal()}}
func(b *Buffer)snapshot(offset int64)Snapshot{
 start:=b.end-int64(b.size);lost:=offset<start||offset>b.end;if lost{offset=start}
 count:=int(b.end-offset);if count>ChunkSize{count=ChunkSize};out:=make([]byte,count)
 if count>0{at:=(b.head+int(offset-start))%Capacity;first:=copy(out,b.data[at:]);copy(out[first:],b.data[:count-first])}
 next:=offset+int64(count)
 return Snapshot{Data:base64.StdEncoding.EncodeToString(out),Start:start,Next:next,Lost:lost,Closed:b.closed&&next==b.end}
}
func(b *Buffer)Read(offset int64)Snapshot{b.mu.Lock();defer b.mu.Unlock();return b.snapshot(offset)}
// Wait releases the buffer lock and wakes immediately on SSH output or close.
func(b *Buffer)Wait(ctx context.Context,offset int64,timeout time.Duration)(Snapshot,error){
 b.mu.Lock();s:=b.snapshot(offset)
 if s.Next!=offset||s.Lost||b.closed{b.mu.Unlock();return s,nil}
 if b.changed==nil{b.changed=make(chan struct{})};changed:=b.changed;b.mu.Unlock()
 timer:=time.NewTimer(timeout);defer timer.Stop()
 select{case <-ctx.Done():return Snapshot{},ctx.Err();case <-changed:case <-timer.C:}
 return b.Read(offset),nil
}
