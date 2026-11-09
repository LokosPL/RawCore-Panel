package terminalbuf
import("context";"time";"testing";"bytes";"encoding/base64")
func TestTailOffsets(t *testing.T){b:=new(Buffer);b.Write(bytes.Repeat([]byte("x"),Capacity+10));s:=b.Read(0);d,_:=base64.StdEncoding.DecodeString(s.Data);if !s.Lost||s.Start!=10||len(d)!=ChunkSize{t.Fatal(s.Start,len(d))};b.Write([]byte("\xe2\x82\xac"));s=b.Read(Capacity+10);d,_=base64.StdEncoding.DecodeString(s.Data);if string(d)!="€"{t.Fatal(d)};b.Close();if !b.Read(s.Next).Closed{t.Fatal("not closed")}}


func TestLongPollWakesOnOutputAndDoesNotConsumeData(t *testing.T){
 b:=new(Buffer);ctx,cancel:=context.WithTimeout(context.Background(),time.Second);defer cancel()
 result:=make(chan Snapshot,1);go func(){s,e:=b.Wait(ctx,0,time.Second);if e==nil{result<-s}}()
 b.Write([]byte("echo"))
 select{case s:=<-result:raw,_:=base64.StdEncoding.DecodeString(s.Data);if string(raw)!="echo"{t.Fatal(s)};case <-time.After(100*time.Millisecond):t.Fatal("echo waits for polling interval")}
 if b.Read(0).Data==""{t.Fatal("read consumed output")}
}
func TestLongPollCancellationCloseAndChunkedTail(t *testing.T){
 b:=new(Buffer);ctx,cancel:=context.WithCancel(context.Background());cancel();if _,e:=b.Wait(ctx,0,time.Second);e==nil{t.Fatal("ignored cancel")}
 b.Write(bytes.Repeat([]byte("x"),Capacity+10));b.Write([]byte("tail"));b.Close()
 var all []byte;offset:=int64(0)
 for i:=0;i<8;i++{s:=b.Read(offset);raw,_:=base64.StdEncoding.DecodeString(s.Data);if len(raw)>ChunkSize{t.Fatal("unbounded render batch")};all=append(all,raw...);offset=s.Next;if s.Closed{break}}
 if len(all)!=Capacity||!bytes.HasSuffix(all,[]byte("tail")){t.Fatal("ring lost retained output",len(all))}
 if _,e:=b.Write([]byte("late"));e==nil{t.Fatal("write after close")}
}
