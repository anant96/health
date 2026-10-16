package main

import (
 "context"
 "fmt"
 "github.com/alexliesenfeld/health/validation"
)

type sampleInspector map[string]string
func (s sampleInspector) Inspect(_ context.Context, device string) (string,error) {return s[device],nil}

func main(){
 req:=validation.Request{ResourceID:"sample-node",AttemptID:"attempt-1",Side:validation.Source,Policy:validation.Policy{Revision:"example-v1",Excluded:map[string]bool{"excluded-fw":true}},Devices:[]string{"disk0"}}
 result,err:=validation.Evaluate(context.Background(),req,sampleInspector{"disk0":"fw-ok"})
 if err!=nil {panic(err)}
 fmt.Printf("allowed=%t definitive=%t\n",result.Allowed,result.Definitive)
}
