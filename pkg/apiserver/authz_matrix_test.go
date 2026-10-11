package apiserver_test

import (
	"testing"

	"github.com/hasirciogluhq/atellar/pkg/apiserver/apiservertest"
)

// TestAuthorizationMatrix calls every API method as every kind of caller,
// in a closed cluster and an open one, without any transport.
func TestAuthorizationMatrix(t *testing.T) {
	for _, mode := range []struct {
		name string
		open bool
	}{{"closed", false}, {"open", true}} {
		for _, tc := range apiservertest.Cases() {
			for _, caller := range apiservertest.Callers {
				t.Run(mode.name+"/"+tc.Name+"/"+string(caller), func(t *testing.T) {
					t.Parallel()
					f := apiservertest.New(t)
					if mode.open {
						f = apiservertest.NewOpen(t)
					}
					err := tc.Call(f.Context(t, caller), f.Server, f)
					if err := apiservertest.Check(err, tc.Expect(caller, f.Open)); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
