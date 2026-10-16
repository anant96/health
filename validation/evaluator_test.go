package validation

import (
 "context"
 "errors"
 "sync"
 "testing"
)

type fakeInspector struct {
 mu sync.Mutex
 values map[string]string
 errs map[string]error
 calls int
}
func (f *fakeInspector) Inspect(_ context.Context, name string) (string,error) {
 f.mu.Lock()
 defer f.mu.Unlock()
 f.calls++
 return f.values[name], f.errs[name]
}
func TestUncachedVerdicts(t *testing.T) {
 cases := []struct {
  name string
  versions map[string]string
  failures map[string]error
  allowed bool
  definitive bool
  blocks int
  missing int
 }{
  {"complete_allow",map[string]string{"a":"ok","b":"ok"},nil,true,true,0,0},
  {"incomplete_allow",map[string]string{"b":"ok"},map[string]error{"a":errors.New("device unavailable")},true,false,0,1},
  {"complete_block",map[string]string{"a":"bad","b":"ok"},nil,false,true,1,0},
  {"block_with_incomplete",map[string]string{"a":"bad"},map[string]error{"b":errors.New("device unavailable")},false,true,1,1},
 }
 for _,tc:=range cases {
  t.Run(tc.name,func(t *testing.T){
   f:=&fakeInspector{values:tc.versions,errs:tc.failures}
   request:=Request{ResourceID:"host-1",AttemptID:"try-1",Side:Source,Policy:Policy{Revision:"v1",Excluded:map[string]bool{"bad":true}},Devices:[]string{"a","b"}}
   got,err:=Evaluate(context.Background(),request,f)
   if err!=nil {t.Fatal(err)}
   if got.Allowed!=tc.allowed || got.Definitive!=tc.definitive || len(got.RejectedDevices)!=tc.blocks || len(got.IncompleteDevices)!=tc.missing {
    t.Fatalf("verdict=%+v want allowed=%v definitive=%v rejected=%d incomplete=%d",got,tc.allowed,tc.definitive,tc.blocks,tc.missing)
   }
   if f.calls!=2 {t.Fatalf("expected 2 fresh observations, got %d",f.calls)}
  })
 }
}
func TestContextSpecificPolicy(t *testing.T){
 f:=&fakeInspector{values:map[string]string{"disk":"fw-7"}}
 req:=Request{ResourceID:"host-1",Side:Source,Policy:Policy{Revision:"source-v1",Excluded:map[string]bool{"fw-7":true}},Devices:[]string{"disk"}}
 source,err:=Evaluate(context.Background(),req,f);if err!=nil{t.Fatal(err)}
 req.Side=Destination
 req.Policy=Policy{Revision:"destination-v1",Excluded:map[string]bool{}}
 destination,err:=Evaluate(context.Background(),req,f);if err!=nil{t.Fatal(err)}
 if source.Allowed || !destination.Allowed {t.Fatalf("context verdicts source=%+v destination=%+v",source,destination)}
}
func TestRepeatedAttemptsInspectAgain(t *testing.T) {
 f:=&fakeInspector{values:map[string]string{"disk":"ok"}}
 r:=Request{ResourceID:"host-1",AttemptID:"try-1",Side:Source,Policy:Policy{Revision:"v1"},Devices:[]string{"disk"}}
 if _,err:=Evaluate(context.Background(),r,f);err!=nil{t.Fatal(err)}
 r.AttemptID="try-2"
 if _,err:=Evaluate(context.Background(),r,f);err!=nil{t.Fatal(err)}
 if f.calls!=2 {t.Fatalf("uncached evaluations must inspect twice, got %d",f.calls)}
}
func TestRequestValidation(t *testing.T){
 f:=&fakeInspector{}
 r:=Request{ResourceID:"host-1",Policy:Policy{Revision:"v1"},Devices:[]string{"disk"}}
 if _,err:=Evaluate(context.Background(),r,f);err==nil{t.Fatal("missing side must be rejected")}
}

type waitingInspector struct { entered chan struct{} }
func (w waitingInspector) Inspect(ctx context.Context, _ string) (string,error) {
 close(w.entered)
 <-ctx.Done()
 return "",ctx.Err()
}

// A cancelled inspection must never be promoted to a reusable complete verdict.
func TestCancelledInspectionIsIncomplete(t *testing.T) {
 ctx,cancel:=context.WithCancel(context.Background())
 entered:=make(chan struct{})
 done:=make(chan Decision,1)
 errs:=make(chan error,1)
 request:=Request{ResourceID:"host-cancel",Side:Source,Policy:Policy{Revision:"v1"},Devices:[]string{"disk"}}
 go func() { got,err:=Evaluate(ctx,request,waitingInspector{entered});done<-got;errs<-err }()
 <-entered
 cancel()
 got:=<-done
 if err:=<-errs;err!=nil {t.Fatal(err)}
 if !got.Allowed || got.Definitive || len(got.IncompleteDevices)!=1 {
  t.Fatalf("cancelled check returned %+v",got)
 }
}

func TestDuplicateDeviceRejected(t *testing.T) {
 req:=Request{ResourceID:"host-1",Side:Source,Policy:Policy{Revision:"v1"},Devices:[]string{"disk","disk"}}
 _,err:=Evaluate(context.Background(),req,&fakeInspector{})
 if err==nil {t.Fatal("duplicate device IDs must be rejected")}
}
