package validation

import (
 "context"
 "errors"
 "fmt"
 "sort"

 "github.com/alexliesenfeld/health"
)

// Side identifies where a maintenance-readiness evaluation is performed.
type Side string
const (
 Source Side = "source"
 Destination Side = "destination"
)

type Request struct {
 ResourceID string
 AttemptID string
 Side Side
 Policy Policy
 Devices []string
}

// A revision identifies an immutable policy snapshot supplied by the caller.
type Policy struct {
 Revision string
 Excluded map[string]bool
}

type Observation struct {
 Device string
 Firmware string
 ReadError error
}

type Inspector interface {
 Inspect(context.Context, string) (string, error)
}

// Allowed is the operational decision. Definitive indicates whether the
// observed evidence supports an unambiguous verdict.
type Decision struct {
 Allowed bool
 Definitive bool
 RejectedDevices []string
 IncompleteDevices []string
}

// Evaluate performs a fresh firmware-readiness evaluation. No verdicts are cached.
func Evaluate(ctx context.Context, req Request, inspect Inspector) (Decision, error) {
 if inspect == nil { return Decision{}, errors.New("inspector is required") }
 if req.ResourceID == "" || req.Policy.Revision == "" { return Decision{}, errors.New("resource and policy revision are required") }
 if req.Side != Source && req.Side != Destination { return Decision{}, errors.New("source or destination side is required") }
 names := make(map[string]struct{}, len(req.Devices))
 checks := make([]health.Check, 0, len(req.Devices))
 resultCh := make(chan Observation, len(req.Devices))
 for _, name := range req.Devices {
  if name == "" { return Decision{}, errors.New("empty device name") }
  if _, found := names[name]; found { return Decision{}, fmt.Errorf("duplicate device %q", name) }
  names[name] = struct{}{}
  device := name
  checks = append(checks, health.Check{Name: device, Check: func(c context.Context) error {
   version, err := inspect.Inspect(c, device)
   resultCh <- Observation{Device: device, Firmware: version, ReadError: err}
   return err
  }})
 }
 checker := health.NewChecker(health.WithDisabledAutostart(), health.WithDisabledCache(), health.WithChecks(checks...))
 checked := checker.Check(ctx)
 // Checks may time out while callbacks are still running. Do not close this
 // fully buffered channel: an eventual callback must never send to a closed channel.
 obs := make(map[string]Observation, len(req.Devices))
 for i := 0; i < len(req.Devices); i++ {
  select {
  case o := <-resultCh:
   obs[o.Device] = o
  default:
  }
 }
 out := Decision{Allowed: true, Definitive: true}
 for _, name := range req.Devices {
  item, observed := obs[name]
  detail, checkedNow := checked.Details[name]
  if !observed || !checkedNow || detail.Error != nil || item.ReadError != nil || item.Firmware == "" {
   out.IncompleteDevices = append(out.IncompleteDevices, name)
   continue
  }
  if req.Policy.Excluded[item.Firmware] { out.RejectedDevices = append(out.RejectedDevices, name) }
 }
 if len(out.RejectedDevices) != 0 {
  out.Allowed = false
 } else if len(out.IncompleteDevices) != 0 {
  out.Definitive = false
 }
 sort.Strings(out.RejectedDevices)
 sort.Strings(out.IncompleteDevices)
 return out, nil
}
